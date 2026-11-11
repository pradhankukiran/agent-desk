package desk

import (
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
