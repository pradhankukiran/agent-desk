package desk

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveValidationConflictBackup(t *testing.T) {
	s := fixture(t)
	path := filepath.Join(s.Home, ".codex/config.toml")
	original := "model = \"test\"\n"
	put(t, path, original)
	f := File{Path: path, Kind: "config", Editable: true}
	if _, e := s.save(f, []byte("invalid = ["), digest([]byte(original)), false); e == nil {
		t.Fatal("accepted invalid TOML")
	}
	if _, e := s.save(f, []byte("model = \"new\""), "stale", false); !errors.Is(e, errConflict) {
		t.Fatalf("expected conflict: %v", e)
	}
	backup, e := s.save(f, []byte("model = \"new\"\n"), digest([]byte(original)), false)
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(backup)
	if e != nil {
		t.Fatal(e)
	}
	var old map[string]string
	if e = json.Unmarshal(b, &old); e != nil {
		t.Fatal(e)
	}
	if old["content"] != original || old["path"] != path {
		t.Fatal("backup does not preserve original")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0640 {
		t.Fatal("permissions changed")
	}
	if _, e = s.save(f, []byte(original), digest([]byte(original)), false); !errors.Is(e, errConflict) {
		t.Fatal("stale revision overwrote file")
	}
}

func TestSymlinkAndTraversalWritesRejected(t *testing.T) {
	s := fixture(t)
	outside := t.TempDir()
	target := filepath.Join(outside, "secret")
	put(t, target, "untouched")
	if e := os.Symlink(outside, filepath.Join(s.Home, "linked")); e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{filepath.Join(s.Home, "linked/secret"), filepath.Join(s.Home, "linked/new"), target} {
		f := File{Path: path, Kind: "instruction", Editable: true}
		if _, e := s.save(f, []byte("changed"), "missing", false); e == nil {
			t.Fatalf("unsafe write allowed: %s", path)
		}
	}
	b, _ := os.ReadFile(target)
	if string(b) != "untouched" {
		t.Fatal("external file changed")
	}
}

func TestNoExternalSymlinkRead(t *testing.T) {
	s := fixture(t)
	outside := filepath.Join(t.TempDir(), "private")
	put(t, outside, "secret")
	os.MkdirAll(filepath.Join(s.Home, ".codex"), 0700)
	path := filepath.Join(s.Home, ".codex/AGENTS.md")
	os.Symlink(outside, path)
	r := httptest.NewRequest(http.MethodGet, "http://localhost/api/file?id="+fileID(path), nil)
	if _, _, e := s.requestFile(r); e == nil {
		t.Fatal("external symlink readable")
	}
}
