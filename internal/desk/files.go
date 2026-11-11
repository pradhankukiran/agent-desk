package desk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxFile = 4 << 20

var errConflict = errors.New("file changed since you opened it; reload before saving")

func readSmall(path string) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	b, e := io.ReadAll(io.LimitReader(f, maxFile+1))
	if len(b) > maxFile {
		return nil, fmt.Errorf("file exceeds the 4 MB editor limit")
	}
	return b, e
}

func revision(path string, b []byte) string {
	if _, e := os.Stat(path); errors.Is(e, fs.ErrNotExist) {
		return "missing"
	}
	return digest(b)
}

// realPath resolves symbolic links in path, also for a file that does not
// exist yet, and requires the result to be inside home.
func (s *Server) realPath(path string) (string, error) {
	rest := []string{}
	p := filepath.Clean(path)
	for {
		r, e := filepath.EvalSymlinks(p)
		if e == nil {
			for i := len(rest) - 1; i >= 0; i-- {
				r = filepath.Join(r, rest[i])
			}
			if !within(s.Home, r) {
				return "", fmt.Errorf("file resolves outside home directory")
			}
			return r, nil
		}
		if !errors.Is(e, fs.ErrNotExist) {
			return "", e
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", e
		}
		rest = append(rest, filepath.Base(p))
		p = parent
	}
}

func (s *Server) safeWrite(path string) error {
	if !within(s.Home, path) {
		return fmt.Errorf("outside home directory")
	}
	p := path
	for {
		info, e := os.Lstat(p)
		if e == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("editing through symbolic links is not supported")
			}
		} else if !errors.Is(e, fs.ErrNotExist) {
			return e
		}
		if p == s.Home {
			break
		}
		parent := filepath.Dir(p)
		if parent == p {
			return fmt.Errorf("outside home directory")
		}
		p = parent
	}
	return nil
}

// backupDir returns the backup folder, creating it if needed.
func (s *Server) backupDir() (string, error) {
	dir := filepath.Join(s.Home, ".local/state/agent-desk/backups")
	if e := s.safeWrite(dir); e != nil {
		return "", e
	}
	return dir, os.MkdirAll(dir, 0700)
}

const stampLayout = "20060102T150405.000000000"

func stamp() string { return time.Now().UTC().Format(stampLayout) }

// Backups are kept if they are among the newest keepBackups for their file,
// or younger than keepBackupDays.
const (
	keepBackups    = 20
	keepBackupDays = 30
)

// pruneBackups deletes old backups. Backup names start with a timestamp and
// the source file's ID; names that do not match are left alone.
func pruneBackups(dir string, now time.Time) {
	entries, e := os.ReadDir(dir)
	if e != nil {
		return
	}
	type backup struct {
		name string
		at   time.Time
	}
	byFile := map[string][]backup{}
	for _, entry := range entries {
		n := entry.Name()
		if len(n) < len(stampLayout)+1+24 || n[len(stampLayout)] != '-' {
			continue
		}
		at, e := time.Parse(stampLayout, n[:len(stampLayout)])
		if e != nil {
			continue
		}
		id := n[len(stampLayout)+1 : len(stampLayout)+25]
		byFile[id] = append(byFile[id], backup{n, at})
	}
	cutoff := now.Add(-keepBackupDays * 24 * time.Hour)
	for _, list := range byFile {
		sort.Slice(list, func(i, j int) bool { return list[i].at.After(list[j].at) })
		for i, b := range list {
			if i >= keepBackups && b.at.Before(cutoff) {
				os.RemoveAll(filepath.Join(dir, b.name))
			}
		}
	}
}

// save validates content, backs up the current file, and replaces or removes
// it if its revision still matches expected.
func (s *Server) save(f File, content []byte, expected string, remove bool) (string, error) {
	return s.write(f, content, expected, remove, true)
}

