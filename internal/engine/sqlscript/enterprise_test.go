package sqlscript

import (
	"strings"
	"testing"
)

func TestEnterpriseScripts(t *testing.T) {
	for _, tt := range []struct {
		dialect, script string
		count           int
	}{
		{"oracle", "BEGIN\n NULL;\n NULL;\nEND;\n/\nSELECT q'[a;b'c]' FROM dual;", 2},
		{"oracle", "CREATE OR REPLACE PROCEDURE p AS BEGIN NULL; END;\n/\nCREATE TABLE t(type VARCHAR2(30));", 2},
		{"oracle", "SELECT 1 FROM dual;\n/\nSELECT 2 FROM dual;", 2},
		{"sqlserver", "DECLARE @v int=1; SELECT @v;\nGO\nSELECT [a;b] FROM t;\nGO -- comment", 2},
		{"sqlserver", "SELECT 'a\nGO\nb'; /* outer /* GO */ inner */ SELECT 2;", 1},
		{"db2", "--#SET TERMINATOR @\nBEGIN ATOMIC\n INSERT INTO t VALUES(1);\n INSERT INTO t VALUES(2);\nEND@\n--#SET TERMINATOR ;\nSELECT * FROM t;", 2},
	} {
		t.Run(tt.dialect+tt.script, func(t *testing.T) {
			statements, err := Split(tt.script, tt.dialect)
			if err != nil || len(statements) != tt.count {
				t.Fatalf("statements: %#v %v", statements, err)
			}
			if strings.Contains(statements[0].SQL, "#SET TERMINATOR") {
				t.Fatal("client directive sent to server")
			}
		})
	}
	if _, err := Split("SELECT 1;\nGO 5", "sqlserver"); err == nil {
		t.Fatal("GO repetition must not silently repeat mutations")
	}
}
