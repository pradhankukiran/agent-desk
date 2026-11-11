package desk

import (
	"strings"
	"testing"
)

func TestCheckSkill(t *testing.T) {
	cases := map[string]string{
		"---\nname: a\ndescription: Does a thing\n---\nBody":            "",
		"---\r\nname: a\r\ndescription: Windows endings\r\n---\r\nBody": "",
		"\ufeff---\ndescription: \"quoted\"\n---\n":                     "",
		"---\ndescription: >-\n  folded text\n  more\n---\n":            "",
		"---\ndescription:\n  indented text\n---\n":                     "",
		"# Title\n\ndescription: not in a header\n":                     "start with",
		"---\nname: a\ndescription: never closed\n":                     "closing",
		"---\nname: a\n---\ndescription: after the header\n":            "needs a description",
		"---\nmetadata:\n  description: nested only\n---\n":             "needs a description",
		"---\ndescription:\n---\n":                                      "empty",
		"---\ndescription: \"\"\n---\n":                                 "empty",
		"---\ndescription: |\n\nname: next key\n---\n":                  "empty",
	}
	for content, want := range cases {
		e := checkSkill([]byte(content))
		if want == "" && e != nil || want != "" && (e == nil || !strings.Contains(e.Error(), want)) {
			t.Errorf("%q: got %v, want %q", content, e, want)
		}
	}
}
