package desk

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// undoRecord remembers what a file held before its last save in this run.
type undoRecord struct {
	previous []byte
	existed  bool   // false when the save created the file
	after    string // revision written by the save
	newDir   string // skill folder created with the file, removed if left empty
}

// current reads a file and its revision, treating a missing file as empty.
func current(path string) ([]byte, string, error) {
	b, e := readSmall(path)
	if errors.Is(e, fs.ErrNotExist) {
		return nil, "missing", nil
	}
	if e != nil {
		return nil, "", e
	}
	return b, digest(b), nil
}

// remember records the state before a save. Call with s.mu held, before saving.
func (s *Server) remember(path, expected string) (undoRecord, bool) {
	b, rev, e := current(path)
	if e != nil || rev != expected {
		return undoRecord{}, false
	}
	return undoRecord{previous: b, existed: rev != "missing"}, true
}

// undoFor returns the record for a file if undoing it is still safe: the file
// must still hold exactly what the last save wrote. Call with s.mu held.
func (s *Server) undoFor(path string) (undoRecord, bool) {
	rec, ok := s.undo[path]
	if !ok {
		return rec, false
	}
	if _, rev, e := current(path); e != nil || rev != rec.after {
		return rec, false
	}
	return rec, true
}

// restore undoes the last save of f. It returns whether the file was removed.
// Call with s.mu held.
func (s *Server) restore(f File, revision string) (bool, error) {
	rec, ok := s.undoFor(f.Path)
	if !ok {
		return false, fmt.Errorf("nothing to undo, or the file changed after the last save")
	}
	if revision != rec.after {
		return false, errConflict
	}
	if rec.existed {
		_, e := s.write(f, rec.previous, rec.after, false, false)
		if e == nil {
			delete(s.undo, f.Path)
		}
		return false, e
	}
	if _, e := s.write(f, nil, rec.after, true, false); e != nil {
		return false, e
	}
	delete(s.undo, f.Path)
	if rec.newDir != "" {
		os.Remove(rec.newDir) // fails, as intended, if anything else is in it
	}
	return true, nil
}

// skillDirIsNew reports whether the folder for a new skill does not exist yet.
func skillDirIsNew(path string) string {
	dir := filepath.Dir(path)
	if _, e := os.Lstat(dir); errors.Is(e, fs.ErrNotExist) {
		return dir
	}
	return ""
}
