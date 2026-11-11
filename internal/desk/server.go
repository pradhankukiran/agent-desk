package desk

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func failure(w http.ResponseWriter, e error) {
	status := 400
	if errors.Is(e, fs.ErrNotExist) {
		status = 404
	}
	if errors.Is(e, errConflict) {
		status = 409
	}
	reply(w, status, map[string]string{"error": e.Error()})
}

func (s *Server) Handler(assets fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/catalog", func(w http.ResponseWriter, r *http.Request) {
		p, e := s.project(r.URL.Query().Get("project"))
		if e != nil {
			failure(w, e)
			return
		}
		projects := []string{s.Home}
		entries, _ := os.ReadDir(filepath.Join(s.Home, "projects"))
		for _, x := range entries {
			if x.IsDir() {
				projects = append(projects, filepath.Join(s.Home, "projects", x.Name()))
			}
		}
		reply(w, 200, map[string]any{"agents": s.Catalog(p), "projects": projects, "home": s.Home})
	})
	mux.HandleFunc("GET /api/file", func(w http.ResponseWriter, r *http.Request) {
		f, e := s.requestFile(r)
		if e != nil {
			failure(w, e)
			return
		}
		b, e := readSmall(f.Path)
		if errors.Is(e, fs.ErrNotExist) {
			b = []byte{}
			e = nil
		}
		if e != nil {
			failure(w, e)
			return
		}
		reply(w, 200, map[string]any{"file": f, "content": string(b), "revision": revision(f.Path, b)})
	})
	mux.HandleFunc("PUT /api/file", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		f, e := s.requestFile(r)
		if e != nil {
			failure(w, e)
			return
		}
		var body struct{ Content, Revision string }
		if e = decode(w, r, &body); e != nil {
			failure(w, e)
			return
		}
		backup, e := s.save(f, []byte(body.Content), body.Revision, false)
		if e != nil {
			failure(w, e)
			return
		}
		reply(w, 200, map[string]string{"revision": digest([]byte(body.Content)), "backup": backup})
	})
	mux.HandleFunc("DELETE /api/file", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		f, e := s.requestFile(r)
		if e != nil {
			failure(w, e)
			return
		}
		if f.Kind != "skill" && f.Kind != "instruction" {
			failure(w, fmt.Errorf("only skills and instruction files can be removed here"))
			return
		}
		var body struct{ Revision string }
		if e = decode(w, r, &body); e != nil {
			failure(w, e)
			return
		}
		backup, e := s.save(f, nil, body.Revision, true)
		if e != nil {
			failure(w, e)
			return
		}
		reply(w, 200, map[string]string{"backup": backup})
	})
	mux.HandleFunc("POST /api/skill", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		var body struct{ Agent, Project, Name, Content string }
		if e := decode(w, r, &body); e != nil {
			failure(w, e)
			return
		}
		p, e := s.project(body.Project)
		if e != nil {
			failure(w, e)
			return
		}
		if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`).MatchString(body.Name) {
			failure(w, fmt.Errorf("use lowercase letters, numbers, and hyphens for the skill name"))
			return
		}
		root := ""
		for _, sp := range specs() {
			if sp.id == body.Agent {
				if p != "" {
					root = filepath.Join(p, sp.projectSkills[0])
				} else {
					root = filepath.Join(s.Home, sp.skills[0])
				}
			}
		}
		if root == "" {
			failure(w, fmt.Errorf("unknown agent"))
			return
		}
		path := filepath.Join(root, body.Name, "SKILL.md")
		f := File{Path: path, Kind: "skill", Editable: true}
		if _, e = os.Stat(path); e == nil {
			failure(w, fmt.Errorf("skill already exists"))
			return
		}
		_, e = s.save(f, []byte(body.Content), "missing", false)
		if e != nil {
			failure(w, e)
			return
		}
		reply(w, 201, map[string]string{"id": fileID(path)})
	})
	mux.HandleFunc("GET /api/sessions", s.sessionsHandler)
	mux.HandleFunc("GET /api/session", s.sessionHandler)
	mux.Handle("/", http.FileServer(http.FS(assets)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'")
		host := r.Host
		if i := strings.LastIndex(host, ":"); i >= 0 {
			host = host[:i]
		}
		if host != "127.0.0.1" && host != "localhost" {
			reply(w, 403, map[string]string{"error": "local access only"})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			supplied := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(supplied), []byte(s.Token)) != 1 {
				reply(w, 401, map[string]string{"error": "Open the app using the link printed in your terminal."})
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" {
				u, e := url.Parse(origin)
				if e != nil || u.Host != r.Host || u.Scheme != "http" {
					reply(w, 403, map[string]string{"error": "foreign origin rejected"})
					return
				}
			}
		}
		mux.ServeHTTP(w, r)
	})
}

func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxFile+4096)
	dec := json.NewDecoder(r.Body)
	if e := dec.Decode(v); e != nil {
		return e
	}
	var extra any
	if e := dec.Decode(&extra); e != io.EOF {
		return fmt.Errorf("expected one JSON object")
	}
	return nil
}

func (s *Server) requestFile(r *http.Request) (File, error) {
	p, e := s.project(r.URL.Query().Get("project"))
	if e != nil {
		return File{}, e
	}
	f, e := s.lookup(p, r.URL.Query().Get("id"))
	if e != nil {
		return f, e
	}
	resolved, err := filepath.EvalSymlinks(f.Path)
	if err == nil && !within(s.Home, resolved) {
		return File{}, fmt.Errorf("file resolves outside home directory")
	}
	return f, nil
}
