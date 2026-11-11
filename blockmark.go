package main

import (
	"encoding/base64"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/richtext"
)

// A block of a kind this version does not know (body.go) has nothing in Markdown to say it. In Markdown it is a
// line of its own that no reader shows, an HTML comment holding the block as the file has it:
//
//	<!--celestine:block eyJrIjoiYWktYW5zd2VyIn0-->
//
// and Markdown read in turns that line back into the block, as it was. So an agent that reads a page and writes
// it back whole (MCP), or a page exported and imported again, keeps it.

const blockMarkStart = "<!--celestine:block "

var blockMark = regexp.MustCompile(`^<!--celestine:block ([A-Za-z0-9+/]+)-->$`)

// blockMarkAsText puts the mark of the block in chip ch, at rune i of t, on a line of its own, as a raw HTML block.
func blockMarkAsText(t richtext.Text, i int, ch *chip, plain richtext.Attributes) richtext.Text {
	mark := blockMarkStart + base64.RawStdEncoding.EncodeToString(ch.raw) + "-->"
	ps, pe := t.ParagraphRange(i)
	if pe > i+1 { // text after it
		t = t.ReplaceString(i+1, i+1, "\n", plain)
	}
	if ps < i { // and before
		t = t.ReplaceString(i, i, "\n", plain)
		i++
	}
	t = t.ReplaceString(i, i+1, mark, plain)
	end := i + utf8.RuneCountInString(mark)
	return t.FormatParagraphs(i, end, func(p *richtext.ParagraphStyle) {
		*p = richtext.ParagraphStyle{CodeBlock: true, ID: rawIDPrefix + "html"}
	})
}

// blockMarksIn turns the marks of blocks in t, read from Markdown, back into the blocks.
func blockMarksIn(t richtext.Text) richtext.Text {
	if !strings.Contains(t.String(), blockMarkStart) {
		return t
	}
	ps := starts(t)
	for k := len(ps) - 1; k >= 0; k-- {
		s, e := t.ParagraphRange(ps[k])
		m := blockMark.FindStringSubmatch(strings.TrimSpace(t.Slice(s, e).String()))
		if m == nil {
			continue
		}
		raw, err := base64.RawStdEncoding.DecodeString(m[1])
		if err != nil || len(raw) == 0 {
			continue
		}
		ch := &chip{kind: unknownKind, raw: raw}
		t = t.ReplaceString(s, e, string(richtext.ObjectReplacement), richtext.Attributes{Attachment: &richtext.Attachment{Name: ch.label(), Data: ch}})
		t = t.FormatParagraphs(s, s, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{} })
	}
	return t
}
