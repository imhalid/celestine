package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var archTableTokenRe = regexp.MustCompile("`([^`]+)`")

// TestArchitectureLayoutComplete verifies that every non-test Go file and top-level
// resource directory is explicitly listed in the ## Layout table of docs/architecture.md.
func TestArchitectureLayoutComplete(t *testing.T) {
	docBytes, err := os.ReadFile(filepath.Join("docs", "architecture.md"))
	if err != nil {
		t.Fatalf("read docs/architecture.md: %v", err)
	}
	doc := string(docBytes)

	inLayout := false
	listed := map[string]bool{}
	for _, line := range strings.Split(doc, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "## ") {
			if inLayout {
				break
			}
			inLayout = trim == "## Layout"
			continue
		}
		if !inLayout || !strings.HasPrefix(trim, "|") {
			continue
		}
		cells := strings.Split(trim, "|")
		if len(cells) < 3 {
			continue
		}
		for _, m := range archTableTokenRe.FindAllStringSubmatch(cells[1], -1) {
			listed[strings.TrimSpace(m[1])] = true
		}
	}

	for _, dir := range []string{"internal/library/", "locales/", "web/", "android/", "cloudflare/", "scripts/"} {
		if !listed[dir] {
			t.Errorf("directory %q missing from docs/architecture.md Layout table", dir)
		}
	}

	skipDirs := map[string]bool{
		".git":       true,
		"prototypes": true,
		"cloudflare": true,
		"web":        true,
		"android":    true,
		"locales":    true, // documented as locales/
		"scripts":    true, // documented as scripts/
	}

	var missing []string
	err = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] || (path != "." && strings.HasPrefix(d.Name(), ".")) { // .git, .claude (worktrees of other sessions) are not the app
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		rel := filepath.ToSlash(path)
		if listed[rel] {
			return nil
		}
		if filepath.Dir(rel) == "." && strings.HasPrefix(name, "caretfx") && listed["caretfx*.go"] {
			return nil
		}
		missing = append(missing, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
	if len(missing) > 0 {
		t.Errorf("non-test .go files missing from docs/architecture.md Layout table:\n  %s", strings.Join(missing, "\n  "))
	}
}
