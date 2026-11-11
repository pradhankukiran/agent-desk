package desk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/tidwall/pretty"
	"github.com/tidwall/sjson"
)

// change is one difference between two JSON documents: a value set or removed
// at a path of object keys. Arrays are compared as whole values.
type change struct {
	path    []string
	value   any
	deleted bool
}

func parseJSON(b []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v map[string]any
	if e := dec.Decode(&v); e != nil {
		return nil, e
	}
	if v == nil {
		return nil, fmt.Errorf("configuration must be an object")
	}
	return v, nil
}

func diffJSON(path []string, a, b any, out *[]change) {
	am, aok := a.(map[string]any)
	bm, bok := b.(map[string]any)
	if !aok || !bok {
		if !reflect.DeepEqual(a, b) {
			*out = append(*out, change{path: path, value: b})
		}
		return
	}
	keys := make([]string, 0, len(am)+len(bm))
	for k := range am {
		keys = append(keys, k)
	}
	for k := range bm {
		if _, ok := am[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		p := append(append([]string{}, path...), k)
		av, inA := am[k]
		bv, inB := bm[k]
		switch {
		case !inB:
			*out = append(*out, change{path: p, deleted: true})
		case !inA:
			*out = append(*out, change{path: p, value: bv})
		default:
			diffJSON(p, av, bv, out)
		}
	}
}

// overlaps reports whether one path is the same as, or inside, the other.
func overlaps(a, b []string) bool {
	if len(a) > len(b) {
		a, b = b, a
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sjsonPath escapes object keys so dots and other path syntax are literal.
func sjsonPath(keys []string) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		var b strings.Builder
		if k != "" && strings.Trim(k, "0123456789") == "" {
			b.WriteByte(':') // a numeric key, not an array index
		}
		for _, r := range k {
			if strings.ContainsRune(`.*?|#@\!=<>%:`, r) {
				b.WriteByte('\\')
			}
			b.WriteRune(r)
		}
		parts[i] = b.String()
	}
	return strings.Join(parts, ".")
}

// mergeJSON applies the edits between base and mine onto disk, which another
// program changed after base was read. It edits disk's text in place so key
// order is kept. It fails with errConflict if both sides changed the same
// setting differently.
func mergeJSON(base, mine, disk []byte) ([]byte, error) {
	var docs [3]map[string]any
	for i, b := range [][]byte{base, mine, disk} {
		d, e := parseJSON(b)
		if e != nil {
			return nil, e
		}
		docs[i] = d
	}
	var ours, theirs []change
	diffJSON(nil, docs[0], docs[1], &ours)
	diffJSON(nil, docs[0], docs[2], &theirs)
	clashes := []string{}
	pending := []change{}
	for _, c := range ours {
		clash, same := false, false
		for _, t := range theirs {
			if !overlaps(c.path, t.path) {
				continue
			}
			if reflect.DeepEqual(c, t) {
				same = true
			} else {
				clash = true
			}
		}
		if clash {
			clashes = append(clashes, strings.Join(c.path, " › "))
		} else if !same {
			pending = append(pending, c)
		}
	}
	if len(clashes) > 0 {
		return nil, fmt.Errorf("%w; another program also changed: %s", errConflict, strings.Join(clashes, ", "))
	}
	out := disk
	for _, c := range pending {
		var e error
		if c.deleted {
			out, e = sjson.DeleteBytes(out, sjsonPath(c.path))
		} else {
			var raw []byte
			if raw, e = json.Marshal(c.value); e == nil {
				out, e = sjson.SetRawBytes(out, sjsonPath(c.path), raw)
			}
		}
		if e != nil {
			return nil, fmt.Errorf("could not merge %s: %w", strings.Join(c.path, " › "), e)
		}
	}
	return reformat(disk, out), nil
}

// reformat lays out merged JSON like the original when it was indented.
func reformat(original, merged []byte) []byte {
	lines := strings.SplitN(string(original), "\n", 3)
	if len(lines) < 2 {
		return merged
	}
	indent := lines[1][:len(lines[1])-len(strings.TrimLeft(lines[1], " \t"))]
	if indent == "" {
		return merged
	}
	out := pretty.PrettyOptions(merged, &pretty.Options{Indent: indent, Width: 0})
	if !bytes.HasSuffix(original, []byte("\n")) {
		out = bytes.TrimSuffix(out, []byte("\n"))
	}
	return out
}
