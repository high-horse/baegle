# Data import and export

Use **Export** beside Refresh at the bottom of a result grid. Import remains in the workspace header beside Settings.

## Export

- **Entire table / view** exports every stored row, independently of result pagination.
- **Current result page** exports the loaded page, before local WHERE/ORDER BY filters or staged edits.
- **All query rows** reruns the last executed single SELECT and streams all returned rows. It does not consume the grid's pagination cursor.
- Follow the three export steps: **Format → Settings → Review**. Choose CSV, JSON, Excel XLSX, or SQL inserts; configure the scope, delimiter and NULL text; then click **Finish** to choose the destination in the native save dialog.
- Output is written to a temporary file first. The destination is replaced only after a successful export.

MongoDB supports entire-collection export as canonical Extended JSON. Current-page JSON export preserves the original documents; current-page CSV/XLSX exports the grid columns. SQL is unavailable for MongoDB.

## Import

For **SQL**, choose the file and click **Open SQL in editor**. The file opens as a new console named after the file; opening it never executes SQL. Edit the text, then use **Run script** (Alt+X) to execute all statements, or **Run / Ctrl+Enter** for the current statement or selection. Ctrl+S persists the edited console in the app.

Scripts run sequentially on one database session, stop on the first error, and report the statement number and source line. **Stop** cancels the running script. Earlier commits remain; explicit BEGIN/COMMIT statements control transactions, and unfinished transactions are discarded on exit. Script SELECT results are discarded; use Run on an individual SELECT to view its grid. SQL scripts support quoted strings/comments, PostgreSQL dollar-quoted bodies and COPY FROM STDIN blocks, MySQL DELIMITER directives, and SQLite trigger bodies. Command-line client directives such as psql `\connect` and SQLite `.read` are rejected before execution. Binary database dump archives must be restored with a native restore tool.

For **CSV, JSON and XLSX**:

1. Select an existing destination table or collection and a file.
2. Choose the format, CSV delimiter, and (for XLSX) worksheet.
3. Select **Preview file**. The dialog shows the total row count, first five rows, and column mappings.
4. Map source columns to destination columns. Blank mappings are skipped. Generated columns cannot be mapped; identity columns are skipped by default, but may be mapped to preserve source IDs if the server permits explicit identity values.
5. Select **Import rows** to insert the mapped data, then refresh the destination table.

Imports append rows, never truncate or replace an existing table. Existing unique constraints apply. Save or discard staged edits to the destination before importing. Read-only connections and views cannot be import destinations.

SQL databases use one transaction per import; row errors roll back the batch. MySQL destinations must use InnoDB. MongoDB stops at the first error and reports the number of documents already inserted; prepare a file containing only the remaining rows before retrying. A file changed since preview must be previewed again.

## Format details

| Format | Data representation |
| --- | --- |
| CSV | UTF-8, header row, selectable comma/semicolon/tab/pipe delimiter, quoted multiline fields. |
| JSON | Array of objects; missing properties use database defaults, explicit `null` stays NULL. Numbers are parsed without passing through JavaScript floating point. Binary SQL values use Extended JSON `$binary`. |
| Excel | XLSX, selectable input worksheet, first row is column headers. Export uses text cells to preserve large IDs and strings beginning with `=`. Import reads stored cell values, not recalculated formulas. |
| SQL | Export writes INSERT statements. Import opens the script as editable SQL; Run script executes the edited statements against the active connection. |

CSV/XLSX use `\N` for NULL and an empty field for an empty string. Literal `\N` and strings beginning with a backslash are escaped with an additional backslash. Binary fields use base64. CSV has no cell types: use XLSX if spreadsheet software must treat exported strings as text.

The gear dialog has an **Import file size limit (MiB)** setting shared across all connections and sessions. Empty or 0 means unlimited (the default); a positive value applies to SQL file loading and data imports. A file above the configured limit explains how to change it. Limits are checked again when importing, including if the file grows after preview.

Row-based imports allow 100,000 rows and 128 MiB expanded XLSX content. These restrictions do not apply to SQL scripts. Excel export rejects values beyond its worksheet/cell limits instead of truncating them. Larger exports should use CSV or JSON. Data imports have a ten-minute timeout and exports a thirty-minute timeout; SQL scripts can be stopped from the console.
