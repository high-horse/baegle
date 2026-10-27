package app

import (
	"context"
	"testing"
)

func TestSQLiteRelations(t *testing.T) {
	service, table, conn := editService(t)
	_, err := conn.DB().Exec(`CREATE TABLE parents (a TEXT,b INTEGER,PRIMARY KEY(a,b)); CREATE TABLE items (id INTEGER PRIMARY KEY,pa TEXT,pb INTEGER,FOREIGN KEY(pa,pb) REFERENCES parents); CREATE TABLE children (id INTEGER PRIMARY KEY,item INTEGER REFERENCES items(id));`)
	if err != nil {
		t.Fatal(err)
	}
	relations, err := service.InspectTableRelations(context.Background(), table)
	if err != nil {
		t.Fatal(err)
	}
	if len(relations) != 2 {
		t.Fatalf("expected incoming and outgoing relations: %#v", relations)
	}
	for _, r := range relations {
		if r.Target.Name == "parents" {
			if len(r.Columns) != 2 || r.Columns[0] != "pa" || r.TargetColumns[0] != "a" || r.TargetColumns[1] != "b" {
				t.Fatalf("composite key pairing: %#v", r)
			}
		}
		if r.Target.Name == "children" {
			if r.Direction != "Referencing rows" || r.Columns[0] != "id" || r.TargetColumns[0] != "item" {
				t.Fatalf("incoming relation: %#v", r)
			}
		}
	}
	table.ConnectionID = "other"
	if _, err = service.InspectTableRelations(context.Background(), table); err == nil {
		t.Fatal("connection mismatch accepted")
	}
}
