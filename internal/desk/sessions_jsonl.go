package desk

import (
	"bufio"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// maxLine caps a single JSONL entry. Longer entries are skipped, not fatal.
var maxLine = 64 << 20

// scanLines calls fn for each JSON object line in the first limit bytes of
// path until fn returns false. Malformed lines are ignored. It returns the
// number of lines skipped for exceeding maxLine.
func scanLines(path string, limit int64, fn func(map[string]any) bool) (int, error) {
	f, e := os.Open(path)
	if e != nil {
		return 0, e
	}
	defer f.Close()
	r := bufio.NewReaderSize(io.LimitReader(f, limit), 64*1024)
	skipped := 0
	var line []byte
	tooLong := false
	for {
		chunk, e := r.ReadSlice('\n')
		if !tooLong {
			if len(line)+len(chunk) > maxLine {
				tooLong, line = true, nil
			} else {
				line = append(line, chunk...)
			}
		}
		if e == bufio.ErrBufferFull {
			continue
		}
		if tooLong {
			skipped++
		} else if len(line) > 0 {
			var d map[string]any
			if json.Unmarshal(line, &d) == nil && !fn(d) {
				return skipped, nil
			}
		}
		line, tooLong = line[:0], false
		if e == io.EOF {
			return skipped, nil
		}
		if e != nil {
			return skipped, e
		}
	}
}

// sessionFile is what the session list needs from one history file. It is
// cached and reused until the file's size or modification time changes.
type sessionFile struct {
	size    int64
	modTime time.Time
	item    Session
	metaID  string // Codex session ID, used to look up its title in the index
	warning string
}

// readSessionFile reads the start of a history file until it has found the
// session's folder and title, or has read 150 lines or 512 KB.
func readSessionFile(agent, p string, info os.FileInfo) sessionFile {
	base := strings.TrimSuffix(filepath.Base(p), ".jsonl")
	if agent == "grok" {
		base = filepath.Base(filepath.Dir(p))
	}
	sf := sessionFile{size: info.Size(), modTime: info.ModTime()}
	item := Session{ID: fileID(p), Agent: agent, Title: base, Path: p, Updated: info.ModTime().UTC().Format(time.RFC3339Nano), Format: "jsonl"}
	if agent == "grok" {
		item.Project, _ = url.PathUnescape(filepath.Base(filepath.Dir(filepath.Dir(p))))
	}
	count := 0
	_, e := scanLines(p, 512<<10, func(d map[string]any) bool {
		count++
		if agent == "codex" && str(d["type"]) == "session_meta" {
			meta := obj(d["payload"])
			item.Project = str(meta["cwd"])
			sf.metaID = str(meta["id"])
		}
		if cwd := str(d["cwd"]); cwd != "" {
			item.Project = cwd
		}
		for _, m := range normalize(agent, d) {
			if m.Role == "user" && item.Title == base && !strings.HasPrefix(m.Text, "# AGENTS.md") && !strings.HasPrefix(m.Text, "<") {
				item.Title = short(m.Text, 120)
			}
		}
		// Claude records the current folder on every line and it can change
		// mid-session, so Claude keeps the last one seen in the first 150 lines.
		done := agent != "claude" && item.Title != base && item.Project != "" && (agent != "codex" || sf.metaID != "")
		return !done && count < 150
	})
	if e != nil {
		sf.warning = filepath.Base(p) + ": " + e.Error()
	}
	sf.item = item
	return sf
}

func (s *Server) fileSessions(agent string) ([]Session, []string) {
	patterns := map[string]string{"codex": ".codex/sessions/*/*/*/*.jsonl", "claude": ".claude/projects/*/*.jsonl", "grok": ".grok/sessions/*/*/chat_history.jsonl"}
	paths, _ := filepath.Glob(filepath.Join(s.Home, patterns[agent]))
	if agent == "codex" {
		more, _ := filepath.Glob(filepath.Join(s.Home, ".codex/archived_sessions/*.jsonl"))
		paths = append(paths, more...)
	}
	titles := map[string]string{}
	if agent == "codex" {
		_, _ = scanLines(filepath.Join(s.Home, ".codex/session_index.jsonl"), 16<<20, func(d map[string]any) bool {
			id := str(d["id"])
			name := str(d["thread_name"])
			if name == "" {
				name = str(d["title"])
			}
			if id != "" && name != "" {
				titles[id] = name
			}
			return true
		})
	}
	files := s.readSessionFiles(agent, paths)
	out := []Session{}
	warnings := []string{}
	for _, sf := range files {
		item := sf.item
		if title := titles[sf.metaID]; sf.metaID != "" && title != "" {
			item.Title = title
		}
		if sf.warning != "" {
			warnings = append(warnings, sf.warning)
		}
		out = append(out, item)
	}
	return out, warnings
}

// readSessionFiles returns the list details of each existing file, reading
// only new or changed files and several at a time.
func (s *Server) readSessionFiles(agent string, paths []string) []sessionFile {
	s.cacheMu.Lock()
	if s.sessionCache == nil {
		s.sessionCache = map[string]sessionFile{}
	}
	cached := make(map[string]sessionFile, len(paths))
	for _, p := range paths {
		if sf, ok := s.sessionCache[p]; ok {
			cached[p] = sf
		}
	}
	s.cacheMu.Unlock()

	results := make([]sessionFile, len(paths))
	found := make([]bool, len(paths))
	jobs := make(chan int)
	var wg sync.WaitGroup
	workers := min(runtime.NumCPU(), 8)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				info, e := os.Stat(paths[i])
				if e != nil {
					continue
				}
				sf, ok := cached[paths[i]]
				if !ok || sf.size != info.Size() || !sf.modTime.Equal(info.ModTime()) {
					sf = readSessionFile(agent, paths[i], info)
				}
				results[i], found[i] = sf, true
			}
		}()
	}
	for i := range paths {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	out := []sessionFile{}
	prefix := filepath.Join(s.Home, sessionRoot[agent])
	s.cacheMu.Lock()
	// Drop this agent's entries for files that no longer exist.
	for p := range s.sessionCache {
		if strings.HasPrefix(p, prefix) {
			delete(s.sessionCache, p)
		}
	}
	for i, p := range paths {
		if found[i] {
			s.sessionCache[p] = results[i]
			out = append(out, results[i])
		}
	}
	s.cacheMu.Unlock()
	return out
}

// sessionRoot is the folder holding each agent's history files.
var sessionRoot = map[string]string{"codex": ".codex" + string(filepath.Separator), "claude": ".claude" + string(filepath.Separator), "grok": ".grok" + string(filepath.Separator)}
