package main

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

const pageFileName = "page.json"

type pageDoc struct {
	Meta map[string]string `json:"meta"`
	Body json.RawMessage   `json:"body"`
}

// splitPage reads a page file into its meta and its body, which is the
// body's JSON as it is. A file that cannot be read has none of either.
func splitPage(src []byte) (map[string]string, string) {
	meta, body, _ := splitPageChecked(src)
	return meta, body
}

// splitPageChecked is splitPage that says when the file is not a page: what writes a page back must not
// take the empty body it is given then for the page's own, and write it over a file that can be mended.
func splitPageChecked(src []byte) (map[string]string, string, error) {
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

// pageMeta reads only the meta of a page file: not the body after it.
func pageMeta(src []byte) map[string]string {
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

// joinPage writes a page file.
func joinPage(meta map[string]string, body string) []byte {
	if strings.TrimSpace(body) == "" {
		body = "[]"
	}
	b, _ := json.Marshal(pageDoc{Meta: meta, Body: json.RawMessage(body)})
	return b
}

// blankText reports whether a body holds no text and no object.
func blankText(body string) bool {
	t := decodeBody(body)
	return strings.TrimSpace(strings.ReplaceAll(t.String(), "\n", "")) == ""
}

// legacyPageFile is where a page was kept when it was Markdown with front matter.
const legacyPageFile = "index.md"

// migrateLegacyPage makes the page file of a page folder that has its
// Markdown file of the time before, once: the file is taken away when the
// new one is written. It returns the page file.
func migrateLegacyPage(fsys FS, pdir string) ([]byte, error) {
	old, err := fsys.ReadFile(filepath.Join(pdir, legacyPageFile))
	if err != nil {
		return nil, err
	}
	meta, md := splitFrontMatter(string(old))
	rest, _ := splitTitle(md, meta["title"])
	src := joinPage(meta, encodeBody(importMarkdown(rest)))
	if err := fsys.WriteFile(filepath.Join(pdir, pageFileName), src); err != nil {
		return nil, err
	}
	fsys.Remove(filepath.Join(pdir, legacyPageFile))
	return src, nil
}
