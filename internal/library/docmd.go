package library

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/imhalid/celestine/internal/table"
)

// The library's own reading and writing of a document as Markdown (#595): its paragraphs' kinds (headings, lists,
// tasks, quotes, blocks of code), the marks of its runs, and its objects as their Markdown ([[Page]], ![[Embed]],
// [ ] and [x], [^note], a picture, a table). It is what the library sees of a body without the app: its tags, tasks,
// links and words. The app's editor reads and writes Markdown more fully (escapes, HTML, math, callouts, the looks of
// lists), with MyGo's richtext, which the library does not import: it gives that format to the library (Format).

// The kinds of list a paragraph is an item of, as the page file numbers them (the editor's).
const (
	ListNone = iota
	ListBullets
	ListNumbers
	ListTasks
)

// Markdown is the document as Markdown, a line to a paragraph.
func (d Doc) Markdown() string {
	var lines []string
	n := map[int]int{} // the number of a numbered item, by its indent
	inCode := false
	for _, p := range d {
		st := ParaStyle{}
		if p.S != nil {
			st = *p.S
		}
		if st.CodeBlock && st.List == ListNone && !st.Quote {
			lang := strings.TrimPrefix(st.ID, "celestine:code:")
			if !inCode || strings.HasPrefix(st.ID, "celestine:code:") || strings.HasPrefix(st.ID, "celestine:raw:") {
				if inCode {
					lines = append(lines, "```")
				}
				if strings.HasPrefix(st.ID, "celestine:raw:") {
					lang = ""
				}
				lines = append(lines, "```"+lang)
				inCode = true
			}
			lines = append(lines, p.text())
			continue
		}
		if inCode {
			lines = append(lines, "```")
			inCode = false
		}
		var b strings.Builder
		if st.Quote {
			b.WriteString("> ")
		}
		b.WriteString(strings.Repeat("  ", st.Indent))
		switch st.List {
		case ListBullets:
			b.WriteString("- ")
		case ListNumbers:
			n[st.Indent]++
			b.WriteString(strconv.Itoa(n[st.Indent]) + ". ")
		case ListTasks:
			b.WriteString(map[bool]string{true: "- [x] ", false: "- [ ] "}[st.Checked])
		}
		if st.List != ListNumbers {
			delete(n, st.Indent)
		}
		if st.Heading > 0 {
			b.WriteString(strings.Repeat("#", st.Heading) + " ")
		}
		for _, r := range p.R {
			b.WriteString(r.markdown())
		}
		lines = append(lines, b.String())
	}
	if inCode {
		lines = append(lines, "```")
	}
	return strings.Join(lines, "\n")
}

// text is the paragraph's text, its objects as the character they stand as.
func (p Para) text() string {
	var b strings.Builder
	for _, r := range p.R {
		if r.O != nil {
			b.WriteString(ObjectChar)
			continue
		}
		b.WriteString(r.T)
	}
	return b.String()
}

// markdown is the run as Markdown: its text in its marks, or its object.
func (r Run) markdown() string {
	if o := r.O; o != nil {
		switch o.K {
		case "page", "date":
			if o.Alias != "" {
				return "[[" + o.Target + "|" + o.Alias + "]]"
			}
			return "[[" + o.Target + "]]"
		case "embed":
			return "![[" + o.Target + "]]"
		case "footnote":
			return "[^" + o.Target + "]"
		case "check":
			return map[bool]string{true: "[x]", false: "[ ]"}[o.Target == "x"]
		case "file":
			return "[" + o.Target + "](" + o.Target + ")"
		case "image":
			return "![" + o.Alt + "](" + o.Ref + ")"
		case "table":
			return (&table.Model{Cols: o.Cols, Rows: o.Rows}).Text()
		case "query":
			return "```query\n" + o.Q + "\n```"
		}
		return ""
	}
	t := r.T
	if t == "" {
		return ""
	}
	if r.C {
		return "`" + t + "`"
	}
	if r.L != "" {
		t = "[" + t + "](" + r.L + ")"
	}
	if r.X {
		t = "~~" + t + "~~"
	}
	if r.I {
		t = "*" + t + "*"
	}
	if r.W >= 600 {
		t = "**" + t + "**"
	}
	return t
}

