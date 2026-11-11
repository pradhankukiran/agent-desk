package desk

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// sessionDirs turns a recorded working folder into plain paths. Agy records a
// JSON list of file:// URIs; other agents record a single path.
func sessionDirs(recorded string) []string {
	raw := []string{recorded}
	if strings.HasPrefix(recorded, "[") {
		raw = nil
		_ = json.Unmarshal([]byte(recorded), &raw)
	}
	dirs := []string{}
	for _, d := range raw {
		if strings.HasPrefix(d, "file://") {
			u, e := url.Parse(d)
			if e != nil {
				continue
			}
			d = u.Path
		}
		if d != "" {
			dirs = append(dirs, filepath.Clean(d))
		}
	}
	return dirs
}

// inProject reports whether the session ran in project or a folder inside it.
// project must already have symbolic links resolved; cache holds resolved dirs.
func inProject(item Session, project string, cache map[string]string) bool {
	dirs := item.dirs
	if dirs == nil {
		dirs = sessionDirs(item.Project)
	}
	for _, d := range dirs {
		if !filepath.IsAbs(d) {
			continue
		}
		real, ok := cache[d]
		if !ok {
			real = d
			if r, e := filepath.EvalSymlinks(d); e == nil {
				real = r
			}
			cache[d] = real
		}
		if within(project, real) {
			return true
		}
	}
	return false
}

func (s *Server) listSessions(agent string) ([]Session, []string, error) {
	switch agent {
	case "codex", "claude", "grok":
		a, b := s.fileSessions(agent)
		return a, b, nil
	case "opencode", "agy":
		a, b := s.dbSessions(agent)
		return a, b, nil
	}
	return nil, nil, fmt.Errorf("unknown agent")
}

func (s *Server) sessionsHandler(w http.ResponseWriter, r *http.Request) {
	items, warnings, e := s.listSessions(r.URL.Query().Get("agent"))
	if e != nil {
		failure(w, e)
		return
	}
	project, e := s.project(r.URL.Query().Get("project"))
	if e != nil {
		failure(w, e)
		return
	}
	query := strings.ToLower(r.URL.Query().Get("q"))
	filtered := []Session{}
	resolved := map[string]string{}
	for _, item := range items {
		if project != "" && !inProject(item, project, resolved) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(item.Title+" "+item.Project+" "+item.ID), query) {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Updated > filtered[j].Updated })
	reply(w, 200, map[string]any{"sessions": filtered, "warnings": warnings})
}

const (
	defaultPage = 200
	maxPage     = 1000
)

// page filters a transcript's messages by the q parameter and returns the
// limit messages starting at offset, so large conversations load in parts.
func page(t Transcript, q url.Values) Transcript {
	if query := strings.ToLower(q.Get("q")); query != "" {
		kept := []Message{}
		for _, m := range t.Messages {
			if strings.Contains(strings.ToLower(m.Text), query) {
				kept = append(kept, m)
			}
		}
		t.Messages = kept
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	limit, e := strconv.Atoi(q.Get("limit"))
	if e != nil || limit <= 0 {
		limit = defaultPage
	}
	limit = min(limit, maxPage)
	t.Total = len(t.Messages)
	t.Offset = min(max(offset, 0), t.Total)
	t.Messages = t.Messages[t.Offset:min(t.Offset+limit, t.Total)]
	return t
}

func (s *Server) sessionHandler(w http.ResponseWriter, r *http.Request) {
	items, _, e := s.listSessions(r.URL.Query().Get("agent"))
	if e != nil {
		failure(w, e)
		return
	}
	for _, item := range items {
		if item.ID == r.URL.Query().Get("id") {
			t, e := s.transcript(item)
			if e != nil {
				failure(w, e)
				return
			}
			reply(w, 200, page(t, r.URL.Query()))
			return
		}
	}
	failure(w, os.ErrNotExist)
}
