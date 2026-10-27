package nosql

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/transports"
)

type couchBackend struct {
	cfg       entities.ConnectionConfig
	transport transports.Transport
	client    *http.Client
	base      string
}
type couchCommand struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Body   json.RawMessage `json:"body"`
}
type couchPage struct {
	Bookmark string `json:"bookmark,omitempty"`
	LastID   string `json:"lastID,omitempty"`
}

func (b *couchBackend) connect(ctx context.Context) error {
	if b.cfg.Database == "" || strings.Contains(b.cfg.Database, "/") {
		return fmt.Errorf("enter a CouchDB database name")
	}
	scheme := "http"
	if b.cfg.SSL {
		scheme = "https"
	}
	b.base = scheme + "://" + b.transport.Address() + "/" + url.PathEscape(b.cfg.Database)
	b.client = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: b.cfg.Host}}, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return b.ping(ctx)
}
func (b *couchBackend) close() error {
	if b.client != nil {
		b.client.CloseIdleConnections()
	}
	return nil
}
func (b *couchBackend) ping(ctx context.Context) error {
	_, err := b.request(ctx, "GET", "", nil)
	return err
}
func (b *couchBackend) request(ctx context.Context, method, path string, body []byte) (any, error) {
	req, err := http.NewRequestWithContext(ctx, method, b.base+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if b.cfg.User != "" || b.cfg.Password != "" {
		req.SetBasicAuth(b.cfg.User, b.cfg.Password)
	}
	response, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	const max = 32 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, max+1))
	if err != nil {
		return nil, err
	}
	if len(data) > max {
		return nil, fmt.Errorf("CouchDB response exceeds 32 MiB; narrow the query")
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	decodeErr := decoder.Decode(&value)
	result, _ := value.(map[string]any)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if result != nil {
			return nil, fmt.Errorf("CouchDB HTTP %d: %v: %v", response.StatusCode, result["error"], result["reason"])
		}
		return nil, fmt.Errorf("CouchDB HTTP %d", response.StatusCode)
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("CouchDB returned invalid JSON: %w", decodeErr)
	}
	return value, nil
}
func parseCouch(q string) (couchCommand, *url.URL, bool, error) {
	var cmd couchCommand
	decoder := json.NewDecoder(strings.NewReader(q))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cmd); err != nil || !json.Valid([]byte(q)) || strings.TrimSpace(q) == "null" {
		return cmd, nil, false, fmt.Errorf(`enter {"method":"GET","path":"/_all_docs?include_docs=true"} or a JSON HTTP request with body`)
	}
	cmd.Method = strings.ToUpper(cmd.Method)
	if cmd.Method == "" {
		cmd.Method = "GET"
	}
	switch cmd.Method {
	case "GET", "POST", "PUT", "DELETE":
	default:
		return cmd, nil, false, fmt.Errorf("use GET, POST, PUT or DELETE")
	}
	path, err := url.Parse(cmd.Path)
	if err != nil || path.IsAbs() || path.Host != "" || path.Fragment != "" || strings.Contains(path.Path, "\\") {
		return cmd, nil, false, fmt.Errorf("path must be relative to the selected database")
	}
	for _, part := range strings.Split(path.Path, "/") {
		if part == ".." || part == "." {
			return cmd, nil, false, fmt.Errorf("path must stay within the selected database")
		}
	}
	// Preserve encoded slashes in document IDs while validating the decoded path.
	escaped := path.EscapedPath()
	if !strings.HasPrefix(escaped, "/") {
		escaped = "/" + escaped
	}
	decoded, err := url.PathUnescape(escaped)
	if err != nil {
		return cmd, nil, false, err
	}
	path.Path, path.RawPath = decoded, escaped
	if path.Path == "/_changes" {
		return cmd, nil, false, fmt.Errorf("continuous change feeds are not supported in the result grid")
	}
	read := cmd.Method == "GET" || (cmd.Method == "POST" && (path.Path == "/_find" || path.Path == "/_explain" || path.Path == "/_all_docs" || path.Path == "/_bulk_get"))
	return cmd, path, read, nil
}
func (b *couchBackend) query(ctx context.Context, q string, size int, state []byte) (*entities.QueryResult, []byte, error) {
	cmd, path, read, err := parseCouch(q)
	if err != nil {
		return nil, nil, err
	}
	if b.cfg.ReadOnly && !read {
		return nil, nil, fmt.Errorf("this connection is read-only")
	}
	var page couchPage
	if len(state) > 0 {
		if err = json.Unmarshal(state, &page); err != nil {
			return nil, nil, err
		}
	}
	query := path.Query()
	body := cmd.Body
	pagedFind := path.Path == "/_find" && cmd.Method == "POST"
	pagedAll := path.Path == "/_all_docs" && cmd.Method == "GET" && query.Get("keys") == "" && query.Get("key") == ""
	// Explicit limits are honored as a single finite response. Browser-generated
	// requests omit limits and use continuation tokens instead of growing offsets.
	var object map[string]any
	if pagedFind {
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()
		if err = decoder.Decode(&object); err != nil || object == nil {
			return nil, nil, fmt.Errorf("_find needs a JSON object body with a selector")
		}
		if _, limited := object["limit"]; limited {
			pagedFind = false
		} else {
			object["limit"] = size
			if page.Bookmark != "" {
				object["bookmark"] = page.Bookmark
				delete(object, "skip")
			}
			body, err = json.Marshal(object)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	if query.Has("limit") {
		pagedAll = false
	}
	if pagedAll {
		query.Set("limit", fmt.Sprint(size+1))
		if page.LastID != "" {
			key, _ := json.Marshal(page.LastID)
			query.Set("startkey", string(key))
			query.Set("startkey_docid", page.LastID)
			query.Set("skip", "1")
		}
	}
	path.RawQuery = query.Encode()
	value, err := b.request(ctx, cmd.Method, path.String(), body)
	if err != nil {
		return nil, nil, err
	}
	if items, ok := value.([]any); ok {
		for _, item := range items {
			if doc, ok := item.(map[string]any); ok && doc["error"] != nil {
				return nil, nil, fmt.Errorf("CouchDB partial failure for %v: %v (%v); other documents may have been saved", doc["id"], doc["error"], doc["reason"])
			}
		}
		return documentRows(items), nil, nil
	}
	response, ok := value.(map[string]any)
	if !ok {
		return documentRows([]any{value}), nil, nil
	}
	var token []byte
	if docs, ok := response["docs"].([]any); ok && path.Path == "/_find" {
		if pagedFind && len(docs) == size {
			bookmark, _ := response["bookmark"].(string)
			if bookmark != "" && bookmark != page.Bookmark {
				token, _ = json.Marshal(couchPage{Bookmark: bookmark})
			}
		}
		return documentRows(docs), token, nil
	}
	if rows, ok := response["rows"].([]any); ok && (strings.HasPrefix(path.Path, "/_") || strings.Contains(path.Path, "/_view/")) {
		if pagedAll && len(rows) > size {
			rows = rows[:size]
			last, _ := rows[len(rows)-1].(map[string]any)
			id, _ := last["id"].(string)
			if id != "" {
				token, _ = json.Marshal(couchPage{LastID: id})
			}
		}
		docs := make([]any, 0, len(rows))
		for _, row := range rows {
			item, _ := row.(map[string]any)
			if doc, ok := item["doc"]; ok && doc != nil {
				docs = append(docs, doc)
			} else {
				docs = append(docs, row)
			}
		}
		return documentRows(docs), token, nil
	}
	return documentRows([]any{response}), nil, nil
}
func documentRows(docs []any) *entities.QueryResult {
	result := &entities.QueryResult{IsQuery: true, Columns: []entities.ColumnInfo{{Name: "document", DatabaseType: "JSON"}}, Rows: [][]any{}}
	for _, doc := range docs {
		encoded, err := json.Marshal(doc)
		if err != nil {
			encoded = []byte(fmt.Sprint(doc))
		}
		result.Rows = append(result.Rows, []any{string(encoded)})
	}
	return result
}
func (b *couchBackend) tables(context.Context) ([]entities.InspectTableInfo, error) {
	return collection(b.cfg, "Documents"), nil
}
func (b *couchBackend) columns(context.Context, entities.InspectTableInfo) ([]entities.InspectColumnInfo, error) {
	return []entities.InspectColumnInfo{{Name: "document", DatabaseType: "JSON"}}, nil
}
func (b *couchBackend) ddl(context.Context, entities.TableRef) (string, error) {
	return "", fmt.Errorf("CouchDB has no SQL DDL; use GET /_index to inspect Mango indexes and GET /_design_docs for design documents")
}
