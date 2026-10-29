# Result-grid tools

- Click a cell and use arrows to navigate. Click and drag, Shift+arrows, or Shift+click selects a highlighted rectangle; dragging at the grid edges scrolls through the loaded page. Ctrl+A selects the loaded page's visible data columns. Row numbers are never selected or copied.
- The row detail view (Tab) and Schema table support the same selection and copying. In row details, select names, values, or both; Tab or Escape returns to the result grid. Schema copying follows the displayed sort order and excludes SN.
- Ctrl+C copies the selected cells as tab-separated spreadsheet text. Ctrl+V pastes into editable SQL table tabs. A single pasted value fills the selected rectangle. Changes are staged: Save applies them, Cancel discards them. Primary/generated keys stay protected, and paste cannot extend beyond the loaded rows. Add rows first when needed. Copying between grids preserves NULL separately from empty text; plain spreadsheet text represents both as empty cells.
- Right-click a cell for equals, exclude, contains, and NULL filters. Filters combine with the existing WHERE expression and apply to the current page. Contains uses SQL LIKE wildcards (`%`, `_`).
- The Columns button hides, pins, and reorders columns. Fit widths sizes columns using up to 500 loaded rows. Visibility, order, and pins are remembered locally per table and column set; Reset layout restores the default arrangement.
- History in the query-console toolbar searches the latest 1,000 stored executions. It shows database, time, duration, and status. Open restores SQL into a new console on the same active connection without executing it.
- Right-click a table cell and choose Related records to follow declared incoming or outgoing foreign keys. Choosing a relationship executes a SELECT in a new console. Composite keys are supported. NULL keys and pending edits disable navigation. Metadata supports SQLite, PostgreSQL, and MySQL; arbitrary query results and MongoDB do not expose foreign-key links.

On platforms using Command shortcuts, Cmd+A/C/V also work in the grid.
