package desk

import (
	"strings"
)

func str(v any) string { s, _ := v.(string); return s }

func obj(v any) map[string]any { m, _ := v.(map[string]any); return m }

// splitContent returns the reply text and the model's thinking text from a
// message body, which is either a string or a list of typed blocks.
func splitContent(v any) (text, thinking string) {
	switch c := v.(type) {
	case string:
		return c, ""
	case []any:
		var parts, thoughts []string
		for _, v := range c {
			m := obj(v)
			switch str(m["type"]) {
			case "text", "input_text", "output_text":
				if t := str(m["text"]); t != "" {
					parts = append(parts, t)
				}
			case "reasoning", "thinking":
				t := str(m["thinking"])
				if t == "" {
					t = str(m["text"])
				}
				if t != "" {
					thoughts = append(thoughts, t)
				}
			}
		}
		return strings.Join(parts, "\n\n"), strings.Join(thoughts, "\n\n")
	}
	return "", ""
}

// messages builds the visible messages for one entry: thinking, then the reply.
func messages(role, text, thinking, time string) []Message {
	out := []Message{}
	if thinking != "" {
		out = append(out, Message{Role: "thinking", Text: thinking, Time: time})
	}
	if text != "" {
		out = append(out, Message{Role: role, Text: text, Time: time})
	}
	return out
}

// normalize extracts user and assistant messages from one JSONL entry.
func normalize(agent string, d map[string]any) []Message {
	var role string
	var body any
	switch agent {
	case "codex":
		p := obj(d["payload"])
		if str(d["type"]) != "response_item" || str(p["type"]) != "message" {
			return nil
		}
		role, body = str(p["role"]), p["content"]
	case "claude":
		p := obj(d["message"])
		role, body = str(p["role"]), p["content"]
	case "grok":
		role, body = str(d["type"]), d["content"]
	}
	if role != "user" && role != "assistant" {
		return nil
	}
	text, thinking := splitContent(body)
	if role == "user" {
		thinking = ""
	}
	return messages(role, text, thinking, str(d["timestamp"]))
}

func short(s string, n int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return string(r)
}
