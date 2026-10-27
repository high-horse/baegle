package tableedit

import (
	"db-lens/internal/engine/entities"
	"encoding/json"
	"strings"
	"testing"
)

func TestEnterpriseRowEditParameters(t *testing.T) {
	for _, tt := range []struct{ dialect, want string }{
		{"oracle", `UPDATE "APP"."items" SET "name" = :1 WHERE "id" = :2`},
		{"sqlserver", `UPDATE [APP].[items] SET [name] = @p1 WHERE [id] = @p2`},
		{"db2", `UPDATE "APP"."items" SET "name" = ? WHERE "id" = ?`},
	} {
		t.Run(tt.dialect, func(t *testing.T) {
			info := entities.TableEditInfo{Driver: tt.dialect, Table: entities.TableRef{Name: "items", Schema: "APP"}, CanInsert: true, CanModify: true, Keys: []string{"id"}, Columns: []entities.InspectColumnInfo{{Name: "id", PrimaryKey: true, AutoIncrement: true}, {Name: "name"}}}
			query, args, err := BuildSQL(info, entities.RowChange{Operation: "update", Keys: map[string]json.RawMessage{"id": json.RawMessage("7")}, Values: map[string]json.RawMessage{"name": json.RawMessage(`"O'Reilly"`)}})
			if err != nil || query != tt.want || len(args) != 2 || args[0] != "O'Reilly" {
				t.Fatalf("%s %#v %v", query, args, err)
			}
			query, _, err = BuildSQL(info, entities.RowChange{Operation: "insert"})
			if err != nil || !strings.Contains(query, "DEFAULT") {
				t.Fatalf("default row: %s %v", query, err)
			}
		})
	}
	if Quote("a]b", "sqlserver") != "[a]]b]" {
		t.Fatal("bracket identifier was not escaped")
	}
}
