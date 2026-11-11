package desk

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxSkillFiles = 1000
	maxSkillBytes = 50 << 20
)

// skillRoots returns the editable skill locations for an agent.
func (s *Server) skillRoots(agent, project string) []string {
	roots := []string{}
	for _, sp := range specs() {
		if sp.id != agent {
			continue
		}
		for _, p := range sp.skills {
			if !strings.Contains(p, "builtin") && !strings.Contains(p, "bundled") {
				roots = append(roots, filepath.Join(s.Home, p))
			}
		}
		if project != "" {
			for _, p := range sp.projectSkills {
				roots = append(roots, filepath.Join(project, p))
			}
		}
	}
	return roots
}

// skillFolder checks that the folder holding a skill's SKILL.md can be removed
// as one unit. It returns the folder and the number of files in it.
func (s *Server) skillFolder(f File, project string) (string, int, error) {
	if f.Kind != "skill" || filepath.Base(f.Path) != "SKILL.md" {
		return "", 0, fmt.Errorf("not a skill definition")
	}
	if !f.Editable {
		return "", 0, fmt.Errorf("this skill is read-only")
	}
	dir := filepath.Dir(f.Path)
	known := false
	for _, root := range s.skillRoots(f.Agent, project) {
		if dir != root && within(root, dir) {
			known = true
		}
	}
	if !known {
		return "", 0, fmt.Errorf("this skill is not inside a known skills folder")
	}
	if e := s.safeWrite(f.Path); e != nil {
		return "", 0, e
	}
	count, size := 0, int64(0)
	e := filepath.WalkDir(dir, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			return fmt.Errorf("the skill folder contains a symbolic link; remove it manually")
		case d.IsDir():
			return nil
		case !d.Type().IsRegular():
			return fmt.Errorf("the skill folder contains a special file; remove it manually")
		case d.Name() == "SKILL.md" && p != f.Path:
			return fmt.Errorf("the skill folder contains another skill; remove it manually")
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		count++
		size += info.Size()
		if count > maxSkillFiles || size > maxSkillBytes {
			return fmt.Errorf("the skill folder is larger than 1,000 files or 50 MB; remove it manually")
		}
		return nil
	})
	return dir, count, e
}

// removeSkill copies a skill's folder into the backups and then deletes it.
func (s *Server) removeSkill(f File, project, expected string) (string, error) {
	dir, _, e := s.skillFolder(f, project)
	if e != nil {
		return "", e
	}
	old, e := readSmall(f.Path)
	if e != nil {
		return "", e
	}
	if digest(old) != expected {
		return "", errConflict
	}
	root, e := s.backupDir()
	if e != nil {
		return "", e
	}
	backup := filepath.Join(root, stamp()+"-"+fileID(dir)+"-"+filepath.Base(dir))
	if e = copyTree(dir, backup); e != nil {
		os.RemoveAll(backup)
		return "", fmt.Errorf("could not back up the skill folder: %w", e)
	}
	return backup, os.RemoveAll(dir)
}

// copyTree copies regular files and folders from src to a new folder dst.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(src, p)
		if e != nil {
			return e
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s is not a regular file", rel)
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		return os.WriteFile(target, b, info.Mode().Perm())
	})
}
