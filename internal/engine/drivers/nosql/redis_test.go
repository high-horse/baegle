package nosql

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/transports"
)

func TestRedisCommandValidation(t *testing.T) {
	for _, q := range []string{`["SUBSCRIBE","x"]`, `["SELECT",1]`, `["MULTI"]`, `["EVAL","return 1",0]`, `["BLPOP","x",0]`, `["KEYS","*"]`, `["GET",{}]`, `[]`, `["GET","a"] ["GET","b"]`} {
		if _, _, err := redisCommand(q); err == nil {
			t.Fatalf("accepted %s", q)
		}
	}
	args, read, err := redisCommand(`["get","a"]`)
	if err != nil || !read || args[0] != "GET" {
		t.Fatal(args, read, err)
	}
	b := &redisBackend{cfg: entities.ConnectionConfig{ReadOnly: true}}
	if _, _, err = b.query(context.Background(), `["SET","a","b"]`, 100, nil); err == nil {
		t.Fatal("read-only write accepted")
	}
}

func TestRedisLocalServer(t *testing.T) {
	binary, err := exec.LookPath("redis-server")
	if err != nil {
		t.Skip("redis-server is not installed")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	cmd := exec.Command(binary, "--bind", "127.0.0.1", "--port", strconv.Itoa(port), "--save", "", "--appendonly", "no", "--requirepass", "test-secret", "--dir", t.TempDir())
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	cfg := entities.ConnectionConfig{Type: "redis", Host: "127.0.0.1", Port: port, Database: "1", Password: "test-secret"}
	b := &redisBackend{cfg: cfg, transport: transports.NewDirect(cfg.Host, port)}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := 0; i < 30; i++ {
		err = b.connect(ctx)
		if err == nil {
			break
		}
		b.close()
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer b.close()
	for _, q := range []string{`["SET","hello","world"]`, `["HSET","hash","a","one","b","two"]`, `["SET","large","9223372036854775807"]`} {
		if _, _, err = b.query(ctx, q, 10, nil); err != nil {
			t.Fatal(err)
		}
	}
	result, _, err := b.query(ctx, `["GET","hello"]`, 10, nil)
	if err != nil || result.Rows[0][0] != "world" {
		t.Fatal(result, err)
	}
	result, _, err = b.query(ctx, `["GET","missing"]`, 10, nil)
	if err != nil || result.Rows[0][0] != nil {
		t.Fatal("nil response", result, err)
	}
	result, _, err = b.query(ctx, `["INCRBY","large",0]`, 10, nil)
	if err != nil || result.Rows[0][0] != "9223372036854775807" {
		t.Fatal("precision lost", result, err)
	}
	result, _, err = b.query(ctx, `["HGETALL","hash"]`, 10, nil)
	if err != nil || len(result.Rows) == 0 {
		t.Fatal(result, err)
	}
	var hash map[string]string
	if len(result.Rows) == 1 {
		if err := json.Unmarshal([]byte(result.Rows[0][0].(string)), &hash); err != nil || hash["a"] != "one" || hash["b"] != "two" {
			t.Fatal("RESP3 hash is not usable JSON", result.Rows, err)
		}
	}
	for i := 0; i < 120; i++ {
		if err = b.client.Set(ctx, fmt.Sprintf("scan:%03d", i), "value", 0).Err(); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	var state []byte
	for page := 0; page < 100; page++ {
		result, next, err := b.query(ctx, `["SCAN","0","MATCH","scan:*"]`, 10, state)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range result.Rows {
			seen[row[0].(string)] = true
		}
		state = next
		if len(state) == 0 {
			break
		}
	}
	if len(seen) != 120 || len(state) != 0 {
		t.Fatal("scan missed keys or did not terminate", len(seen))
	}
	b.cfg.ReadOnly = true
	if _, _, err = b.query(ctx, `["DEL","hello"]`, 10, nil); err == nil {
		t.Fatal("read-only delete accepted")
	}
	value, err := b.client.Get(ctx, "hello").Result()
	if err != nil || value != "world" {
		t.Fatal("read-only command changed data")
	}
}

// Opt-in smoke test uses only a read query against the supplied keyspace.
func TestCassandraLive(t *testing.T) {
	host := os.Getenv("DBVIEWER_CASSANDRA_HOST")
	if host == "" {
		t.Skip("set DBVIEWER_CASSANDRA_HOST and DBVIEWER_CASSANDRA_KEYSPACE for a live smoke test")
	}
	port := 9042
	if value := os.Getenv("DBVIEWER_CASSANDRA_PORT"); value != "" {
		var err error
		port, err = strconv.Atoi(value)
		if err != nil {
			t.Fatal(err)
		}
	}
	cfg := entities.ConnectionConfig{Type: "cassandra", Host: host, Port: port, Database: os.Getenv("DBVIEWER_CASSANDRA_KEYSPACE"), User: os.Getenv("DBVIEWER_CASSANDRA_USER"), Password: os.Getenv("DBVIEWER_CASSANDRA_PASSWORD"), SSL: os.Getenv("DBVIEWER_CASSANDRA_TLS") == "1", ReadOnly: true}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, err := New("cassandra").Create(ctx, cfg, transports.NewDirect(host, port))
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	defer c.Disconnect()
	result, err := c.(*Connection).ExecuteInput(ctx, entities.QueryInput{Query: "SELECT release_version FROM system.local", PageSize: 100})
	if err != nil || len(result.Rows) != 1 {
		t.Fatal(result, err)
	}
	if _, err = New("cassandra").ListTables(ctx, c); err != nil {
		t.Fatal(err)
	}
}