// DocFromMarkdown reads Markdown into a document, a paragraph to a line: headings, lists, tasks, quotes and fenced
// blocks of code, and [[links]], ![[embeds]] and [x] boxes in the text; the rest of the text as it is.
func DocFromMarkdown(md string) Doc {
	md = strings.ReplaceAll(strings.TrimPrefix(md, "\uFEFF"), "\r\n", "\n")
	var d Doc
	fence, lang, first := false, "", false
	for _, l := range strings.Split(md, "\n") {
		if t := strings.TrimSpace(l); strings.HasPrefix(t, "```") {
			if !fence {
				fence, lang, first = true, strings.TrimSpace(strings.TrimPrefix(t, "```")), true
			} else {
				fence = false
			}
			continue
		}
		if fence {
			st := ParaStyle{CodeBlock: true}
			if first && lang != "" {
				st.ID = "celestine:code:" + lang
			}
			first = false
			d = append(d, Para{S: &st, R: []Run{{T: l}}})
			continue
		}
		st := ParaStyle{}
		if q, ok := strings.CutPrefix(l, "> "); ok {
			st.Quote, l = true, q
		}
		ind := len(l) - len(strings.TrimLeft(l, " \t"))
		rest := strings.TrimLeft(l, " \t")
		if m := mdTask.FindStringSubmatch(rest); m != nil {
			st.List, st.Checked, rest = ListTasks, m[1] == "x" || m[1] == "X", rest[len(m[0]):]
		} else if m := mdBullet.FindString(rest); m != "" {
			st.List, rest = ListBullets, rest[len(m):]
		} else if m := mdNumber.FindString(rest); m != "" {
			st.List, rest = ListNumbers, rest[len(m):]
		}
		if st.List != ListNone {
			st.Indent = ind / 2
		}
		if m := mdHeading.FindString(rest); m != "" {
			st.Heading, rest = strings.Count(m, "#"), rest[len(m):]
		}
		p := Para{R: runsOf(rest)}
		if st != (ParaStyle{}) {
			p.S = &st
		}
		if st == (ParaStyle{}) && rest == "" {
			p.R = nil
		}
		d = append(d, p)
	}
	for len(d) > 0 && d[len(d)-1].S == nil && len(d[len(d)-1].R) == 0 {
		d = d[:len(d)-1]
	}
	return d
}

var (
	mdTask    = regexp.MustCompile(`^[-*+]\s+\[([ xX])\]\s?`)
	mdBullet  = regexp.MustCompile(`^[-*+]\s+`)
	mdNumber  = regexp.MustCompile(`^\d+[.)]\s+`)
	mdHeading = regexp.MustCompile(`^#{1,6}\s+`)
	mdObject  = regexp.MustCompile(`!?\[\[([^\[\]|]+)(?:\|([^\[\]]+))?\]\]|\[([ xX])\]`)
	mdDay     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// runsOf is a line's text as runs: its links to pages, its embeds and its boxes objects, the rest text.
func runsOf(s string) []Run {
	var out []Run
	at := 0
	for _, m := range mdObject.FindAllStringSubmatchIndex(s, -1) {
		if m[0] > at {
			out = append(out, Run{T: s[at:m[0]]})
		}
		var o Obj
		switch {
		case m[6] >= 0:
			o = Obj{K: "check"}
			if c := s[m[6]:m[7]]; c != " " {
				o.Target = "x"
			}
		case s[m[0]] == '!':
			o = Obj{K: "embed", Target: s[m[2]:m[3]]}
		default:
			o = Obj{K: "page", Target: s[m[2]:m[3]]}
			if mdDay.MatchString(o.Target) {
				o.K = "date"
			}
			if m[4] >= 0 {
				o.Alias = s[m[4]:m[5]]
			}
		}
		out = append(out, Run{T: ObjectChar, O: &o})
		at = m[1]
	}
	if at < len(s) {
		out = append(out, Run{T: s[at:]})
	}
	return out
}

// docFormat is the library's own format of the bodies (Format): page.json's document, read and written as Markdown
// by the library alone. A body that is a JSON string is read as its Markdown, as the library's tests once wrote them.
type docFormat struct{}

func (docFormat) Markdown(body string) string {
	if d, err := DecodeBody(body); err == nil {
		return d.Markdown()
	}
	var md string
	if json.Unmarshal([]byte(body), &md) == nil {
		return md
	}
	return ""
}

func (f docFormat) Plain(body string) string {
	if d, err := DecodeBody(body); err == nil {
		return d.Plain()
	}
	return f.Markdown(body)
}

func (f docFormat) Blank(body string) bool {
	if d, err := DecodeBody(body); err == nil {
		return d.Blank()
	}
	return strings.TrimSpace(f.Markdown(body)) == ""
}

func (docFormat) FromMarkdown(md string) string { return EncodeBody(DocFromMarkdown(md)) }
