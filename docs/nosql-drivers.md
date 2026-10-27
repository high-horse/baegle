# Cassandra, CouchDB and Redis

All three appear under **NoSQL** alongside MongoDB and are included in the standard build. No native client is required. Cassandra uses the [Apache GoCQL driver](https://github.com/apache/cassandra-gocql-driver), Redis uses go-redis, and CouchDB uses Go's HTTP client. Dependencies stay in the normal Go module cache and frontend `node_modules`, not `/tmp`.

| Engine | Default port | Database field | Console syntax |
| --- | --- | --- | --- |
| Cassandra | 9042 | Existing keyspace | One CQL statement |
| CouchDB | 5984 | Existing database name | JSON HTTP request |
| Redis | 6379 | Non-negative index, normally `0` | JSON command array |

Saved connections, username/password authentication, SSH, verified TLS, history, cancellation, console restoration and result grids are supported. TLS verifies the configured database hostname, including through SSH. For CouchDB, omit the scheme from Host and enable TLS for HTTPS, supplying the server's HTTPS port. Redis supports an optional ACL username or password-only authentication.

## Cassandra

Open a table from the sidebar or execute a CQL statement with Ctrl+Enter:

```sql
SELECT * FROM "my_keyspace"."users";
```

```sql
INSERT INTO users (id, name) VALUES (42, 'Ada');
```

CQL highlighting, table/column completion, native pagination, table/view discovery, column metadata and server-side table DDL inspection are available. DDL inspection requires Cassandra 4+ with server-side `DESCRIBE` support.

The connection targets the configured node, so discovered private cluster endpoints cannot bypass SSH. Multi-node failover, CQL batches, `USE`, cqlsh commands, script execution and client-certificate configuration are not provided. Change the connection's keyspace instead of issuing `USE`. Read-only connections accept only SELECT statements.

## CouchDB

Open **Documents** in the sidebar. Console paths are relative to the selected database:

```json
{"method":"GET","path":"/_all_docs?include_docs=true"}
```

```json
{"method":"POST","path":"/_find","body":{"selector":{"type":"user"}}}
```

```json
{"method":"PUT","path":"/user-42","body":{"name":"Ada"}}
```

Updates and deletes require the current `_rev`, as in CouchDB's HTTP API; for example `{"method":"DELETE","path":"/user-42?rev=2-..."}`. Triple-click a JSON result cell to inspect it.

`GET /_all_docs` and `POST /_find` page automatically unless the request supplies an explicit limit. `_all_docs` uses key continuation and Mango uses [bookmarks](https://docs.couchdb.org/en/stable/api/database/find.html). Other endpoints return their finite JSON response, capped at 32 MiB. Redirects and `_changes` feeds are rejected. HTTP and bulk-document errors are surfaced; bulk writes can partially succeed, so review errors before retrying.

Read-only connections allow GET and read POST endpoints `_find`, `_explain`, `_all_docs`, and `_bulk_get`. Requests cannot escape the selected database. Use `GET /_index` and `GET /_design_docs` to inspect indexes and design documents instead of SQL DDL.

## Redis

Open **Keys** in the sidebar to scan the selected database. Commands use JSON arrays:

```json
["SCAN", "0", "MATCH", "user:*"]
```

```json
["GET", "user:42"]
```

```json
["HGETALL", "user:42"]
```

```json
["SET", "greeting", "Hello"]
```

Common finite commands for strings, hashes, lists, sets, sorted sets, streams and key expiry are supported. Arguments must be strings or JSON numbers. Use bounded ranges for large values, such as `["LRANGE","queue",0,99]`.

SCAN pages automatically. COUNT is a server hint: pages can vary in size or be empty, and changing data may produce repeated keys. HSCAN/SSCAN/ZSCAN expose native cursor responses for manual continuation. This connection targets standalone Redis; Cluster and Sentinel are not supported. Session-changing commands, transactions, subscriptions, blocking commands, scripting, server administration and `KEYS` are rejected. Change the saved database index instead of issuing `SELECT`. Read-only mode rejects writes before sending them.

## Result behavior

Native cursor pages are sequential; the UI caches the latest ten pages. Specify server filtering/sorting in the native query. Grid WHERE/ORDER BY controls filter the current page locally. Server-side header sorting, arbitrary page jumps, SQL relations and SQL scripts do not apply.

Selection, copying, JSON inspection and current-page CSV/JSON/Excel export work. Modify data through native commands; inline row editing, file import, all-query export and SQL INSERT export are unavailable for these three drivers. Cassandra bigint values and document/collection values are retained as text to avoid JavaScript precision loss.

## Verification

```bash
go test ./internal/engine/drivers/nosql -v
go test ./internal/...
```

Tests cover cursor isolation/retries/expiry, CQL validation, CouchDB HTTP fixtures and a disposable local Redis server when `redis-server` is installed. For the opt-in read-only Cassandra smoke test, set `DBVIEWER_CASSANDRA_HOST` and `DBVIEWER_CASSANDRA_KEYSPACE`. Optional settings: `DBVIEWER_CASSANDRA_PORT`, `DBVIEWER_CASSANDRA_USER`, `DBVIEWER_CASSANDRA_PASSWORD`, and `DBVIEWER_CASSANDRA_TLS=1`. Mocked tests and compilation do not replace verification against a live server.
