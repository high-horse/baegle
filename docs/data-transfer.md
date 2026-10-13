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
| SQL | Data-only `INSERT INTO … (columns) VALUES (literal values)` statements, using the connected database's quoting rules. Multiple tuples are supported. One source table and a consistent column order per file; source values are mapped into the chosen destination. |

CSV/XLSX use `\N` for NULL and an empty field for an empty string. Literal `\N` and strings beginning with a backslash are escaped with an additional backslash. Binary fields use base64. CSV has no cell types: use XLSX if spreadsheet software must treat exported strings as text.

SQL import does not execute arbitrary scripts. DDL, COPY, INSERT SELECT, expressions, stored routines, client directives and complete database dumps are rejected. It does not create tables or reset identity sequences. Use a native database restore tool for full backups.

Import limits: 32 MiB per file, 100,000 rows, 128 MiB expanded XLSX content. Excel export rejects values beyond its worksheet/cell limits instead of truncating them. Larger exports should use CSV or JSON. Imports have a ten-minute timeout and exports a thirty-minute timeout.
