package desk

import (
	"encoding/json"
	"fmt"
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

func codexSession(id, cwd, prompt string) string {
	return `{"type":"session_meta","payload":{"id":"` + id + `","cwd":"` + cwd + `"}}` + "\n" +
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"` + prompt + `"}]}}` + "\n"
}

func titlesByPath(items []Session) map[string]string {
	out := map[string]string{}
	for _, it := range items {
		out[filepath.Base(it.Path)] = it.Title + " @ " + it.Project
	}
	return out
}

func TestSessionListCacheTracksChanges(t *testing.T) {
	s := fixture(t)
	dir := filepath.Join(s.Home, ".codex/sessions/2026/01/01")
	put(t, filepath.Join(dir, "a.jsonl"), codexSession("a", "/w/a", "first prompt"))
	put(t, filepath.Join(dir, "b.jsonl"), codexSession("b", "/w/b", "second prompt"))
	items, _ := s.fileSessions("codex")
	if got := titlesByPath(items); got["a.jsonl"] != "first prompt @ /w/a" || got["b.jsonl"] != "second prompt @ /w/b" {
		t.Fatalf("first list: %v", got)
	}

	// A changed file is re-read, a new file appears, a removed file disappears.
	put(t, filepath.Join(dir, "a.jsonl"), codexSession("a", "/w/a2", "edited prompt that is longer"))
	put(t, filepath.Join(dir, "c.jsonl"), codexSession("c", "/w/c", "third prompt"))
	os.Remove(filepath.Join(dir, "b.jsonl"))
	// Index titles apply even to cached files.
	put(t, filepath.Join(s.Home, ".codex/session_index.jsonl"), `{"id":"c","thread_name":"Named thread"}`+"\n")
	items, _ = s.fileSessions("codex")
	got := titlesByPath(items)
	if len(got) != 2 || got["a.jsonl"] != "edited prompt that is longer @ /w/a2" || got["c.jsonl"] != "Named thread @ /w/c" {
		t.Fatalf("second list: %v", got)
	}
	s.cacheMu.Lock()
	n := len(s.sessionCache)
	s.cacheMu.Unlock()
	if n != 2 {
		t.Fatalf("cache holds %d entries, want 2", n)
	}
}

func TestSessionListStopsEarly(t *testing.T) {
	s := fixture(t)
	path := filepath.Join(s.Home, ".codex/sessions/2026/01/01/long.jsonl")
	// The later line would change the folder if it were read.
	put(t, path, codexSession("x", "/w/x", "hello")+`{"cwd":"/late"}`+"\n")
	info, _ := os.Stat(path)
	sf := readSessionFile("codex", path, info)
	if sf.item.Title != "hello" || sf.item.Project != "/w/x" || sf.metaID != "x" {
		t.Fatalf("codex did not stop after finding its details: %+v", sf.item)
	}
	// Claude keeps reading, so a later folder change is still picked up.
	path = filepath.Join(s.Home, ".claude/projects/p/s.jsonl")
	put(t, path, `{"cwd":"/w/start"}`+"\n"+claudeLine("hello")+`{"cwd":"/w/moved"}`+"\n")
	info, _ = os.Stat(path)
	if sf = readSessionFile("claude", path, info); sf.item.Project != "/w/moved" {
		t.Fatalf("claude folder: %+v", sf.item)
	}
}

func TestTranscriptPaging(t *testing.T) {
	s := fixture(t)
	path := filepath.Join(s.Home, ".claude/projects/p/long.jsonl")
	content := ""
	for i := 0; i < 450; i++ {
		word := "plain"
		if i%100 == 0 {
			word = "needle"
		}
		content += claudeLine(fmt.Sprintf("message %d %s", i, word))
	}
	put(t, path, content)
	id := fileID(path)
	get := func(params string) Transcript {
		r := callAPI(t, s, "GET", "session?agent=claude&id="+id+params, "")
		if r.code != 200 {
			t.Fatalf("%s: %d %v", params, r.code, r.body)
		}
		b, _ := json.Marshal(r.body)
		var tr Transcript
		json.Unmarshal(b, &tr)
		return tr
	}
	first := get("")
	if first.Total != 450 || first.Offset != 0 || len(first.Messages) != 200 || first.Messages[0].Text != "message 0 needle" {
		t.Fatalf("first page: total %d offset %d len %d", first.Total, first.Offset, len(first.Messages))
	}
	last := get("&offset=400")
	if len(last.Messages) != 50 || last.Messages[49].Text != "message 449 plain" {
		t.Fatalf("last page: %d messages", len(last.Messages))
	}
	if past := get("&offset=9999&limit=-3"); len(past.Messages) != 0 || past.Offset != 450 {
		t.Fatalf("past the end: %+v", past)
	}
	found := get("&q=NEEDLE&limit=2")
	if found.Total != 5 || len(found.Messages) != 2 || found.Messages[1].Text != "message 100 needle" {
		t.Fatalf("filtered: total %d %+v", found.Total, found.Messages)
	}
}

func TestTitleText(t *testing.T) {
	cases := map[string]string{
		"plain question": "plain question",
		"\n\n<pasted_content id=\"1\">\nJob post\n</pasted_content>\n\nthis is a job, tell me": "this is a job, tell me",
		"<system-reminder>rules</system-reminder>":                                             "",
		"  <command-name>/init</command-name>":                                                 "",
		"# AGENTS.md instructions for /x":                                                      "",
		"<unclosed tag and text":                                                               "",
		"compare a < b and c > d":                                                              "compare a < b and c > d",
	}
	for in, want := range cases {
		if got := titleText(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}
