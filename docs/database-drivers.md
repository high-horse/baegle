# Oracle, Microsoft SQL Server, and IBM Db2 LUW

The connection form supports the following database engines with username/password authentication:

| Engine | Driver key | Default port | Database field | Target versions |
| --- | --- | --- | --- | --- |
| Oracle | `oracle` | 1521 | Service name, such as `FREEPDB1` | Oracle 12c+ |
| Microsoft SQL Server | `sqlserver` | 1433 | Database name | SQL Server 2016+ |
| IBM Db2 LUW | `db2` | 50000 | Database name, such as `SAMPLE` | Db2 LUW 11.1+ |

SQL Server uses SQL authentication and an explicit TCP port. Oracle uses service names, not SIDs or wallet configuration. Db2 for i and z/OS are outside this integration's scope. Db2 connection fields containing semicolons or control characters are rejected to prevent CLI connection-string options from being injected.

The integration includes connection testing and saved connections, SSH transport, persisted TLS settings, table/view and column inspection, SQL completion/highlighting/formatting, result paging, primary-key-based staged edits, data transfer, and incoming/outgoing foreign-key navigation. Use appropriate database privileges; the client does not grant permissions.

TLS verifies certificates; it does not silently trust a self-signed certificate. Configure the database client's trust store as needed. Oracle wallets, Windows integrated/Entra authentication, and custom certificate settings are not exposed in this connection form.

## Drivers and installation

Go dependencies are pinned in `go.mod` and `go.sum`, and installed in Go's normal module cache:

- [go-ora](https://github.com/sijms/go-ora) supplies Oracle connectivity without Oracle Instant Client.
- [Microsoft go-mssqldb](https://github.com/microsoft/go-mssqldb) supplies SQL Server connectivity without an ODBC installation.
- [IBM go_ibm_db](https://github.com/ibmdb/go_ibm_db) requires IBM's native CLI headers and libraries. Follow [IBM's installation instructions](https://github.com/ibmdb/go_ibm_db/blob/master/INSTALL.md) for your operating system and architecture.

Oracle and SQL Server are included in the standard build. Db2 uses the `db2` build tag plus CGO, so installations without the IBM client can still build/run the other databases. A standard build returns an actionable error when Db2 is selected.

Keep native dependencies in a persistent location. This workspace uses the Git-ignored `.local/drivers/db2/clidriver` directory. No dependency needs to be stored in `/tmp`.

On Linux, from the project root:

```bash
export IBM_DB_HOME="$PWD/.local/drivers/db2/clidriver"
bash scripts/with-db2.sh go test -tags db2 ./internal/...
bash scripts/with-db2.sh wails3 task build EXTRA_TAGS=db2 PACKAGE_MANAGER=npm
bash scripts/with-db2.sh ./bin/baegle
```

The helper sets native include/library paths for the command. The IBM runtime libraries must also be available when launching the resulting application. The ordinary packaging tasks do not automatically bundle IBM's CLI client; deploy that runtime separately. Windows and macOS builds require the corresponding IBM client and native compiler environment, following IBM's instructions.

## SQL behavior

- SQL Server uses `GO` lines to delimit batches, preserving local variables across semicolon-separated statements. Run/Ctrl+Enter executes the current batch; Run script executes all batches. `GO` repetition counts are rejected. Results display the first row-returning result set of a batch.
- Oracle PL/SQL blocks and routine definitions retain their internal semicolons. Separate blocks with `/` on its own line. Ordinary SQL statements have their client terminators removed before execution. Oracle q-quoted strings are supported by statement selection and script splitting.
- Db2 compound SQL can use the standard `--#SET TERMINATOR @` directive and a matching `@` terminator; reset with `--#SET TERMINATOR ;` for ordinary statements.
- Simple SELECTs support paging, sorting, and counting with dialect-specific syntax. SQL Server CTEs, ordered queries, and queries containing OPTION use streaming results instead of unsafe derived-table rewriting; page jumps and server-side header sorting are unavailable for those shapes.
- Oracle DDL is read through `DBMS_METADATA.GET_DDL`, subject to database permissions. SQL Server and Db2 views use their stored definitions. Their table DDL is explicitly labeled **column structure only**: indexes, constraints, triggers, storage options, and exact identity/generated-column options must be reviewed separately. It is not a full schema backup.

## Verification

Unit tests cover connection-string escaping, TLS persistence and migration, script/statement boundaries, paging SQL, large row counts, parameterized edits, catalog mapping, and editor dialects. The Db2-enabled tests also compile and link against IBM CLI.

Live smoke tests are opt-in and read-only. Set a driver's DSN in `DB_VIEWER_ORACLE_DSN`, `DB_VIEWER_SQLSERVER_DSN`, or `DB_VIEWER_DB2_DSN`, then run:

```bash
go test ./internal/engine/drivers/enterprise -run TestLiveEnterpriseDatabase -v
# Db2:
bash scripts/with-db2.sh go test -tags db2 ./internal/engine/drivers/enterprise -run TestLiveEnterpriseDatabase -v
```

Optional `DB_VIEWER_<DRIVER>_SCHEMA` and `DB_VIEWER_<DRIVER>_TABLE` values also exercise column inspection. Tests skip engines without a configured DSN. Local compilation and mocked tests do not replace verification against your actual database server.
