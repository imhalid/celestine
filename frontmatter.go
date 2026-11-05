package main

import (
	"bytes"
	"errors"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// A page's front matter, read and written as YAML. Saving a page changes
// only the keys that are ours - title, created, order, rank and the values
// of its collection's properties - in the front matter as it is on disk:
// other keys, lists (tags: [a, b]), nested values, comments and their order
// are kept, as other tools (Obsidian, Hugo, Jekyll) wrote them.

// front is a page's front matter.
type front struct {
	doc *yaml.Node // a document holding a mapping; nil for none
	raw string     // the text between the --- lines
	bad bool       // not a YAML mapping: kept as it is, never rewritten
}

var errBadFront = errors.New("the front matter is not YAML ryudit can read: it is kept as it is, the title and properties are not written into it")

// readFront splits src into its front matter and its body.
func readFront(src string) (*front, string) {
	f := &front{}
	if !strings.HasPrefix(src, "---\n") && !strings.HasPrefix(src, "---\r\n") {
		return f, src
	}
	start := strings.Index(src, "\n") + 1
	end, after := -1, 0
	for i := start; i <= len(src); {
		j := strings.IndexByte(src[i:], '\n')
		line := src[i:]
		if j >= 0 {
			line = src[i : i+j]
		}
		if l := strings.TrimRight(line, "\r "); l == "---" || l == "..." {
			end = i
			after = i + len(line)
			if j >= 0 {
				after++
			}
			break
		}
		if j < 0 {
			break
		}
		i += j + 1
	}
	if end < 0 {
		return &front{}, src // no closing line: no front matter
	}
	f.raw = src[start:end]
	body := strings.TrimLeft(src[after:], "\r\n")
	if strings.TrimSpace(f.raw) == "" {
		return f, body
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(f.raw), &doc); err != nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		f.bad = true
		return f, body
	}
	f.doc = &doc
	return f, body
}

func (f *front) mapping() *yaml.Node {
	if f.doc == nil {
		f.doc = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	return f.doc.Content[0]
}

// value is the node of key, or nil.
func (f *front) value(key string) *yaml.Node {
	if f.doc == nil {
		return nil
	}
	m := f.mapping()
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// get is key's value as text: a list's items joined by ", "; "" for none
// or for a nested value.
func (f *front) get(key string) string {
	v := f.value(key)
	if v == nil {
		return ""
	}
	if v.Kind == yaml.AliasNode && v.Alias != nil {
		v = v.Alias
	}
	switch v.Kind {
	case yaml.ScalarNode:
		if v.Tag == "!!null" {
			return ""
		}
		return v.Value
	case yaml.SequenceNode:
		var items []string
		for _, it := range v.Content {
			if it.Kind == yaml.ScalarNode {
				items = append(items, it.Value)
			}
		}
		return strings.Join(items, ", ")
	}
	return ""
}

// keys are the front matter's keys, in order.
func (f *front) keys() []string {
	if f.doc == nil {
		return nil
	}
	m := f.mapping()
	var out []string
	for i := 0; i+1 < len(m.Content); i += 2 {
		out = append(out, m.Content[i].Value)
	}
	return out
}

// set gives key the text v; "" takes the key away.
func (f *front) set(key, v string) {
	if f.bad {
		return
	}
	m := f.mapping()
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value != key {
			continue
		}
		if v == "" {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
		m.Content[i+1] = scalar(v)
		return
	}
	if v != "" {
		m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, scalar(v))
	}
}

func (f *front) setInt(key string, n int) {
	if n <= 0 {
		f.set(key, "")
		return
	}
	f.set(key, strconv.Itoa(n))
}

// scalar is a value node: quoted when YAML would read it as something else.
func scalar(v string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.ScalarNode, Value: v}
	switch strings.ToLower(v) {
	case "true", "false", "yes", "no", "on", "off", "null", "~":
		n.Style = yaml.DoubleQuotedStyle
	}
	if strings.ContainsAny(v, ":#\"'\n[]{},&*!|>%@`") || strings.TrimSpace(v) != v {
		n.Style = yaml.DoubleQuotedStyle
	}
	return n
}

// text is the front matter's YAML, without the --- lines; "" for none.
func (f *front) text() string {
	if f.bad {
		return f.raw
	}
	if f.doc == nil || len(f.mapping().Content) == 0 {
		return ""
	}
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(f.doc); err != nil {
		return f.raw
	}
	enc.Close()
	return b.String()
}

// splitFrontMatter is src's front matter as text values, and its body.
func splitFrontMatter(src string) (map[string]string, string) {
	f, body := readFront(src)
	meta := map[string]string{}
	for _, k := range f.keys() {
		meta[k] = f.get(k)
	}
	return meta, body
}
