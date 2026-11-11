package desk

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// saveVia opens a file through the API, saves content, and returns the new revision.
func saveVia(t *testing.T, s *Server, path, content string) string {
	t.Helper()
	opened := callAPI(t, s, "GET", "file?id="+fileID(path), "")
	saved := callAPI(t, s, "PUT", "file?id="+fileID(path), fmt.Sprintf(`{"content":%q,"revision":%q}`, content, opened.body["revision"]))
	if saved.code != 200 {
		t.Fatalf("save: %d %v", saved.code, saved.body)
	}
	return saved.body["revision"].(string)
}

func undoVia(t *testing.T, s *Server, path, revision string) apiResult {
	t.Helper()
	return callAPI(t, s, "POST", "undo?id="+fileID(path), fmt.Sprintf(`{"revision":%q}`, revision))
}

func undoInfo(t *testing.T, s *Server, path string) map[string]any {
	t.Helper()
	u, _ := callAPI(t, s, "GET", "file?id="+fileID(path), "").body["undo"].(map[string]any)
	return u
}

func TestUndoEdit(t *testing.T) {
	s := fixture(t)
	path := filepath.Join(s.Home, ".codex/config.toml")
	put(t, path, "a = 1\n")
	rev := saveVia(t, s, path, "a = 2\n")
	// A later GET, as after closing the editor or reloading the page, still offers undo.
	if u := undoInfo(t, s, path); u["content"] != "a = 1\n" || u["removes"] != false {
		t.Fatalf("undo info: %v", u)
	}
	if r := undoVia(t, s, path, rev); r.code != 200 || r.body["removed"] != false {
		t.Fatalf("undo: %d %v", r.code, r.body)
	}
	if b, _ := os.ReadFile(path); string(b) != "a = 1\n" {
		t.Fatalf("content: %q", b)
	}
	if undoInfo(t, s, path) != nil {
		t.Fatal("undo still offered after undoing")
	}
}

func TestUndoNewConfigRemovesIt(t *testing.T) {
	s := fixture(t)
	path := filepath.Join(s.Home, ".claude/settings.json")
	rev := saveVia(t, s, path, "{}")
	if u := undoInfo(t, s, path); u["removes"] != true {
		t.Fatalf("undo info: %v", u)
	}
	if r := undoVia(t, s, path, rev); r.code != 200 || r.body["removed"] != true {
		t.Fatalf("undo: %d %v", r.code, r.body)
	}
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("created file still exists")
	}
}

func TestUndoNewSkill(t *testing.T) {
	for _, extra := range []bool{false, true} {
		t.Run(fmt.Sprint("extra file ", extra), func(t *testing.T) {
			s := fixture(t)
			created := callAPI(t, s, "POST", "skill", fmt.Sprintf(`{"agent":"claude","name":"demo","content":%q}`, skillDoc))
			if created.code != 201 {
				t.Fatalf("create: %d %v", created.code, created.body)
			}
			dir := filepath.Join(s.Home, ".claude/skills/demo")
			if extra {
				put(t, filepath.Join(dir, "notes.md"), "keep me")
			}
			if r := undoVia(t, s, filepath.Join(dir, "SKILL.md"), digest([]byte(skillDoc))); r.code != 200 {
				t.Fatalf("undo: %d %v", r.code, r.body)
			}
			if _, e := os.Stat(filepath.Join(dir, "SKILL.md")); !os.IsNotExist(e) {
				t.Fatal("SKILL.md still exists")
			}
			_, e := os.Stat(dir)
			if extra && e != nil {
				t.Fatal("folder with other files was removed")
			}
			if !extra && !os.IsNotExist(e) {
				t.Fatal("empty skill folder left behind")
			}
		})
	}
}

func TestUndoRefusedAfterOutsideChange(t *testing.T) {
	s := fixture(t)
	path := filepath.Join(s.Home, ".codex/AGENTS.md")
	put(t, path, "one\n")
	rev := saveVia(t, s, path, "two\n")
	if r := undoVia(t, s, path, "stale"); r.code != 409 {
		t.Fatalf("stale revision: %d %v", r.code, r.body)
	}
	put(t, path, "three\n")
	if undoInfo(t, s, path) != nil {
		t.Fatal("undo offered after outside change")
	}
	if r := undoVia(t, s, path, rev); r.code != 400 {
		t.Fatalf("undo after outside change: %d %v", r.code, r.body)
	}
	if b, _ := os.ReadFile(path); string(b) != "three\n" {
		t.Fatalf("outside change overwritten: %q", b)
	}
}
