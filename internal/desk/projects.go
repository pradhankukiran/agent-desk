package desk

import (
	"os"
	"path/filepath"
	"sort"
)

const maxProjects = 100

// projects lists folders to offer as project scopes: the home folder, then
// folders the agents recorded working in (most recent first), then folders
// inside ~/projects. Only existing folders inside home are included.
func (s *Server) projects() []string {
	latest := map[string]string{} // folder -> most recent session time
	note := func(dir, at string) {
		if dir == "" || !filepath.IsAbs(dir) {
			return
		}
		real, e := filepath.EvalSymlinks(dir)
		if e != nil || real == s.Home || !within(s.Home, real) {
			return
		}
		if info, e := os.Stat(real); e != nil || !info.IsDir() {
			return
		}
		if at > latest[real] {
			latest[real] = at
		} else if _, ok := latest[real]; !ok {
			latest[real] = at
		}
	}
	for _, sp := range specs() {
		items, _, _ := s.listSessions(sp.id)
		for _, item := range items {
			dirs := item.dirs
			if dirs == nil {
				dirs = sessionDirs(item.Project)
			}
			for _, d := range dirs {
				note(d, item.Updated)
			}
		}
	}
	// Claude Code keeps per-folder settings for every folder it has opened.
	if b, e := readSmall(filepath.Join(s.Home, ".claude.json")); e == nil {
		if d, e := parseConfig(".claude.json", b); e == nil {
			if m, ok := d["projects"].(map[string]any); ok {
				for dir := range m {
					note(dir, "")
				}
			}
		}
	}
	entries, _ := os.ReadDir(filepath.Join(s.Home, "projects"))
	for _, x := range entries {
		if x.IsDir() {
			note(filepath.Join(s.Home, "projects", x.Name()), "")
		}
	}
	list := make([]string, 0, len(latest))
	for dir := range latest {
		list = append(list, dir)
	}
	sort.Slice(list, func(i, j int) bool {
		if latest[list[i]] != latest[list[j]] {
			return latest[list[i]] > latest[list[j]]
		}
		return list[i] < list[j]
	})
	if len(list) > maxProjects {
		list = list[:maxProjects]
	}
	return append([]string{s.Home}, list...)
}
