package desk

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
)

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
	for _, item := range items {
		if project != "" && !strings.Contains(item.Project, project) {
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
			reply(w, 200, t)
			return
		}
	}
	failure(w, os.ErrNotExist)
}
