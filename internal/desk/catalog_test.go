package desk

import (
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
