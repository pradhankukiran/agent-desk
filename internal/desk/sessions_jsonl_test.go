package desk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func claudeLine(text string) string {
	return `{"type":"user","message":{"role":"user","content":[{"type":"text","text":"` + text + `"}]}}` + "\n"
}

func TestTranscriptReadsLargeLines(t *testing.T) {
	s := fixture(t)
	path := filepath.Join(s.Home, ".claude/projects/p/large.jsonl")
	big := strings.Repeat("x", 9<<20) // larger than the old 8 MB scanner limit
	put(t, path, claudeLine("before")+claudeLine(big)+"not json\n"+claudeLine("after"))
	tr, e := s.transcript(Session{Agent: "claude", Path: path, Format: "jsonl"})
	if e != nil {
		t.Fatal(e)
	}
	if len(tr.Messages) != 3 || tr.Messages[0].Text != "before" || tr.Messages[1].Text != big || tr.Messages[2].Text != "after" {
		t.Fatalf("unexpected messages: %d", len(tr.Messages))
	}
	if strings.Contains(tr.Note, "skipped") {
		t.Fatalf("nothing should be skipped: %q", tr.Note)
	}
}

func TestTranscriptSkipsOversizedLines(t *testing.T) {
	old := maxLine
	maxLine = 1 << 10
	defer func() { maxLine = old }()
	s := fixture(t)
	path := filepath.Join(s.Home, ".claude/projects/p/huge.jsonl")
	huge := strings.Repeat("y", 200<<10) // spans several reader buffers
	put(t, path, claudeLine("before")+claudeLine(huge)+claudeLine(huge)+claudeLine("after"))
	tr, e := s.transcript(Session{Agent: "claude", Path: path, Format: "jsonl"})
	if e != nil {
		t.Fatal(e)
	}
	if len(tr.Messages) != 2 || tr.Messages[0].Text != "before" || tr.Messages[1].Text != "after" {
		t.Fatalf("unexpected messages: %+v", tr.Messages)
	}
	if !strings.Contains(tr.Note, "2 very large entries were skipped.") {
		t.Fatalf("missing skip note: %q", tr.Note)
	}
}

func TestSessionProjectFilter(t *testing.T) {
	s := fixture(t)
	app := filepath.Join(s.Home, "projects/app")
	for _, d := range []string{app + "/src", s.Home + "/projects/app2"} {
		put(t, filepath.Join(d, ".keep"), "")
	}
	link := filepath.Join(s.Home, "linked")
	if e := os.Symlink(app, link); e != nil {
		t.Fatal(e)
	}
	agy := `["file://` + strings.ReplaceAll(s.Home, " ", "%20") + `/other","file://` + app + `/src"]`
	cases := []struct {
		recorded string
		want     bool
	}{
		{app, true},
		{app + "/src", true},
		{s.Home + "/projects/app2", false},
		{s.Home + "/projects", false},
		{link + "/src", true},
		{agy, true},
		{`["file://` + s.Home + `/projects/app2"]`, false},
		{"", false},
		{"relative/app", false},
	}
	for _, c := range cases {
		item := Session{Project: c.recorded}
		if got := inProject(item, app, map[string]string{}); got != c.want {
			t.Errorf("%q: got %v, want %v", c.recorded, got, c.want)
		}
	}
	if got := sessionDirs(agy); len(got) != 2 || got[1] != app+"/src" {
		t.Fatalf("agy folders: %v", got)
	}
}
