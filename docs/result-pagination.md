# Result pagination

SQL SELECT queries and supported read-only CTEs fetch a bounded page with
`LIMIT pageSize + 1 OFFSET offset`. The extra row indicates whether Next is
available. No automatic COUNT runs before the first page or during navigation.
The default page size is 100, configurable from 1 to 500 in workspace settings.
The database connection is released after each page, including on MySQL.

The total remains unknown until a final page is read. Next works using lookahead;
Previous and First use the ten-page cache or a bounded offset query. Last is
available only when the total is known. Opening a table never scans to the end
just to populate the footer. Explicit limits, filters, grouping, and ordering
remain inside the original query; global sorting uses an outer ORDER BY ordinal.
Deep offsets and unindexed sorts can still be expensive. Use selective filters
and indexed ordering for large datasets. Pages are separate statements, not a
transaction snapshot; use unique ordering for stable navigation.

An explicitly entered SELECT COUNT(*) still computes an exact count. It runs
once, without the former automatic count-of-the-count query. On large MySQL
InnoDB tables it may still take time to scan the relevant index or rows.

Query timeout is saved with workspace settings, defaults to 60 seconds, and
accepts 0 (unlimited) through 86400 seconds. It applies to new executions,
page fetches, and entire SQL scripts. On expiration the backend cancels the
operation and returns a timeout error; completed script commits remain committed.
Changing the setting does not modify a query already running. Driver and server
cancellation behavior determines how quickly server resources are reclaimed.

Non-pageable SQL (such as mutations with RETURNING) retains execute-once cursor
streaming, with a ten-minute cursor lifetime, a sixteen-cursor cap, and bounded
page caching. Each cursor pins a connection until closed or exhausted. Closing a
cursor cancels it; it does not guarantee rollback. MongoDB retains its existing
command/cursor pagination behavior.

Tests cover bounded reads without counting, connection release, exact counts,
original query limits, global sorting, empty/final pages, unknown-total navigation,
saved timeout defaults and validation, database cancellation, and subsequent reuse.
An optional MySQL integration test uses DBVIEWER_TEST_MYSQL_DSN pointing to an
isolated database; it creates only a session-local temporary table.
