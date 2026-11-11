package desk

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func openDB(path string) (*sql.DB, error) {
	if _, e := os.Stat(path); e != nil {
		return nil, e
	}
	u := url.URL{Scheme: "file", Path: path}
	q := u.Query()
	q.Set("mode", "ro")
	q.Set("_pragma", "busy_timeout(2000)")
	u.RawQuery = q.Encode()
	db, e := sql.Open("sqlite", u.String())
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	if e = db.Ping(); e != nil {
		db.Close()
		return nil, e
	}
	return db, nil
}

func (s *Server) dbSessions(agent string) ([]Session, []string) {
	out := []Session{}
	warnings := []string{}
	path := filepath.Join(s.Home, ".local/share/opencode/opencode.db")
	if agent == "agy" {
		path = filepath.Join(s.Home, ".gemini/antigravity-cli/conversation_summaries.db")
	}
	db, e := openDB(path)
	if e != nil {
		if !os.IsNotExist(e) {
			warnings = append(warnings, e.Error())
		}
		return out, warnings
	}
	defer db.Close()
	if agent == "opencode" {
		for _, table := range []string{"session", "session_v2"} {
			rows, e := db.Query("SELECT id, COALESCE(title,''), directory, time_updated FROM " + table + " ORDER BY time_updated DESC")
			if e != nil {
				warnings = append(warnings, table+": "+e.Error())
				continue
			}
			for rows.Next() {
				var id, title, project string
				var updated int64
				if e = rows.Scan(&id, &title, &project, &updated); e != nil {
					warnings = append(warnings, e.Error())
					continue
				}
				if title == "" {
					title = id
				}
				out = append(out, Session{ID: table + ":" + id, Agent: agent, Title: title, Project: project, Updated: time.UnixMilli(updated).UTC().Format(time.RFC3339Nano), Path: path, Format: table})
			}
			if e = rows.Err(); e != nil {
				warnings = append(warnings, e.Error())
			}
			rows.Close()
		}
	} else {
		rows, e := db.Query("SELECT conversation_id, COALESCE(title,''), COALESCE(preview,''), COALESCE(workspace_uris,''), last_modified_time FROM conversation_summaries ORDER BY last_modified_time DESC")
		if e != nil {
			return out, []string{e.Error()}
		}
		defer rows.Close()
		for rows.Next() {
			var id, title, preview, project, updated string
			if e = rows.Scan(&id, &title, &preview, &project, &updated); e != nil {
				warnings = append(warnings, e.Error())
				continue
			}
			if title == "" {
				title = short(preview, 120)
			}
			if title == "" {
				title = id
			}
			dirs := sessionDirs(project)
			out = append(out, Session{ID: id, Agent: agent, Title: title, Project: strings.Join(dirs, ", "), Updated: updated, Path: path, Format: "indexed excerpts", dirs: dirs})
		}
		if e = rows.Err(); e != nil {
			warnings = append(warnings, e.Error())
		}
	}
	return out, warnings
}

func (s *Server) transcript(item Session) (Transcript, error) {
	t := Transcript{Messages: []Message{}}
	if item.Format == "jsonl" {
		if info, e := os.Stat(item.Path); e == nil {
			t.Truncated = info.Size() > 64<<20
		}
		skipped, e := scanLines(item.Path, 64<<20, func(d map[string]any) bool {
			t.Messages = append(t.Messages, normalize(item.Agent, d)...)
			return true
		})
		t.Note = "Conversation messages. Tool events and system instructions are omitted."
		if t.Truncated {
			t.Note += " Showing the first 64 MB."
		}
		if skipped == 1 {
			t.Note += " 1 very large entry was skipped."
		} else if skipped > 1 {
			t.Note += fmt.Sprintf(" %d very large entries were skipped.", skipped)
		}
		if e != nil {
			if len(t.Messages) == 0 {
				return t, e
			}
			t.Note += " Reading stopped early: " + e.Error() + "."
		}
		return t, nil
	}
	db, e := openDB(item.Path)
	if e != nil {
		return t, e
	}
	defer db.Close()
	if item.Agent == "agy" {
		t.Note = "Indexed excerpts from Agy’s local search index. User and assistant text are grouped by chunk; this is not a complete, ordered transcript."
		rows, e := db.Query("SELECT COALESCE(user_body,''), COALESCE(agent_body,'') FROM conversations_fts WHERE conversation_id=? ORDER BY chunk_index", item.ID)
		if e != nil {
			return t, e
		}
		defer rows.Close()
		for rows.Next() {
			var user, assistant string
			if e = rows.Scan(&user, &assistant); e != nil {
				return t, e
			}
			if user != "" {
				t.Messages = append(t.Messages, Message{Role: "user", Text: user})
			}
			if assistant != "" {
				t.Messages = append(t.Messages, Message{Role: "assistant", Text: assistant})
			}
		}
		return t, rows.Err()
	}
	pair := strings.SplitN(item.ID, ":", 2)
	if len(pair) != 2 {
		return t, fmt.Errorf("invalid session ID")
	}
	id := pair[1]
	if item.Format == "session_v2" {
		rows, e := db.Query("SELECT type,data FROM session_message WHERE session_id=? ORDER BY seq", id)
		if e != nil {
			return t, e
		}
		defer rows.Close()
		for rows.Next() {
			var role, raw string
			if e = rows.Scan(&role, &raw); e != nil {
				return t, e
			}
			var d map[string]any
			if json.Unmarshal([]byte(raw), &d) != nil {
				continue
			}
			text, thinking := str(d["text"]), ""
			if text == "" {
				text, thinking = splitContent(d["content"])
			}
			t.Messages = append(t.Messages, messages(role, text, thinking, "")...)
		}
		return t, rows.Err()
	}
	rows, e := db.Query("SELECT m.id,m.data,p.data FROM message m LEFT JOIN part p ON p.message_id=m.id WHERE m.session_id=? ORDER BY m.time_created,m.id,p.time_created,p.id", id)
	if e != nil {
		return t, e
	}
	defer rows.Close()
	last := ""
	for rows.Next() {
		var id, raw string
		var part sql.NullString
		if e = rows.Scan(&id, &raw, &part); e != nil {
			return t, e
		}
		var d, p map[string]any
		_ = json.Unmarshal([]byte(raw), &d)
		_ = json.Unmarshal([]byte(part.String), &p)
		text := str(p["text"])
		if text == "" {
			continue
		}
		role := str(d["role"])
		if str(p["type"]) == "reasoning" {
			role = "thinking"
		}
		// Join consecutive parts of the same message and role into one message.
		if key := id + "\x00" + role; last == key && len(t.Messages) > 0 {
			t.Messages[len(t.Messages)-1].Text += "\n\n" + text
		} else {
			t.Messages = append(t.Messages, Message{Role: role, Text: text})
			last = key
		}
	}
	t.Note = "Conversation text. Tool events and attachments are omitted."
	return t, rows.Err()
}
