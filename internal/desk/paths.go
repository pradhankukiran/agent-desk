package desk

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func fileID(path string) string { return digest([]byte(path))[:24] }

func within(root, path string) bool {
	r, e := filepath.Rel(root, path)
	return e == nil && r != ".." && !strings.HasPrefix(r, ".."+string(os.PathSeparator))
}

func (s *Server) project(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if raw == "~" {
		raw = s.Home
	}
	if strings.HasPrefix(raw, "~/") {
		raw = filepath.Join(s.Home, raw[2:])
	}
	p, e := filepath.EvalSymlinks(raw)
	if e != nil {
		return "", fmt.Errorf("project directory does not exist")
	}
	st, e := os.Stat(p)
	if e != nil || !st.IsDir() || !within(s.Home, p) {
		return "", fmt.Errorf("choose a directory inside %s", s.Home)
	}
	return p, nil
}
