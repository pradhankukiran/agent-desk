package desk

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dotfiles links ~/.claude to a folder elsewhere in home, and ~/.codex/AGENTS.md
// to a single file, as dotfile managers do.
func dotfiles(t *testing.T) *Server {
	t.Helper()
	s := fixture(t)
	put(t, filepath.Join(s.Home, "dotfiles/claude/settings.json"), "{}")
	put(t, filepath.Join(s.Home, "dotfiles/claude/skills/demo/SKILL.md"), skillDoc)
	put(t, filepath.Join(s.Home, "dotfiles/codex-agents.md"), "# Notes\n")
	os.MkdirAll(filepath.Join(s.Home, ".codex"), 0700)
	if e := os.Symlink(filepath.Join(s.Home, "dotfiles/claude"), filepath.Join(s.Home, ".claude")); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(filepath.Join(s.Home, "dotfiles/codex-agents.md"), filepath.Join(s.Home, ".codex/AGENTS.md")); e != nil {
		t.Fatal(e)
	}
	return s
}

func TestEditThroughLinks(t *testing.T) {
	s := dotfiles(t)
	for _, c := range []struct{ link, real, content string }{
		{".claude/settings.json", "dotfiles/claude/settings.json", `{"theme":"light"}`},
		{".claude/CLAUDE.md", "dotfiles/claude/CLAUDE.md", "# New file\n"},
		{".codex/AGENTS.md", "dotfiles/codex-agents.md", "# Changed\n"},
	} {
		link := filepath.Join(s.Home, c.link)
		f, e := s.lookup("", fileID(link))
		if e != nil || !f.Editable || f.Target != filepath.Join(s.Home, c.real) {
			t.Fatalf("%s: %+v %v", c.link, f, e)
		}
		saveVia(t, s, link, c.content)
		if b, _ := os.ReadFile(filepath.Join(s.Home, c.real)); string(b) != c.content {
			t.Fatalf("%s: real file has %q", c.link, b)
		}
	}
	// The links themselves are still links.
	for _, l := range []string{".claude", ".codex/AGENTS.md"} {
		if info, e := os.Lstat(filepath.Join(s.Home, l)); e != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s is no longer a link", l)
		}
	}
}

func TestRemoveThroughLinks(t *testing.T) {
	s := dotfiles(t)
	// A skill inside a linked folder is removed from the real folder.
	skill := filepath.Join(s.Home, ".claude/skills/demo/SKILL.md")
	if _, r := openAndRemove(t, s, skill, ""); r.code != 200 {
		t.Fatalf("remove skill: %d %v", r.code, r.body)
	}
	if _, e := os.Stat(filepath.Join(s.Home, "dotfiles/claude/skills/demo")); !os.IsNotExist(e) {
		t.Fatal("real skill folder still exists")
	}
	// A file that is itself a link is not removed.
	link := filepath.Join(s.Home, ".codex/AGENTS.md")
	opened := callAPI(t, s, "GET", "file?id="+fileID(link), "")
	r := callAPI(t, s, "DELETE", "file?id="+fileID(link), fmt.Sprintf(`{"revision":%q}`, opened.body["revision"]))
	if r.code != 400 || !strings.Contains(fmt.Sprint(r.body["error"]), "symbolic link") {
		t.Fatalf("remove link: %d %v", r.code, r.body)
	}
	if _, e := os.Stat(filepath.Join(s.Home, "dotfiles/codex-agents.md")); e != nil {
		t.Fatal("linked file target was removed")
	}
}
