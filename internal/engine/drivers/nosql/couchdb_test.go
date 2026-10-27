package nosql

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/transports"
)

func TestCouchPagingAndErrors(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		user, pass, ok := r.BasicAuth()
		if !ok || user != "test" || pass != "secret" {
			t.Error("missing auth")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/db":
			fmt.Fprint(w, `{"db_name":"db"}`)
		case "/db/_all_docs":
			if r.URL.Query().Get("limit") != "3" {
				t.Error("missing lookahead limit")
			}
			if r.URL.Query().Get("startkey") == "" {
				fmt.Fprint(w, `{"rows":[{"id":"a","doc":{"_id":"a"}},{"id":"b","doc":{"_id":"b"}},{"id":"c","doc":{"_id":"c"}}]}`)
			} else {
				if r.URL.Query().Get("startkey") != `"b"` || r.URL.Query().Get("skip") != "1" {
					t.Error("bad continuation")
				}
				fmt.Fprint(w, `{"rows":[{"id":"c","doc":{"_id":"c"}}]}`)
			}
		case "/db/_find":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["limit"] != float64(2) {
				t.Error("missing limit")
			}
			if body["bookmark"] == "after-b" {
				fmt.Fprint(w, `{"docs":[],"bookmark":"after-b"}`)
			} else {
				fmt.Fprint(w, `{"docs":[{"_id":"a"},{"_id":"b"}],"bookmark":"after-b"}`)
			}
		case "/db/_bulk_docs":
			fmt.Fprint(w, `[{"id":"a","ok":true},{"id":"b","error":"conflict","reason":"Document update conflict"}]`)
		case "/db/conflict":
			w.WriteHeader(409)
			fmt.Fprint(w, `{"error":"conflict","reason":"revision mismatch"}`)
		case "/db/doc":
			fmt.Fprint(w, `{"_id":"doc","rows":[1,2],"docs":[3]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	host, port, _ := net.SplitHostPort(u.Host)
	n, _ := strconv.Atoi(port)
	cfg := entities.ConnectionConfig{Type: "couchdb", Database: "db", Host: host, Port: n, User: "test", Password: "secret"}
	b := &couchBackend{cfg: cfg, transport: transports.NewDirect(host, n)}
	if err := b.connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer b.close()
	q := `{"method":"GET","path":"/_all_docs?include_docs=true"}`
	first, state, err := b.query(context.Background(), q, 2, nil)
	if err != nil || len(first.Rows) != 2 || len(state) == 0 {
		t.Fatalf("first %v %v", first, err)
	}
	last, state, err := b.query(context.Background(), q, 2, state)
	if err != nil || len(last.Rows) != 1 || len(state) != 0 {
		t.Fatalf("last %v %v", last, err)
	}
	q = `{"method":"POST","path":"/_find","body":{"selector":{}}}`
	_, state, err = b.query(context.Background(), q, 2, nil)
	if err != nil || len(state) == 0 {
		t.Fatal("missing bookmark", err)
	}
	_, state, err = b.query(context.Background(), q, 2, state)
	if err != nil || len(state) != 0 {
		t.Fatal("bookmark did not terminate", err)
	}
	for _, path := range []string{"/_bulk_docs", "/conflict"} {
		if _, _, err = b.query(context.Background(), fmt.Sprintf(`{"method":"POST","path":%q,"body":{}}`, path), 2, nil); err == nil {
			t.Fatal("write failure ignored")
		}
	}
	doc, _, err := b.query(context.Background(), `{"path":"/doc"}`, 2, nil)
	if err != nil || len(doc.Rows) != 1 || !strings.Contains(doc.Rows[0][0].(string), `"_id":"doc"`) {
		t.Fatal("document fields mistaken for query results", doc, err)
	}
	b.cfg.ReadOnly = true
	before := calls
	if _, _, err = b.query(context.Background(), `{"method":"PUT","path":"/a","body":{}}`, 2, nil); err == nil || calls != before {
		t.Fatal("read-only write reached server")
	}
}
func TestCouchPathValidation(t *testing.T) {
	_, path, _, err := parseCouch(`{"path":"/folder%2Fdoc"}`)
	if err != nil || path.String() != "/folder%2Fdoc" {
		t.Fatalf("encoded document ID lost: %v %v", path, err)
	}
	for _, path := range []string{"https://other/db", "//other/db", "/../other", "/%2e%2e/other", "/a/../../other", "/_changes", "/a#fragment"} {
		q, _ := json.Marshal(couchCommand{Method: "GET", Path: path})
		if _, _, _, err := parseCouch(string(q)); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
	for _, q := range []string{`null`, `{"path":"/a","unknown":true}`, `{"path":"/a"} {"path":"/b"}`} {
		if _, _, _, err := parseCouch(q); err == nil {
			t.Fatalf("accepted %s", q)
		}
	}
}
