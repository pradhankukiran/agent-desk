package desk

import (
	"encoding/json"
	"testing"
)

func TestSessionParsers(t *testing.T) {
	for _, tc := range []struct{ agent, raw string }{{"codex", `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}`}, {"claude", `{"type":"user","message":{"role":"user","content":[{"type":"text","text":"hello"}]}}`}, {"grok", `{"type":"user","content":[{"type":"text","text":"hello"}]}`}} {
		t.Run(tc.agent, func(t *testing.T) {
			var d map[string]any
			json.Unmarshal([]byte(tc.raw), &d)
			m, ok := normalize(tc.agent, d)
			if !ok || m.Text != "hello" || m.Role != "user" {
				t.Fatalf("bad message: %+v", m)
			}
		})
	}
}
