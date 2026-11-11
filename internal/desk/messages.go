package desk

import (
	"strings"
)

func str(v any) string { s, _ := v.(string); return s }

func obj(v any) map[string]any { m, _ := v.(map[string]any); return m }

func textContent(v any) string {
	switch c := v.(type) {
	case string:
		return c
	case []any:
		parts := []string{}
		for _, v := range c {
			m := obj(v)
			kind := str(m["type"])
			if kind == "text" || kind == "input_text" || kind == "output_text" || kind == "reasoning" || kind == "thinking" {
				t := str(m["text"])
				if t == "" {
					t = str(m["thinking"])
				}
				if t != "" {
					parts = append(parts, t)
				}
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

func normalize(agent string, d map[string]any) (Message, bool) {
	m := Message{Time: str(d["timestamp"])}
	switch agent {
	case "codex":
		if str(d["type"]) != "response_item" {
			return m, false
		}
		p := obj(d["payload"])
		if str(p["type"]) != "message" {
			return m, false
		}
		m.Role = str(p["role"])
		m.Text = textContent(p["content"])
	case "claude":
		p := obj(d["message"])
		m.Role = str(p["role"])
		m.Text = textContent(p["content"])
	case "grok":
		m.Role = str(d["type"])
		m.Text = textContent(d["content"])
	}
	return m, (m.Role == "user" || m.Role == "assistant") && m.Text != ""
}

func short(s string, n int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return string(r)
}
