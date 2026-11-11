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
		f, p, e := s.requestFile(r)
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
		out := map[string]any{"file": f, "content": string(b), "revision": revision(f.Path, b)}
		s.mu.Lock()
		if rec, ok := s.undoFor(f.Path); ok {
			out["undo"] = map[string]any{"content": string(rec.previous), "removes": !rec.existed}
		}
		s.mu.Unlock()
		if f.Kind == "skill" && f.Exists && f.Editable {
			if _, n, e := s.skillFolder(f, p); e != nil {
				out["skillProblem"] = e.Error()
			} else {
				out["skillFiles"] = n
			}
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("PUT /api/file", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		f, _, e := s.requestFile(r)
		if e != nil {
			failure(w, e)
			return
		}
		// Base is the content the editor opened; it lets JSON edits merge with
		// changes another program made in the meantime.
		var body struct{ Content, Revision, Base string }
		if e = decode(w, r, &body); e != nil {
			failure(w, e)
			return
		}
		content := []byte(body.Content)
		rec, remembered := s.remember(f.Path, body.Revision)
		backup, e := s.save(f, content, body.Revision, false)
		merged := false
		if errors.Is(e, errConflict) && f.Kind == "config" && strings.HasSuffix(f.Path, ".json") && body.Revision != "missing" && digest([]byte(body.Base)) == body.Revision {
			disk, rev, err := current(f.Path)
			if err == nil && rev != "missing" {
				if content, e = mergeJSON([]byte(body.Base), []byte(body.Content), disk); e == nil {
					rec, remembered = s.remember(f.Path, rev)
					backup, e = s.save(f, content, rev, false)
					merged = true
				}
			}
		}
		if e != nil {
			failure(w, e)
			return
		}
		if remembered {
			rec.after = digest(content)
			s.undo[f.Path] = rec
		}
		reply(w, 200, map[string]any{"revision": digest(content), "backup": backup, "merged": merged})
	})
	mux.HandleFunc("DELETE /api/file", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		f, p, e := s.requestFile(r)
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
		var backup string
		if f.Kind == "skill" {
			backup, e = s.removeSkill(f, p, body.Revision)
		} else {
			backup, e = s.save(f, nil, body.Revision, true)
		}
		if e != nil {
			failure(w, e)
			return
		}
		delete(s.undo, f.Path)
		reply(w, 200, map[string]string{"backup": backup})
	})
	mux.HandleFunc("POST /api/undo", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		f, _, e := s.requestFile(r)
		if e != nil {
			failure(w, e)
			return
		}
		var body struct{ Revision string }
		if e = decode(w, r, &body); e != nil {
			failure(w, e)
			return
		}
		removed, e := s.restore(f, body.Revision)
		if e != nil {
			failure(w, e)
			return
		}
		reply(w, 200, map[string]bool{"removed": removed})
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
		newDir := skillDirIsNew(path)
		_, e = s.save(f, []byte(body.Content), "missing", false)
		if e != nil {
			failure(w, e)
			return
		}
		s.undo[path] = undoRecord{after: digest([]byte(body.Content)), newDir: newDir}
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
	r.Body = http.MaxBytesReader(w, r.Body, 2*maxFile+4096)
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

// requestFile finds the file named by the request and the resolved project.
func (s *Server) requestFile(r *http.Request) (File, string, error) {
	p, e := s.project(r.URL.Query().Get("project"))
	if e != nil {
		return File{}, "", e
	}
	f, e := s.lookup(p, r.URL.Query().Get("id"))
	if e != nil {
		return f, "", e
	}
	resolved, err := filepath.EvalSymlinks(f.Path)
	if err == nil && !within(s.Home, resolved) {
		return File{}, "", fmt.Errorf("file resolves outside home directory")
	}
	return f, p, nil
}
