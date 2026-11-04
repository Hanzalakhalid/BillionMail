package migrations

import (
	"strings"
	"testing"
)

func TestUpSQLRemovesAnnotationsAndDownMigration(t *testing.T) {
	input := "-- +goose Up\n-- +goose StatementBegin\nCREATE TABLE example(id INT);\n-- +goose StatementEnd\n-- +goose Down\nDROP TABLE example;"
	got := upSQL(input)
	if strings.Contains(got, "+goose") {
		t.Fatalf("upSQL retained annotation: %q", got)
	}
	if strings.Contains(got, "DROP TABLE") {
		t.Fatalf("upSQL retained down migration: %q", got)
	}
	if !strings.Contains(got, "CREATE TABLE") {
		t.Fatalf("upSQL removed up migration: %q", got)
	}
}
