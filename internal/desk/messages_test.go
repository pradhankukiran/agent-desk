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
			m := normalize(tc.agent, d)
			if len(m) != 1 || m[0].Text != "hello" || m[0].Role != "user" {
				t.Fatalf("bad message: %+v", m)
			}
		})
	}
}

func TestThinkingIsSeparate(t *testing.T) {
	for _, tc := range []struct{ agent, raw, want string }{
		{"claude", `{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"plan it","signature":"x"},{"type":"text","text":"done"}]}}`, "thinking:plan it|assistant:done"},
		{"claude", `{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"only thinking"}]}}`, "thinking:only thinking"},
		{"claude", `{"type":"user","message":{"role":"user","content":"plain prompt"}}`, "user:plain prompt"},
		{"codex", `{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"reasoning","text":"plan it"},{"type":"output_text","text":"done"}]}}`, "thinking:plan it|assistant:done"},
	} {
		var d map[string]any
		if e := json.Unmarshal([]byte(tc.raw), &d); e != nil {
			t.Fatal(e)
		}
		if got := roles(normalize(tc.agent, d)); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.agent, got, tc.want)
		}
	}
}
