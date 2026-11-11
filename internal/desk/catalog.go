package desk

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func (s *Server) Catalog(project string) []Agent {
	out := []Agent{}
	for _, sp := range specs() {
		a := Agent{ID: sp.id, Name: sp.name, Files: []File{}, Extensions: []Extension{}, Warnings: []string{}}
		a.Executable, _ = exec.LookPath(sp.id)
		a.Installed = a.Executable != ""
		seen := map[string]bool{}
		add := func(path, kind, scope string, editable bool) {
			path = filepath.Clean(path)
			if seen[path] {
				return
			}
			seen[path] = true
			f := File{ID: fileID(path), Agent: sp.id, Name: filepath.Base(path), Path: path, Kind: kind, Scope: scope, Editable: editable}
			if info, e := os.Stat(path); e == nil && !info.IsDir() {
				f.Exists = true
				f.Size = info.Size()
			}
			// Never edit a symlink through its alias, nor files outside the user's home.
			if resolved, e := filepath.EvalSymlinks(path); e == nil && (!within(s.Home, resolved) || resolved != path) {
				f.Editable = false
			}
			a.Files = append(a.Files, f)
			if f.Exists && (kind == "skill" || kind == "plugin" || kind == "agent" || kind == "command" || kind == "rule") {
				name := strings.TrimSuffix(f.Name, filepath.Ext(f.Name))
				if kind == "skill" || f.Name == "plugin.json" {
					name = filepath.Base(filepath.Dir(path))
					if strings.HasPrefix(name, ".") {
						name = filepath.Base(filepath.Dir(filepath.Dir(path)))
					}
				}
				a.Extensions = append(a.Extensions, Extension{Name: name, Kind: kind, Status: "present", Scope: scope, FileID: f.ID, Source: path})
			}
		}
		walk := func(root, kind, scope string, editable bool) {
			var visit func(string, int)
			visited := map[string]bool{}
			visit = func(dir string, depth int) {
				if depth > 7 {
					return
				}
				real, e := filepath.EvalSymlinks(dir)
				if e != nil || !within(s.Home, real) || visited[real] {
					return
				}
				visited[real] = true
				entries, _ := os.ReadDir(dir)
				for _, entry := range entries {
					n := entry.Name()
					if n == "node_modules" || n == ".git" || n == "assets" || n == "references" || n == "scripts" {
						continue
					}
					p := filepath.Join(dir, n)
					info, e := os.Stat(p)
					if e != nil {
						continue
					}
					if info.IsDir() {
						visit(p, depth+1)
						continue
					}
					match := kind == "skill" && n == "SKILL.md" || kind == "plugin" && (n == "plugin.json" || sp.id == "opencode" && depth == 0 && (strings.HasSuffix(n, ".ts") || strings.HasSuffix(n, ".js"))) || (kind == "agent" || kind == "command" || kind == "rule") && (strings.HasSuffix(n, ".md") || strings.HasSuffix(n, ".toml"))
					if match {
						add(p, kind, scope, editable)
					}
				}
			}
			visit(root, 0)
		}
		for _, p := range sp.configs {
			add(filepath.Join(s.Home, p), "config", "user", !strings.Contains(p, "managed_config") && !strings.Contains(p, "requirements"))
		}
		for _, p := range sp.instructions {
			add(filepath.Join(s.Home, p), "instruction", "user", true)
		}
		for _, p := range sp.skills {
			scope := "user"
			editable := true
			if strings.Contains(p, "builtin") || strings.Contains(p, "bundled") {
				scope = "bundled"
				editable = false
			}
			if strings.HasPrefix(p, ".agents/") {
				scope = "shared"
			}
			walk(filepath.Join(s.Home, p), "skill", scope, editable)
		}
		for _, p := range sp.plugins {
			walk(filepath.Join(s.Home, p), "plugin", "installed", false)
			walk(filepath.Join(s.Home, p), "skill", "plugin", false)
		}
		for _, pair := range [][2]string{{"agents", "agent"}, {"commands", "command"}, {"rules", "rule"}} {
			walk(filepath.Join(s.Home, sp.root, pair[0]), pair[1], "user", true)
		}
		if project != "" {
			for _, p := range sp.projectConfigs {
				add(filepath.Join(project, p), "config", "project", true)
			}
			for _, p := range sp.projectInstructions {
				add(filepath.Join(project, p), "instruction", "project", true)
			}
			for _, p := range sp.projectSkills {
				walk(filepath.Join(project, p), "skill", "project", true)
			}
			for _, pair := range [][2]string{{"agents", "agent"}, {"commands", "command"}, {"rules", "rule"}} {
				root := "." + sp.id
				if sp.id == "agy" {
					root = ".agents"
				}
				walk(filepath.Join(project, root, pair[0]), pair[1], "project", true)
			}
		}
		// Home instructions are useful even when the selected project is deeper.
		for _, name := range sp.projectInstructions {
			if !strings.Contains(name, "/") {
				p := filepath.Join(s.Home, name)
				if _, e := os.Stat(p); e == nil {
					add(p, "instruction", "home", true)
				}
			}
		}
		for _, f := range a.Files {
			if f.Kind != "config" || !f.Exists {
				continue
			}
			resolved, resolveErr := filepath.EvalSymlinks(f.Path)
			if resolveErr != nil || !within(s.Home, resolved) {
				a.Warnings = append(a.Warnings, f.Path+": cannot read configuration outside home")
				continue
			}
			b, e := readSmall(f.Path)
			if e != nil {
				a.Warnings = append(a.Warnings, f.Name+": "+e.Error())
				continue
			}
			if len(strings.TrimSpace(string(b))) == 0 {
				continue
			}
			d, e := parseConfig(f.Path, b)
			if e != nil {
				a.Warnings = append(a.Warnings, f.Path+": invalid configuration: "+e.Error())
				continue
			}
			extract := func(d map[string]any, scope string) {
				for _, key := range []string{"mcp_servers", "mcpServers", "mcp"} {
					if m, ok := d[key].(map[string]any); ok {
						for n, v := range m {
							status := "configured"
							if props, ok := v.(map[string]any); ok {
								if props["enabled"] == false || props["disabled"] == true {
									status = "disabled"
								}
							}
							a.Extensions = append(a.Extensions, Extension{n, "mcp", status, scope, f.ID, f.Path})
						}
					}
				}
				for _, key := range []string{"plugins", "enabledPlugins", "plugin"} {
					switch m := d[key].(type) {
					case map[string]any:
						for n, v := range m {
							status := "configured"
							if v == false {
								status = "disabled"
							}
							a.Extensions = append(a.Extensions, Extension{n, "plugin", status, scope, f.ID, f.Path})
						}
					case []any:
						for _, v := range m {
							if n, ok := v.(string); ok {
								a.Extensions = append(a.Extensions, Extension{n, "plugin", "configured", scope, f.ID, f.Path})
							}
						}
					}
				}
				if hooks, ok := d["hooks"].(map[string]any); ok {
					for n := range hooks {
						a.Extensions = append(a.Extensions, Extension{n, "hook", "configured", scope, f.ID, f.Path})
					}
				}
			}
			extract(d, f.Scope)
			if project != "" && filepath.Base(f.Path) == ".claude.json" {
				if projects, ok := d["projects"].(map[string]any); ok {
					if cfg, ok := projects[project].(map[string]any); ok {
						extract(cfg, "project")
					}
				}
			}
		}
		sort.Slice(a.Files, func(i, j int) bool { return a.Files[i].Path < a.Files[j].Path })
		sort.Slice(a.Extensions, func(i, j int) bool {
			return a.Extensions[i].Kind+a.Extensions[i].Name < a.Extensions[j].Kind+a.Extensions[j].Name
		})
		out = append(out, a)
	}
	return out
}

func (s *Server) lookup(project, id string) (File, error) {
	for _, a := range s.Catalog(project) {
		for _, f := range a.Files {
			if f.ID == id {
				return f, nil
			}
		}
	}
	return File{}, fs.ErrNotExist
}
