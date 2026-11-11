package desk

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCodeSchemasAndReadOnly(t *testing.T) {
	s := fixture(t)
	path := filepath.Join(s.Home, ".local/share/opencode/opencode.db")
	dbFixture(t, path,
		`CREATE TABLE session (id TEXT,title TEXT,directory TEXT,time_updated INTEGER)`,
		`CREATE TABLE session_v2 (id TEXT,title TEXT,directory TEXT,time_updated INTEGER)`,
		`CREATE TABLE message (id TEXT,session_id TEXT,data TEXT,time_created INTEGER)`,
		`CREATE TABLE part (id TEXT,message_id TEXT,data TEXT,time_created INTEGER)`,
		`CREATE TABLE session_message (session_id TEXT,type TEXT,data TEXT,seq INTEGER)`,
		`INSERT INTO session VALUES ('old','Old session','/project',1000)`,
		`INSERT INTO session_v2 VALUES ('new','New session','/project',2000)`,
		`INSERT INTO message VALUES ('msg','old','{"role":"assistant"}',1000)`,
		`INSERT INTO part VALUES ('part','msg','{"type":"text","text":"old reply"}',1001)`,
		`INSERT INTO session_message VALUES ('new','user','{"text":"new prompt"}',1)`)
	items, warnings := s.dbSessions("opencode")
	if len(items) != 2 || len(warnings) != 0 {
		t.Fatalf("sessions %v warnings %v", items, warnings)
	}
	for _, item := range items {
		tr, e := s.transcript(item)
		if e != nil || len(tr.Messages) != 1 {
			t.Fatalf("transcript %+v %v", tr, e)
		}
	}
	db, e := openDB(path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = db.Exec("DELETE FROM session"); e == nil {
		t.Fatal("database was writable")
	}
}

func TestAgyExcerpts(t *testing.T) {
	s := fixture(t)
	path := filepath.Join(s.Home, ".gemini/antigravity-cli/conversation_summaries.db")
	dbFixture(t, path,
		`CREATE TABLE conversation_summaries (conversation_id TEXT,title TEXT,preview TEXT,workspace_uris TEXT,last_modified_time TEXT)`,
		`CREATE TABLE conversations_fts (conversation_id TEXT,chunk_index INTEGER,user_body TEXT,agent_body TEXT)`,
		`INSERT INTO conversation_summaries VALUES ('abc','Agy session','','/project','2026-01-01T00:00:00Z')`,
		`INSERT INTO conversations_fts VALUES ('abc',0,'question','answer')`)
	items, w := s.dbSessions("agy")
	if len(items) != 1 || len(w) != 0 {
		t.Fatalf("%v %v", items, w)
	}
	tr, e := s.transcript(items[0])
	if e != nil || len(tr.Messages) != 2 || !strings.Contains(tr.Note, "not a complete") {
		t.Fatalf("%+v %v", tr, e)
	}
}

func roles(m []Message) string {
	out := []string{}
	for _, x := range m {
		out = append(out, x.Role+":"+x.Text)
	}
	return strings.Join(out, "|")
}

func TestOpenCodeThinkingIsSeparate(t *testing.T) {
	s := fixture(t)
	path := filepath.Join(s.Home, ".local/share/opencode/opencode.db")
	dbFixture(t, path,
		`CREATE TABLE session (id TEXT,title TEXT,directory TEXT,time_updated INTEGER)`,
		`CREATE TABLE session_v2 (id TEXT,title TEXT,directory TEXT,time_updated INTEGER)`,
		`CREATE TABLE message (id TEXT,session_id TEXT,data TEXT,time_created INTEGER)`,
		`CREATE TABLE part (id TEXT,message_id TEXT,data TEXT,time_created INTEGER)`,
		`CREATE TABLE session_message (session_id TEXT,type TEXT,data TEXT,seq INTEGER)`,
		`INSERT INTO session VALUES ('old','Old session','/project',1000)`,
		`INSERT INTO session_v2 VALUES ('new','New session','/project',2000)`,
		`INSERT INTO message VALUES ('msg','old','{"role":"assistant"}',1000)`,
		`INSERT INTO part VALUES ('p1','msg','{"type":"reasoning","text":"plan it"}',1001)`,
		`INSERT INTO part VALUES ('p2','msg','{"type":"text","text":"done"}',1002)`,
		`INSERT INTO part VALUES ('p3','msg','{"type":"text","text":"more"}',1003)`,
		`INSERT INTO session_message VALUES ('new','assistant','{"content":[{"type":"reasoning","text":"think v2"},{"type":"tool"},{"type":"text","text":"answer v2"}]}',1)`)
	items, _ := s.dbSessions("opencode")
	want := map[string]string{
		"session:old":    "thinking:plan it|assistant:done\n\nmore",
		"session_v2:new": "thinking:think v2|assistant:answer v2",
	}
	for _, item := range items {
		tr, e := s.transcript(item)
		if e != nil {
			t.Fatal(e)
		}
		if got := roles(tr.Messages); got != want[item.ID] {
			t.Errorf("%s: got %q, want %q", item.ID, got, want[item.ID])
		}
	}
}
