package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// The library as Markdown: a folder for each collection, a .md file for each page, its front matter (title,
// when it was made, the values of its properties) over its text; the sub-pages of a page in a folder of its
// name beside it, and the pictures of a page beside it. What "Markdown klasörü içe aktar" reads, and what
// Obsidian or any editor opens. (The library's own folder form, page.json, is what backups keep.)

// exportMarkdown writes the library's collections under dir, which is made.
func (l *Library) exportMarkdown(dir string) error {
	osf := osFS{}
	if err := osf.MkdirAll(dir); err != nil {
		return err
	}
	taken := map[string]bool{}
	for _, c := range l.Collections {
		if c.Dynamic() { // a search: its pages are found, not kept
			continue
		}
		folder := filepath.Join(dir, uniqueName(taken, fileSafe(c.Name)))
		if err := osf.MkdirAll(folder); err != nil {
			return err
		}
		if err := exportPages(osf, c.Pages, folder); err != nil {
			return err
		}
	}
	return nil
}

// exportPages writes pages into folder, each page's sub-pages in a folder of its name beside it.
func exportPages(dst FS, pages []*Page, folder string) error {
	taken := map[string]bool{}
	for _, p := range pages {
		name := uniqueName(taken, fileSafe(p.Title))
		body, err := p.Text()
		if err != nil {
			return fmt.Errorf("%s: %w", p.Title, err)
		}
		// the pictures of the page, beside its file: the text names them by their file name
		ents, _ := p.fs().ReadDir(p.Dir)
		for _, e := range ents {
			if e.IsDir() || e.Name() == pageFileName || e.Name() == legacyPageFile || strings.HasPrefix(e.Name(), "page (") || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			data, err := p.fs().ReadFile(filepath.Join(p.Dir, e.Name()))
			if err != nil {
				continue
			}
			to := filepath.Join(folder, e.Name())
			if old, err := dst.ReadFile(to); err == nil && string(old) != string(data) { // another page's file by the same name
				renamed := name + " - " + e.Name()
				body = strings.ReplaceAll(body, "("+e.Name()+")", "("+strings.ReplaceAll(renamed, " ", "%20")+")")
				to = filepath.Join(folder, renamed)
			}
			if err := dst.WriteFile(to, data); err != nil {
				return err
			}
		}
		if err := dst.WriteFile(filepath.Join(folder, name+".md"), []byte(frontMatterOf(p)+body)); err != nil {
			return err
		}
		if len(p.Children) > 0 {
			sub := filepath.Join(folder, name)
			if err := dst.MkdirAll(sub); err != nil {
				return err
			}
			if err := exportPages(dst, p.Children, sub); err != nil {
				return err
			}
		}
	}
	return nil
}

// frontMatterOf is page p's front matter, YAML between --- lines and a blank line after: its title, when it
// was made, then its properties' values by their names, in order.
func frontMatterOf(p *Page) string {
	m := &yaml.Node{Kind: yaml.MappingNode}
	add := func(k, v string) {
		m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k}, &yaml.Node{Kind: yaml.ScalarNode, Value: v, Style: quoteStyle(v)})
	}
	add("title", p.Title)
	if !p.Created.IsZero() {
		add("created", p.Created.Format(time.RFC3339))
	}
	keys := make([]string, 0, len(p.Props))
	for k, v := range p.Props {
		if v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		add(k, p.Props[k])
	}
	out, err := yaml.Marshal(m)
	if err != nil {
		return ""
	}
	return "---\n" + string(out) + "---\n\n"
}

// quoteStyle is the YAML style of a value: double-quoted when a reader might take it for something other than
// the text it is (a number, a date, true, null, a list), else plain.
func quoteStyle(v string) yaml.Style {
	var any interface{}
	if err := yaml.Unmarshal([]byte(v), &any); err != nil {
		return yaml.DoubleQuotedStyle
	}
	if _, ok := any.(string); !ok || strings.ContainsAny(v, ":#\n\"'") || strings.TrimSpace(v) != v {
		return yaml.DoubleQuotedStyle
	}
	return 0
}

// fileSafe is a title as a file or folder name: what a file system or a link refuses is a dash, and it is not
// empty, nor made of dots, nor long.
func fileSafe(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, strings.TrimSpace(s))
	s = strings.Trim(s, ". ")
	if r := []rune(s); len(r) > 100 {
		s = strings.TrimSpace(string(r[:100]))
	}
	if s == "" {
		s = "Adsız"
	}
	return s
}

// uniqueName is name, or name 2, 3… when it was taken (by case too: a disk may not tell them apart).
func uniqueName(taken map[string]bool, name string) string {
	n := name
	for i := 2; taken[strings.ToLower(n)]; i++ {
		n = fmt.Sprintf("%s %d", name, i)
	}
	taken[strings.ToLower(n)] = true
	return n
}
