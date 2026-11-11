package desk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const mergeBase = `{
  "numStartups": 1,
  "theme": "dark",
  "mcpServers": {
    "docs": {
      "url": "https://example.test/mcp"
    }
  },
  "projects": {
    "/home/me/app.v2": {
      "allowedTools": [
        "Read"
      ]
    },
    "8080": true
  },
  "a:b": 1
}
`

func TestMergeJSON(t *testing.T) {
	cases := []struct {
		name, mine, disk, want, clash string
	}{
		{
			name: "different settings merge and keep order",
			mine: strings.Replace(mergeBase, `"theme": "dark"`, `"theme": "light"`, 1),
			disk: strings.Replace(mergeBase, `"numStartups": 1`, `"numStartups": 2`, 1),
			want: strings.Replace(strings.Replace(mergeBase, `"theme": "dark"`, `"theme": "light"`, 1), `"numStartups": 1`, `"numStartups": 2`, 1),
		},
		{
			name: "key with dots and nested add",
			mine: strings.Replace(mergeBase, `"Read"`, `"Read",
        "Edit"`, 1),
			disk: strings.Replace(mergeBase, `"numStartups": 1`, `"numStartups": 9`, 1),
			want: strings.Replace(strings.Replace(mergeBase, `"Read"`, `"Read",
        "Edit"`, 1), `"numStartups": 1`, `"numStartups": 9`, 1),
		},
		{
			name: "deleted setting and odd keys",
			mine: strings.Replace(strings.Replace(mergeBase, `,
    "8080": true`, ``, 1), `"a:b": 1`, `"a:b": 2`, 1),
			disk: strings.Replace(mergeBase, `"numStartups": 1`, `"numStartups": 3`, 1),
			want: strings.Replace(strings.Replace(strings.Replace(mergeBase, `,
    "8080": true`, ``, 1), `"a:b": 1`, `"a:b": 2`, 1), `"numStartups": 1`, `"numStartups": 3`, 1),
		},
		{
			name: "new server added while the other side edits another",
			mine: strings.Replace(mergeBase, `"mcpServers": {`, `"mcpServers": {
    "local": {
      "command": "run"
    },`, 1),
			disk: strings.Replace(mergeBase, `"theme": "dark"`, `"theme": "auto"`, 1),
			want: strings.Replace(strings.Replace(mergeBase, `"theme": "dark"`, `"theme": "auto"`, 1), `"url": "https://example.test/mcp"
    }`, `"url": "https://example.test/mcp"
    },
    "local": {
      "command": "run"
    }`, 1),
		},
		{
			name:  "same setting changed differently",
			mine:  strings.Replace(mergeBase, `"theme": "dark"`, `"theme": "light"`, 1),
			disk:  strings.Replace(mergeBase, `"theme": "dark"`, `"theme": "auto"`, 1),
			clash: "theme",
		},
		{
			name: "edit inside a setting the other side deleted",
			mine: strings.Replace(mergeBase, `"Read"`, `"Write"`, 1),
			disk: strings.Replace(mergeBase, `"/home/me/app.v2": {
      "allowedTools": [
        "Read"
      ]
    },
`, ``, 1),
			clash: "allowedTools",
		},
		{
			name: "same change on both sides",
			mine: strings.Replace(mergeBase, `"theme": "dark"`, `"theme": "light"`, 1),
			disk: strings.Replace(mergeBase, `"theme": "dark"`, `"theme": "light"`, 1),
			want: strings.Replace(mergeBase, `"theme": "dark"`, `"theme": "light"`, 1),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, e := mergeJSON([]byte(mergeBase), []byte(c.mine), []byte(c.disk))
			if c.clash != "" {
				if !errors.Is(e, errConflict) || !strings.Contains(e.Error(), c.clash) {
					t.Fatalf("want clash on %s, got %v", c.clash, e)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if string(got) != c.want {
				t.Fatalf("got:\n%s\nwant:\n%s", got, c.want)
			}
		})
	}
}

func TestSaveMergesOutsideJSONChanges(t *testing.T) {
	s := fixture(t)
	path := filepath.Join(s.Home, ".claude.json")
	put(t, path, mergeBase)
	opened := callAPI(t, s, "GET", "file?id="+fileID(path), "")
	put(t, path, strings.Replace(mergeBase, `"numStartups": 1`, `"numStartups": 2`, 1))
	mine := strings.Replace(mergeBase, `"theme": "dark"`, `"theme": "light"`, 1)
	body := fmt.Sprintf(`{"content":%q,"revision":%q,"base":%q}`, mine, opened.body["revision"], mergeBase)
	saved := callAPI(t, s, "PUT", "file?id="+fileID(path), body)
	if saved.code != 200 || saved.body["merged"] != true {
		t.Fatalf("save: %d %v", saved.code, saved.body)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), `"numStartups": 2`) || !strings.Contains(string(b), `"theme": "light"`) {
		t.Fatalf("merged file:\n%s", b)
	}
	// Without the opened content the strict check still applies.
	put(t, path, mergeBase)
	strict := fmt.Sprintf(`{"content":%q,"revision":%q}`, mine, digest([]byte("old")))
	if r := callAPI(t, s, "PUT", "file?id="+fileID(path), strict); r.code != 409 {
		t.Fatalf("strict save: %d %v", r.code, r.body)
	}
}
