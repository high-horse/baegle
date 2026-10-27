package nosql

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/sqlscript"
	"db-lens/internal/engine/transports"
	gocql "github.com/apache/cassandra-gocql-driver/v2"
)

type cassandraBackend struct {
	cfg       entities.ConnectionConfig
	transport transports.Transport
	session   *gocql.Session
}

func (b *cassandraBackend) connect(ctx context.Context) error {
	host, portText, err := net.SplitHostPort(b.transport.Address())
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return err
	}
	cluster := gocql.NewCluster(host)
	cluster.Port = port
	cluster.Keyspace = b.cfg.Database
	cluster.Timeout = 30 * time.Second
	cluster.ConnectTimeout = 10 * time.Second
	// One configured endpoint also works through SSH, without dialing private
	// addresses advertised by cluster discovery outside the tunnel.
	cluster.DisableInitialHostLookup = true
	cluster.IgnorePeerAddr = true
	cluster.Events.DisableNodeStatusEvents = true
	cluster.Events.DisableTopologyEvents = true
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolve Cassandra host: %w", err)
	}
	cluster.HostFilter = gocql.HostFilterFunc(func(candidate *gocql.HostInfo) bool {
		for _, address := range addresses {
			if address.IP.Equal(candidate.ConnectAddress()) {
				return true
			}
		}
		return false
	})
	if b.cfg.User != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{Username: b.cfg.User, Password: b.cfg.Password}
	}
	if b.cfg.SSL {
		cluster.SslOpts = &gocql.SslOptions{Config: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: b.cfg.Host}, EnableHostVerification: true}
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	b.session, err = cluster.CreateSession()
	if err != nil {
		return err
	}
	return b.ping(ctx)
}
func (b *cassandraBackend) close() error {
	if b.session != nil {
		b.session.Close()
	}
	return nil
}
func (b *cassandraBackend) ping(ctx context.Context) error {
	if b.session == nil {
		return fmt.Errorf("Cassandra is disconnected")
	}
	return b.session.Query("SELECT release_version FROM system.local").WithContext(ctx).Exec()
}
func cqlStatement(q string) (string, bool, error) {
	// Strip only leading comments for classification. Quoted text and comments
	// inside the statement are left to the native CQL parser.
	for {
		q = strings.TrimSpace(q)
		if strings.HasPrefix(q, "--") || strings.HasPrefix(q, "//") {
			i := strings.IndexByte(q, '\n')
			if i < 0 {
				q = ""
				break
			}
			q = q[i+1:]
			continue
		}
		if strings.HasPrefix(q, "/*") {
			i := strings.Index(q[2:], "*/")
			if i < 0 {
				return "", false, fmt.Errorf("unterminated CQL comment")
			}
			q = q[i+4:]
			continue
		}
		break
	}
	parts, err := sqlscript.Split(q, "cassandra")
	if err != nil {
		return "", false, err
	}
	if len(parts) != 1 {
		return "", false, fmt.Errorf("run one CQL statement at a time")
	}
	fields := strings.Fields(parts[0].SQL)
	if len(fields) == 0 {
		return "", false, fmt.Errorf("enter a CQL statement")
	}
	verb := strings.ToUpper(fields[0])
	switch verb {
	case "SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "ALTER", "DROP", "TRUNCATE":
	default:
		return "", false, fmt.Errorf("unsupported CQL statement %q; USE and cqlsh commands are not supported", verb)
	}
	return parts[0].SQL, verb == "SELECT", nil
}
func (b *cassandraBackend) query(ctx context.Context, q string, size int, state []byte) (*entities.QueryResult, []byte, error) {
	statement, read, err := cqlStatement(q)
	if err != nil {
		return nil, nil, err
	}
	if b.cfg.ReadOnly && !read {
		return nil, nil, fmt.Errorf("this connection is read-only")
	}
	if !read && len(state) > 0 {
		return nil, nil, fmt.Errorf("writes cannot be paged")
	}
	iter := b.session.Query(statement).WithContext(ctx).PageSize(size).PageState(state).Iter()
	result := &entities.QueryResult{IsQuery: read, Columns: []entities.ColumnInfo{}, Rows: [][]any{}}
	for _, column := range iter.Columns() {
		result.Columns = append(result.Columns, entities.ColumnInfo{Name: column.Name, DatabaseType: fmt.Sprint(column.TypeInfo), Nullable: true})
	}
	for {
		row := map[string]any{}
		if !iter.MapScan(row) {
			break
		}
		values := make([]any, len(result.Columns))
		for i, column := range result.Columns {
			values[i] = cqlValue(row[column.Name])
		}
		result.Rows = append(result.Rows, values)
	}
	next := append([]byte(nil), iter.PageState()...)
	if err = iter.Close(); err != nil {
		return nil, nil, err
	}
	if len(result.Columns) > 0 {
		result.IsQuery = true
	} // Lightweight transactions return [applied].
	if !read {
		next = nil
	}
	return result, next, nil
}
func cqlValue(value any) any {
	switch v := value.(type) {
	case nil, string, bool, float64, float32, int, int8, int16, int32, uint8, uint16, uint32:
		return v
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case time.Time:
		return v.Format(time.RFC3339Nano)
	case []byte:
		return fmt.Sprintf("0x%x", v)
	case fmt.Stringer:
		return v.String()
	default:
		encoded, err := json.Marshal(v)
		if err == nil {
			return string(encoded)
		}
		return fmt.Sprint(v)
	}
}
func (b *cassandraBackend) tables(ctx context.Context) ([]entities.InspectTableInfo, error) {
	result := []entities.InspectTableInfo{}
	for _, source := range []struct{ catalog, column, kind string }{{"tables", "table_name", "TABLE"}, {"views", "view_name", "VIEW"}} {
		iter := b.session.Query("SELECT "+source.column+" FROM system_schema."+source.catalog+" WHERE keyspace_name = ?", b.cfg.Database).WithContext(ctx).Iter()
		var name string
		for iter.Scan(&name) {
			result = append(result, entities.InspectTableInfo{Name: name, Type: source.kind, Database: b.cfg.Database, Schema: b.cfg.Database, Engine: "Cassandra"})
		}
		if err := iter.Close(); err != nil {
			return nil, err
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}
func (b *cassandraBackend) columns(ctx context.Context, table entities.InspectTableInfo) ([]entities.InspectColumnInfo, error) {
	if table.Database != b.cfg.Database || table.Schema != b.cfg.Database {
		return nil, fmt.Errorf("table belongs to another keyspace")
	}
	iter := b.session.Query("SELECT column_name, type, kind FROM system_schema.columns WHERE keyspace_name = ? AND table_name = ?", b.cfg.Database, table.Name).WithContext(ctx).Iter()
	result := []entities.InspectColumnInfo{}
	var name, kind, dataType string
	for iter.Scan(&name, &dataType, &kind) {
		key := kind == "partition_key" || kind == "clustering"
		result = append(result, entities.InspectColumnInfo{Name: name, DatabaseType: dataType, PrimaryKey: key, Nullable: !key})
	}
	return result, iter.Close()
}
func (b *cassandraBackend) ddl(ctx context.Context, table entities.TableRef) (string, error) {
	if table.Database != b.cfg.Database || table.Schema != b.cfg.Database {
		return "", fmt.Errorf("table belongs to another keyspace")
	}
	// DESCRIBE is server-side CQL in Cassandra 4+, preserving table options,
	// clustering order and indexes instead of reconstructing incomplete DDL.
	quote := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	iter := b.session.Query("DESCRIBE TABLE " + quote(table.Schema) + "." + quote(table.Name)).WithContext(ctx).Iter()
	statements := []string{}
	for {
		row := map[string]any{}
		if !iter.MapScan(row) {
			break
		}
		if ddl, ok := row["create_statement"].(string); ok {
			statements = append(statements, ddl)
		}
	}
	if err := iter.Close(); err != nil {
		return "", fmt.Errorf("Cassandra DDL inspection requires server-side DESCRIBE (Cassandra 4+): %w", err)
	}
	return strings.Join(statements, "\n\n"), nil
}
