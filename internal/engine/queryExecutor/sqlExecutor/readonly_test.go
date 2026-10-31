package sqlExecutor

import "testing"

func TestReadOnlySQL(t *testing.T) {
	for _, query := range []string{
		"SELECT * FROM items", "-- comment\nSELECT 'DELETE; UPDATE' AS note; -- trailing",
		`SELECT "update", [delete], ` + "`insert`" + " FROM items",
		"WITH records AS (SELECT 1 AS id) SELECT * FROM records",
		"SELECT CASE WHEN id = 1 THEN 'yes' ELSE 'no' END FROM items",
		"SELECT REPLACE(name, 'a', 'b') FROM items", "SELECT SUBSTRING(name FROM 1 FOR 2) FROM items",
		"VALUES (1), (2)", "TABLE items",
		"SHOW TABLES", "SHOW CREATE TABLE items", "DESCRIBE items", "EXPLAIN SELECT * FROM items",
		"PRAGMA table_info('items')", "PRAGMA main.table_xinfo('items')", "PRAGMA index_list(items)",
	} {
		if err := CheckReadOnlySQL(query, "sqlite"); err != nil {
			t.Errorf("read rejected: %s: %v", query, err)
		}
	}
	if err := CheckReadOnlySQL("SELECT $$text; DELETE FROM items$$", "pgx"); err != nil {
		t.Fatal(err)
	}
	if err := CheckReadOnlySQL("SELECT 1--not_a_comment; DELETE FROM items", "mysql"); err == nil {
		t.Fatal("MySQL comment ambiguity accepted")
	}
	for _, query := range []string{
		"", "/* comment */", "INSERT INTO items VALUES (1)", "UPDATE items SET id=1 RETURNING *",
		"DELETE FROM items RETURNING *", "DROP TABLE items", "CREATE TABLE x (id INT)",
		"WITH changed AS (DELETE FROM items RETURNING *) SELECT * FROM changed",
		"SELECT 1; DELETE FROM items", "SELECT 1; /*! DELETE FROM items */", "SELECT 1 /*M! INTO OUTFILE '/tmp/out' */",
		"SELECT * INTO backup FROM items", "SELECT * FROM items FOR UPDATE", "SELECT * FROM items LOCK IN SHARE MODE",
		"SELECT * FROM items WITH (UPDLOCK)", "SELECT NEXT VALUE FOR seq", "SELECT seq.nextval FROM dual",
		"EXPLAIN ANALYZE DELETE FROM items", "CALL change_data()", "EXEC change_data",
		"SET default_transaction_read_only = off", "PRAGMA query_only = 0", "PRAGMA writable_schema(1)",
		"PRAGMA journal_mode(WAL)", "ATTACH DATABASE 'other.db' AS other", "VACUUM", "BEGIN",
		"SELECT set_config('transaction_read_only','off',false)", `SELECT "set_config"('transaction_read_only','off',false)`,
		"SELECT load_extension('evil')", "SELECT writefile('/tmp/out','data')", "SELECT nextval('seq')",
		"SELECT 1 INTO OUTFILE '/tmp/out'", "SELECT @value := 1", "SELECT 'unterminated", "SELECT 1 /* unclosed",
	} {
		if err := CheckReadOnlySQL(query, "sqlite"); err == nil {
			t.Errorf("write or ambiguous SQL accepted: %s", query)
		}
	}
}
