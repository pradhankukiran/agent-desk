package desk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
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

func (s *Server) save(f File, content []byte, expected string, remove bool) (string, error) {
	if !f.Editable {
		return "", fmt.Errorf("this file is read-only")
	}
	if e := s.safeWrite(f.Path); e != nil {
		return "", e
	}
	if len(content) > maxFile {
		return "", fmt.Errorf("file exceeds 4 MB")
	}
	old, e := readSmall(f.Path)
	exists := e == nil
	if e != nil && !errors.Is(e, fs.ErrNotExist) {
		return "", e
	}
	if revision(f.Path, old) != expected {
		return "", errConflict
	}
	if !remove && f.Kind == "config" {
		if _, e := parseConfig(f.Path, content); e != nil {
			return "", fmt.Errorf("invalid configuration: %w", e)
		}
	}
	if !remove && f.Kind == "skill" && (!strings.HasPrefix(string(content), "---\n") || !strings.Contains(string(content), "description:")) {
		return "", fmt.Errorf("skills need YAML frontmatter with a description")
	}
	mode := fs.FileMode(0600)
	backup := ""
	if exists {
		info, e := os.Stat(f.Path)
		if e != nil {
			return "", e
		}
		mode = info.Mode().Perm()
		dir := filepath.Join(s.Home, ".local/state/agent-desk/backups")
		if e = s.safeWrite(dir); e != nil {
			return "", e
		}
		if e = os.MkdirAll(dir, 0700); e != nil {
			return "", e
		}
		backup = filepath.Join(dir, time.Now().UTC().Format("20060102T150405.000000000")+"-"+fileID(f.Path)+".json")
		data, _ := json.Marshal(map[string]string{"path": f.Path, "content": string(old), "savedAt": time.Now().UTC().Format(time.RFC3339)})
		if e = os.WriteFile(backup, data, 0600); e != nil {
			return "", e
		}
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
