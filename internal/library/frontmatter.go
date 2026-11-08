package library

import (
	"strings"

	"go.yaml.in/yaml/v3"
)

// The front matter of a Markdown file that comes in from outside (an import,
// a file dropped), read as YAML. Pages are not kept this way (pagefile.go).

// front is a page's front matter.
type front struct {
	doc *yaml.Node // a document holding a mapping; nil for none
	raw string     // the text between the --- lines
	bad bool       // not a YAML mapping: kept as it is, never rewritten
}

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

// SplitFrontMatter is src's front matter as text values, and its body.
func SplitFrontMatter(src string) (map[string]string, string) {
	f, body := readFront(src)
	meta := map[string]string{}
	for _, k := range f.keys() {
		meta[k] = f.get(k)
	}
	return meta, body
}
