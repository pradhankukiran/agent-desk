package desk

import (
	"encoding/json"
	"fmt"
	"github.com/pelletier/go-toml/v2"
	"github.com/tidwall/jsonc"
	"strings"
)

func parseConfig(path string, b []byte) (map[string]any, error) {
	d := map[string]any{}
	var e error
	if strings.HasSuffix(path, ".toml") {
		e = toml.Unmarshal(b, &d)
	} else {
		if strings.HasSuffix(path, ".jsonc") {
			b = jsonc.ToJSON(b)
		}
		e = json.Unmarshal(b, &d)
	}
	if e == nil && d == nil {
		e = fmt.Errorf("configuration must be an object")
	}
	return d, e
}
