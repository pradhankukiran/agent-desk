package desk

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func put(t *testing.T, path, content string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, []byte(content), 0640); e != nil {
		t.Fatal(e)
	}
}

func fixture(t *testing.T) *Server { t.Helper(); return New(t.TempDir()) }

func dbFixture(t *testing.T, path string, statements ...string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0700)
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	for _, statement := range statements {
		if _, e = db.Exec(statement); e != nil {
			t.Fatal(e)
		}
	}
}
