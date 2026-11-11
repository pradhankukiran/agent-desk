package desk

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

const skillDoc = "---\nname: demo\ndescription: Demo skill\n---\n\nDo it.\n"

type apiResult struct {
	code int
	body map[string]any
}

func callAPI(t *testing.T, s *Server, method, path, body string) apiResult {
	t.Helper()
	handler := s.Handler(fstest.MapFS{})
	r := httptest.NewRequest(method, "http://localhost:7437/api/"+path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+s.Token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	out := apiResult{code: w.Code}
	json.Unmarshal(w.Body.Bytes(), &out.body)
	return out
}

// openAndRemove reads a file through the API and then asks to delete it.
func openAndRemove(t *testing.T, s *Server, path, revision string) (apiResult, apiResult) {
	t.Helper()
	opened := callAPI(t, s, "GET", "file?id="+fileID(path), "")
	if opened.code != 200 {
		t.Fatalf("open %s: %d %v", path, opened.code, opened.body)
	}
	if revision == "" {
		revision = opened.body["revision"].(string)
	}
	removed := callAPI(t, s, "DELETE", "file?id="+fileID(path), fmt.Sprintf(`{"revision":%q}`, revision))
	return opened, removed
}

func TestRemoveSkillFolderWithBackup(t *testing.T) {
	s := fixture(t)
	dir := filepath.Join(s.Home, ".claude/skills/demo")
	put(t, filepath.Join(dir, "SKILL.md"), skillDoc)
	put(t, filepath.Join(dir, "scripts/run.sh"), "echo hi\n")
	os.Chmod(filepath.Join(dir, "scripts/run.sh"), 0755)
	opened, removed := openAndRemove(t, s, filepath.Join(dir, "SKILL.md"), "")
	if opened.body["skillFiles"] != float64(2) {
		t.Fatalf("file count: %v", opened.body)
	}
	if removed.code != 200 {
		t.Fatalf("remove: %d %v", removed.code, removed.body)
	}
	if _, e := os.Stat(dir); !os.IsNotExist(e) {
		t.Fatal("skill folder still exists")
	}
	backup := removed.body["backup"].(string)
	if b, _ := os.ReadFile(filepath.Join(backup, "SKILL.md")); string(b) != skillDoc {
		t.Fatalf("backup SKILL.md: %q", b)
	}
	info, e := os.Stat(filepath.Join(backup, "scripts/run.sh"))
	if e != nil || info.Mode().Perm() != 0755 {
		t.Fatalf("backup script: %v %v", info, e)
	}
}

func TestRemoveSkillBlocked(t *testing.T) {
	cases := map[string]struct {
		setup    func(t *testing.T, home, dir string)
		revision string
		code     int
		problem  string
	}{
		"stale revision": {func(*testing.T, string, string) {}, "stale", 409, ""},
		"symbolic link": {func(t *testing.T, home, dir string) {
			os.Symlink(filepath.Join(home, "elsewhere"), filepath.Join(dir, "link"))
		}, "", 400, "symbolic link"},
		"nested skill": {func(t *testing.T, home, dir string) {
			put(t, filepath.Join(dir, "inner/SKILL.md"), skillDoc)
		}, "", 400, "another skill"},
		"too many files": {func(t *testing.T, home, dir string) {
			for i := 0; i < maxSkillFiles; i++ {
				put(t, filepath.Join(dir, "data", fmt.Sprint(i)), "x")
			}
		}, "", 400, "larger than"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			s := fixture(t)
			dir := filepath.Join(s.Home, ".claude/skills/demo")
			put(t, filepath.Join(dir, "SKILL.md"), skillDoc)
			c.setup(t, s.Home, dir)
			opened, removed := openAndRemove(t, s, filepath.Join(dir, "SKILL.md"), c.revision)
			if c.problem != "" && !strings.Contains(fmt.Sprint(opened.body["skillProblem"]), c.problem) {
				t.Fatalf("open did not report problem: %v", opened.body["skillProblem"])
			}
			if removed.code != c.code {
				t.Fatalf("remove: %d %v", removed.code, removed.body)
			}
			if _, e := os.Stat(filepath.Join(dir, "SKILL.md")); e != nil {
				t.Fatal("skill was removed")
			}
		})
	}
}

func TestRemoveSkillAtRootIsBlocked(t *testing.T) {
	s := fixture(t)
	path := filepath.Join(s.Home, ".claude/skills/SKILL.md")
	put(t, path, skillDoc)
	_, removed := openAndRemove(t, s, path, "")
	if removed.code != 400 || !strings.Contains(fmt.Sprint(removed.body["error"]), "known skills folder") {
		t.Fatalf("remove: %d %v", removed.code, removed.body)
	}
}

func TestRemoveInstructionRemovesOnlyFile(t *testing.T) {
	s := fixture(t)
	path := filepath.Join(s.Home, ".claude/CLAUDE.md")
	put(t, path, "# Notes\n")
	put(t, filepath.Join(s.Home, ".claude/keep.txt"), "keep")
	_, removed := openAndRemove(t, s, path, "")
	if removed.code != http.StatusOK {
		t.Fatalf("remove: %d %v", removed.code, removed.body)
	}
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("instruction still exists")
	}
	if _, e := os.Stat(filepath.Join(s.Home, ".claude/keep.txt")); e != nil {
		t.Fatal("neighbour file removed")
	}
}
