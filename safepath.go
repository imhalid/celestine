package main

import (
	"path/filepath"
	"strings"
)

// Names that come from outside - a sync's remote, a vault being imported, a note in the trash - are not
// trusted to stay in the folder they are joined to.

// safeName says whether name, slash-separated, is a name inside a folder: not empty, not absolute, with no
// ".." segment (either slash), no NUL, no drive.
func safeName(name string) bool {
	if name == "" || strings.ContainsRune(name, 0) || strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) {
		return false
	}
	if filepath.VolumeName(filepath.FromSlash(name)) != "" || filepath.IsAbs(filepath.FromSlash(name)) {
		return false
	}
	for _, seg := range strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return false
		}
	}
	return true
}

// safeSubpath is root joined with name, or false when name is not safe (safeName).
func safeSubpath(root, name string) (string, bool) {
	if !safeName(name) {
		return "", false
	}
	return filepath.Join(root, filepath.FromSlash(name)), true
}

// within says whether p, once cleaned, is root or below it.
func within(root, p string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(p))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
