package desk

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPruneBackups(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	idA, idB := fileID("/a"), fileID("/b")
	name := func(id string, age time.Duration, suffix string) string {
		n := now.Add(-age).Format(stampLayout) + "-" + id + suffix
		if e := os.WriteFile(filepath.Join(dir, n), nil, 0600); e != nil {
			t.Fatal(e)
		}
		return n
	}
	var oldA []string
	// File A: 25 old backups; only the newest 20 stay.
	for i := 0; i < 25; i++ {
		oldA = append(oldA, name(idA, time.Duration(40+i)*24*time.Hour, ".json"))
	}
	// File A also has 5 recent ones, which always stay.
	for i := 0; i < 5; i++ {
		name(idA, time.Duration(i)*time.Hour, ".json")
	}
	// File B: a few old backups, all kept because they are within its newest 20.
	for i := 0; i < 3; i++ {
		name(idB, time.Duration(100+i)*24*time.Hour, ".json")
	}
	// A skill folder backup and an unrelated file.
	os.Mkdir(filepath.Join(dir, now.Add(-200*24*time.Hour).Format(stampLayout)+"-"+idB+"-demo"), 0700)
	os.WriteFile(filepath.Join(dir, "notes.txt"), nil, 0600)

	pruneBackups(dir, now)
	entries, _ := os.ReadDir(dir)
	count := map[string]int{}
	for _, e := range entries {
		n := e.Name()
		switch {
		case len(n) > len(stampLayout)+25 && n[len(stampLayout)+1:len(stampLayout)+25] == idA:
			count["a"]++
		case len(n) > len(stampLayout)+25 && n[len(stampLayout)+1:len(stampLayout)+25] == idB:
			count["b"]++
		default:
			count[n]++
		}
	}
	// A keeps its 5 recent backups plus the 15 newest old ones (20 newest overall).
	if count["a"] != 20 || count["b"] != 4 || count["notes.txt"] != 1 {
		t.Fatalf("after pruning: %v", count)
	}
	for _, n := range oldA[15:] {
		if _, e := os.Stat(filepath.Join(dir, n)); !os.IsNotExist(e) {
			t.Fatalf("oldest backup kept: %s", n)
		}
	}
}
