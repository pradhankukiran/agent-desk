package desk

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCatalogJSONCSharedSkillsAndProject(t *testing.T) {
	s := fixture(t)
	put(t, filepath.Join(s.Home, ".config/opencode/opencode.jsonc"), "{ // comment\n \"mcp\": {\"demo\": {\"enabled\": false}}, }")
	put(t, filepath.Join(s.Home, ".agents/skills/test/SKILL.md"), "---\nname: test\ndescription: test\n---\n")
	project := filepath.Join(s.Home, "projects/example")
	put(t, filepath.Join(project, "AGENTS.md"), "project instructions")
	var open Agent
	for _, a := range s.Catalog(project) {
		if a.ID == "opencode" {
			open = a
		}
	}
	found := map[string]bool{}
	for _, x := range open.Extensions {
		found[x.Kind+":"+x.Name+":"+x.Status] = true
	}
	if !found["mcp:demo:disabled"] || !found["skill:test:present"] {
		t.Fatalf("missing extensions: %+v", open.Extensions)
	}
	for _, f := range open.Files {
		if f.Path == filepath.Join(project, "AGENTS.md") && f.Scope == "project" {
			return
		}
	}
	t.Fatal("project instructions not found")
}

func TestLookupReusesCatalog(t *testing.T) {
	s := fixture(t)
	agents := filepath.Join(s.Home, ".codex/AGENTS.md")
	put(t, agents, "# Notes\n")
	scans := func() int { s.cacheMu.Lock(); defer s.cacheMu.Unlock(); return s.scans }
	s.Catalog("")
	for i := 0; i < 3; i++ {
		if f, e := s.lookup("", fileID(agents)); e != nil || !f.Exists {
			t.Fatalf("lookup: %+v %v", f, e)
		}
	}
	if n := scans(); n != 1 {
		t.Fatalf("lookups rescanned: %d scans", n)
	}
	// State is refreshed from disk, not taken from the cached scan.
	os.Remove(agents)
	if f, _ := s.lookup("", fileID(agents)); f.Exists {
		t.Fatal("removed file reported as present")
	}
	// An unknown ID triggers one rescan, which finds a new skill.
	skill := filepath.Join(s.Home, ".codex/skills/new/SKILL.md")
	put(t, skill, skillDoc)
	if _, e := s.lookup("", fileID(skill)); e != nil {
		t.Fatalf("new skill not found: %v", e)
	}
	if _, e := s.lookup("", "unknown"); e == nil {
		t.Fatal("unknown ID found")
	}
	if n := scans(); n != 3 {
		t.Fatalf("want 3 scans, got %d", n)
	}
}

func TestPluginNameFromManifest(t *testing.T) {
	s := fixture(t)
	put(t, filepath.Join(s.Home, ".claude/plugins/cache/2da0c8d9/.claude-plugin/plugin.json"), `{"name":"engineering"}`)
	put(t, filepath.Join(s.Home, ".claude/plugins/cache/unnamed/.claude-plugin/plugin.json"), `{}`)
	names := map[string]bool{}
	for _, a := range s.Catalog("") {
		for _, x := range a.Extensions {
			if x.Kind == "plugin" {
				names[x.Name] = true
			}
		}
	}
	if !names["engineering"] || !names["unnamed"] || names["2da0c8d9"] {
		t.Fatalf("plugin names: %v", names)
	}
}
