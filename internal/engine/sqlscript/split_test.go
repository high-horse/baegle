package sqlscript

import (
	"strings"
	"testing"
)

func TestDialectScriptSplitting(t *testing.T) {
	cases := []struct {
		name, dialect, text string
		count               int
	}{
		{"quotes", "sqlite", "-- header\nCREATE TABLE x(t TEXT);\nINSERT INTO x VALUES('it''s; fine'); SELECT \"semi;colon\";", 3},
		{"dollar body", "pgx", "CREATE FUNCTION f() RETURNS void AS $body$ BEGIN PERFORM 1; PERFORM 2; END $body$ LANGUAGE plpgsql; SELECT 1;", 2},
		{"escaped string", "pgx", `SELECT E'it\'s; fine'; SELECT 2;`, 2},
		{"nested comment", "pgx", "/* outer /* nested; */ end */ SELECT 1;", 1},
		{"mysql routine", "mysql", "DELIMITER $$\nCREATE PROCEDURE p() BEGIN SELECT 1; SELECT 2; END$$\nDELIMITER ;\nCALL p();", 2},
		{"mysql version directive", "mysql", "/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */; SELECT 1;", 2},
		{"sqlite trigger", "sqlite", "CREATE TRIGGER t AFTER INSERT ON x BEGIN INSERT INTO y VALUES(CASE WHEN NEW.id = 1 THEN 2 ELSE 3 END); UPDATE x SET id=4; END; SELECT 1;", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			statements, err := Split(c.text, c.dialect)
			if err != nil || len(statements) != c.count {
				t.Fatalf("got %d statements: %v %+v", len(statements), err, statements)
			}
		})
	}
	statements, err := Split(cases[0].text, "sqlite")
	if err != nil || statements[0].Line != 2 || statements[1].Line != 3 {
		t.Fatalf("incorrect line numbers: %+v %v", statements, err)
	}
}
func TestCopyDataAndEarlyValidation(t *testing.T) {
	statements, err := Split("CREATE TABLE x(t text);\nCOPY x(t) FROM stdin;\none;two\nthree\n\\.\nSELECT 1;", "pgx")
	if err != nil || len(statements) != 3 || statements[1].CopyData == nil || *statements[1].CopyData != "one;two\nthree\n" {
		t.Fatalf("COPY: %+v %v", statements, err)
	}
	for _, text := range []string{"SELECT 1; SELECT 'unfinished", "SELECT 1; /* unfinished", "COPY x FROM STDIN;\ndata without terminator", "\\connect other\nSELECT 1;", "DO $x$ BEGIN SELECT 1;"} {
		if _, err = Split(text, "pgx"); err == nil {
			t.Fatalf("accepted malformed input: %s", text)
		}
	}
	statements, err = Split("-- empty\n /*comment*/ ;", "sqlite")
	if err != nil || len(statements) != 0 {
		t.Fatalf("comments became SQL: %v %+v", err, statements)
	}
	statements, err = Split(strings.Repeat("-- large comment\n", 100000)+"SELECT 1;", "sqlite")
	if err != nil || len(statements) != 1 {
		t.Fatalf("large script: %v", err)
	}
}
