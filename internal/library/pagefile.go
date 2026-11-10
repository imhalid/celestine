package library

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
)

// A page is a file of its own format, page.json in the page's folder: what
// is known of the page (its title, when it was made, its place, its
// properties' values) and its body (body.go):
//
//	{"meta":{"created":"2026-11-05T10:00:00+03:00","order":"1","title":"Dune"},
//	 "body":[{"s":{"heading":1},"r":[{"t":"Dune"}]}]}
//
// Meta comes first, so that a list of pages reads only that of each.

const PageFileName = "page.json"

type pageDoc struct {
	Meta map[string]string `json:"meta"`
	Body json.RawMessage   `json:"body"`
}

// SplitPage reads a page file into its meta and its body, which is the
// body's JSON as it is. A file that cannot be read has none of either.
func SplitPage(src []byte) (map[string]string, string) {
	meta, body, _ := SplitPageChecked(src)
	return meta, body
}

// SplitPageChecked is splitPage that says when the file is not a page: what writes a page back must not
// take the empty body it is given then for the page's own, and write it over a file that can be mended.
func SplitPageChecked(src []byte) (map[string]string, string, error) {
	var d pageDoc
	if len(bytes.TrimSpace(src)) == 0 { // nothing yet: an empty page
		return map[string]string{}, "[]", nil
	}
	if err := json.Unmarshal(src, &d); err != nil {
		return map[string]string{}, "[]", errors.New("sayfa dosyası bozuk: " + err.Error())
	}
	if d.Meta == nil {
		d.Meta = map[string]string{}
	}
	body := strings.TrimSpace(string(d.Body))
	if body == "" || body == "null" {
		body = "[]"
	}
	return d.Meta, body, nil
}

// PageMeta reads only the meta of a page file: not the body after it.
func PageMeta(src []byte) map[string]string {
	dec := json.NewDecoder(bytes.NewReader(src))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return map[string]string{}
	}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			break
		}
		if key == "meta" {
			var m map[string]string
			if dec.Decode(&m) == nil && m != nil {
				return m
			}
			break
		}
		var skip json.RawMessage
		if dec.Decode(&skip) != nil {
			break
		}
	}
	return map[string]string{}
}

// JoinPage writes a page file. A body that is not JSON makes nil: what writes a page uses JoinPageChecked.
func JoinPage(meta map[string]string, body string) []byte {
	b, _ := JoinPageChecked(meta, body)
	return b
}

// JoinPageChecked writes a page file, or says why it cannot: a body that is not JSON would make an empty file,
// which reads back as an empty page with no title (#135).
func JoinPageChecked(meta map[string]string, body string) ([]byte, error) {
	if strings.TrimSpace(body) == "" {
		body = "[]"
	}
	if !json.Valid([]byte(body)) {
		return nil, errors.New("the page's text could not be written: its body is not valid")
	}
	return json.Marshal(pageDoc{Meta: meta, Body: json.RawMessage(body)})
}

// blankText reports whether a body holds no text and no object.
func blankText(body string) bool {
	return Format.Blank(body)
}

// LegacyPageFile is where a page was kept when it was Markdown with front matter.
const LegacyPageFile = "index.md"

// migrateLegacyPage makes the page file of a page folder that has its
// Markdown file of the time before, once: the file is taken away when the
// new one is written. It returns the page file.
func migrateLegacyPage(fsys FS, pdir string) ([]byte, error) {
	old, err := fsys.ReadFile(filepath.Join(pdir, LegacyPageFile))
	if err != nil {
		return nil, err
	}
	meta, md := SplitFrontMatter(string(old))
	rest, _ := SplitTitle(md, meta["title"])
	src, err := JoinPageChecked(meta, Format.FromMarkdown(rest))
	if err != nil {
		return nil, err
	}
	if err := fsys.WriteFile(filepath.Join(pdir, PageFileName), src); err != nil {
		return nil, err
	}
	fsys.Remove(filepath.Join(pdir, LegacyPageFile))
	return src, nil
}

// SplitTitle takes the first heading of md off when it is the title: the
// rest, and what was taken (the heading and the blank lines after it).
func SplitTitle(md, title string) (rest, head string) {
	lines := strings.SplitAfter(md, "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) {
		return md, ""
	}
	line := strings.TrimSpace(lines[i])
	if !strings.HasPrefix(line, "# ") || !strings.EqualFold(strings.TrimSpace(strings.TrimRight(line[2:], "# ")), strings.TrimSpace(title)) {
		return md, ""
	}
	j := i + 1
	for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
		j++
	}
	return strings.Join(lines[j:], ""), strings.Join(lines[:j], "")
}
