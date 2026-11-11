package desk

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestProjectsFromHistoryAndSettings(t *testing.T) {
	s := fixture(t)
	h := s.Home
	for _, d := range []string{"projects/listed", "work/older", "work/recent", "work/claude-only"} {
		os.MkdirAll(filepath.Join(h, d), 0700)
	}
	outside := t.TempDir()
	day := filepath.Join(h, ".codex/sessions/2026/01/01")
	put(t, filepath.Join(day, "old.jsonl"), codexSession("o", filepath.Join(h, "work/older"), "old"))
	put(t, filepath.Join(day, "new.jsonl"), codexSession("n", filepath.Join(h, "work/recent/sub/.."), "new"))
	put(t, filepath.Join(day, "gone.jsonl"), codexSession("g", filepath.Join(h, "work/deleted"), "gone"))
	put(t, filepath.Join(day, "out.jsonl"), codexSession("x", outside, "outside home"))
	os.Chtimes(filepath.Join(day, "old.jsonl"), time.Now(), time.Now().Add(-48*time.Hour))
	put(t, filepath.Join(h, ".claude.json"), `{"projects":{"`+filepath.Join(h, "work/claude-only")+`":{},"`+h+`":{}}}`)
	got := s.projects()
	want := []string{h, filepath.Join(h, "work/recent"), filepath.Join(h, "work/older"), filepath.Join(h, "projects/listed"), filepath.Join(h, "work/claude-only")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}
