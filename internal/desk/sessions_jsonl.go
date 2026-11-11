package desk

import (
	"bufio"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

func (s *Server) fileSessions(agent string) ([]Session, []string) {
	patterns := map[string]string{"codex": ".codex/sessions/*/*/*/*.jsonl", "claude": ".claude/projects/*/*.jsonl", "grok": ".grok/sessions/*/*/chat_history.jsonl"}
	paths, _ := filepath.Glob(filepath.Join(s.Home, patterns[agent]))
	if agent == "codex" {
		more, _ := filepath.Glob(filepath.Join(s.Home, ".codex/archived_sessions/*.jsonl"))
		paths = append(paths, more...)
	}
	out := []Session{}
	warnings := []string{}
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
	for _, p := range paths {
		info, e := os.Stat(p)
		if e != nil {
			continue
		}
		base := strings.TrimSuffix(filepath.Base(p), ".jsonl")
		if agent == "grok" {
			base = filepath.Base(filepath.Dir(p))
		}
		item := Session{ID: fileID(p), Agent: agent, Title: base, Path: p, Updated: info.ModTime().UTC().Format(time.RFC3339Nano), Format: "jsonl"}
		if agent == "grok" {
			item.Project, _ = url.PathUnescape(filepath.Base(filepath.Dir(filepath.Dir(p))))
		}
		count := 0
		_, e = scanLines(p, 512<<10, func(d map[string]any) bool {
			count++
			if agent == "codex" && str(d["type"]) == "session_meta" {
				meta := obj(d["payload"])
				item.Project = str(meta["cwd"])
				if title := titles[str(meta["id"])]; title != "" {
					item.Title = title
				}
			}
			if cwd := str(d["cwd"]); cwd != "" {
				item.Project = cwd
			}
			for _, m := range normalize(agent, d) {
				if m.Role == "user" && item.Title == base && !strings.HasPrefix(m.Text, "# AGENTS.md") && !strings.HasPrefix(m.Text, "<") {
					item.Title = short(m.Text, 120)
				}
			}
			return count < 150
		})
		if e != nil {
			warnings = append(warnings, filepath.Base(p)+": "+e.Error())
		}
		out = append(out, item)
	}
	return out, warnings
}