// write is save with optional validation; restores skip it so the original
// content always comes back as it was.
func (s *Server) write(f File, content []byte, expected string, remove, validate bool) (string, error) {
	if !f.Editable {
		return "", fmt.Errorf("this file is read-only")
	}
	if remove {
		if info, e := os.Lstat(f.Path); e == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("this file is a symbolic link; remove it manually")
		}
	}
	// Write to the real file so links in the path, such as a linked
	// dotfiles folder, stay in place.
	real, e := s.realPath(f.Path)
	if e != nil {
		return "", e
	}
	f.Path = real
	if e := s.safeWrite(f.Path); e != nil {
		return "", e
	}
	if len(content) > maxFile {
		return "", fmt.Errorf("file exceeds 4 MB")
	}
	old, err := readSmall(f.Path)
	e = err
	exists := e == nil
	if e != nil && !errors.Is(e, fs.ErrNotExist) {
		return "", e
	}
	if revision(f.Path, old) != expected {
		return "", errConflict
	}
	if validate && !remove && f.Kind == "config" {
		if _, e := parseConfig(f.Path, content); e != nil {
			return "", fmt.Errorf("invalid configuration: %w", e)
		}
	}
	if validate && !remove && f.Kind == "skill" {
		if e := checkSkill(content); e != nil {
			return "", e
		}
	}
	mode := fs.FileMode(0600)
	backup := ""
	if exists {
		info, e := os.Stat(f.Path)
		if e != nil {
			return "", e
		}
		mode = info.Mode().Perm()
		dir, e := s.backupDir()
		if e != nil {
			return "", e
		}
		backup = filepath.Join(dir, stamp()+"-"+fileID(f.Path)+".json")
		data, _ := json.Marshal(map[string]string{"path": f.Path, "content": string(old), "savedAt": time.Now().UTC().Format(time.RFC3339)})
		if e = os.WriteFile(backup, data, 0600); e != nil {
			return "", e
		}
		pruneBackups(dir, time.Now())
	}
	if remove {
		if !exists {
			return "", fs.ErrNotExist
		}
		current, err := readSmall(f.Path)
		if err != nil {
			return "", err
		}
		if digest(current) != expected {
			return "", errConflict
		}
		return backup, os.Remove(f.Path)
	}
	if e = os.MkdirAll(filepath.Dir(f.Path), 0700); e != nil {
		return "", e
	}
	tmp, e := os.CreateTemp(filepath.Dir(f.Path), ".agent-desk-*")
	if e != nil {
		return "", e
	}
	defer os.Remove(tmp.Name())
	if e = tmp.Chmod(mode); e == nil {
		_, e = tmp.Write(content)
	}
	if e == nil {
		e = tmp.Sync()
	}
	closeErr := tmp.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return "", e
	}
	// Recheck immediately before replacement to catch edits during validation/backup.
	current, e := readSmall(f.Path)
	if e != nil && !errors.Is(e, fs.ErrNotExist) {
		return "", e
	}
	if revision(f.Path, current) != expected {
		return "", errConflict
	}
	return backup, os.Rename(tmp.Name(), f.Path)
}

// checkSkill requires a SKILL.md to start with a YAML frontmatter block that
// has a non-empty top-level description. Both LF and CRLF line endings work.
func checkSkill(content []byte) error {
	text := strings.TrimPrefix(string(content), "\ufeff")
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], " \t") != "---" {
		return fmt.Errorf("skills must start with a frontmatter block opened by a --- line")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " \t") == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return fmt.Errorf("the skill frontmatter block needs a closing --- line")
	}
	header := lines[1:end]
	for i, line := range header {
		value, ok := strings.CutPrefix(line, "description:")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimRight(value, "+-") {
		case "", "|", ">":
			// The text is on the following indented lines.
			for _, next := range header[i+1:] {
				if next != "" && next[0] != ' ' && next[0] != '\t' {
					break
				}
				if strings.TrimSpace(next) != "" {
					return nil
				}
			}
		case `""`, `''`:
		default:
			return nil
		}
		return fmt.Errorf("the skill description is empty")
	}
	return fmt.Errorf("the skill frontmatter needs a description")
}
