package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Markdown is only the file format: blocks are written out as Markdown and
// read back from it, so a document survives the trip and any other tool can
// open it. Inside the editor there is no Markdown.

// toMarkdown writes the document.
func toMarkdown(blocks []*Block) string {
	var sb strings.Builder
	for i, b := range blocks {
		if i > 0 {
			prev := blocks[i-1]
			// List items of a list sit line by line; everything else gets a blank line between.
			if !(prev.Kind.isList() && b.Kind.isList()) && !(prev.Kind == Quote && b.Kind == Quote) {
				sb.WriteString("\n")
			}
		}
		indent := strings.Repeat("  ", b.Depth)
		text := inlineMarkdown(b)
		switch b.Kind {
		case Heading1, Heading2, Heading3:
			sb.WriteString(strings.Repeat("#", int(b.Kind-Heading1)+1) + " " + oneLine(text))
		case Bullet:
			sb.WriteString(indent + "- " + continued(text, indent+"  "))
		case Numbered:
			sb.WriteString(indent + fmt.Sprintf("%d. ", numberIn(blocks, i)) + continued(text, indent+"   "))
		case Todo:
			box := "[ ]"
			if b.Checked {
				box = "[x]"
			}
			sb.WriteString(indent + "- " + box + " " + continued(text, indent+"  "))
		case Quote:
			sb.WriteString("> " + continued(text, "> "))
		case Code:
			fence := "```"
			for strings.Contains(b.Text, fence) {
				fence += "`"
			}
			sb.WriteString(fence + b.Lang + "\n" + b.Text + "\n" + fence)
		case Divider:
			sb.WriteString("---")
		default:
			sb.WriteString(escapeLineStart(continued(text, "")))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func numberIn(blocks []*Block, i int) int { return (&Doc{Blocks: blocks}).numberOf(i) }

// continued writes a block's line breaks as Markdown hard breaks, each next
// line indented to stay in the block.
func continued(text, indent string) string {
	return strings.ReplaceAll(text, "\n", "  \n"+indent)
}

func oneLine(text string) string { return strings.ReplaceAll(text, "\n", " ") }

var lineStartMarks = regexp.MustCompile(`^(#{1,6} |[-*+] |\d+[.)] |> |---$|\x60\x60\x60)`)

// escapeLineStart keeps a paragraph that begins like a block mark a paragraph.
func escapeLineStart(s string) string {
	if lineStartMarks.MatchString(s) {
		return `\` + s
	}
	return s
}

var mdDelim = map[MarkKind]string{Bold: "**", Italic: "_", InlineCode: "`", Strike: "~~"}

// inlineMarkdown writes a block's text with its marks as delimiters; marks
// open and close at each boundary, innermost last, so they always nest.
func inlineMarkdown(b *Block) string {
	if b.Kind == Code {
		return b.Text
	}
	cuts := map[int]bool{0: true, b.len(): true}
	for _, m := range b.Marks {
		cuts[m.S], cuts[m.E] = true, true
	}
	var at []int
	for c := range cuts {
		at = append(at, c)
	}
	sort.Ints(at)
	var sb strings.Builder
	order := []MarkKind{Bold, Italic, Strike, InlineCode}
	for i := 0; i+1 < len(at); i++ {
		s, e := at[i], at[i+1]
		seg := b.slice(s, e)
		var on []MarkKind
		for _, k := range order {
			if b.hasMark(k, s, e) {
				on = append(on, k)
			}
		}
		// Spaces at a segment's edges go outside the delimiters ("** x**" is not bold).
		lead := len(seg) - len(strings.TrimLeft(seg, " "))
		trail := len(seg) - len(strings.TrimRight(seg, " "))
		if lead == len(seg) {
			sb.WriteString(seg)
			continue
		}
		core := seg[lead : len(seg)-trail]
		sb.WriteString(seg[:lead])
		for _, k := range on {
			sb.WriteString(mdDelim[k])
		}
		if len(on) > 0 && on[len(on)-1] == InlineCode {
			sb.WriteString(core)
		} else {
			sb.WriteString(escapeInline(core))
		}
		for j := len(on) - 1; j >= 0; j-- {
			sb.WriteString(mdDelim[on[j]])
		}
		sb.WriteString(seg[len(seg)-trail:])
	}
	return sb.String()
}

func escapeInline(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `*`, `\*`, `_`, `\_`, "`", "\\`", `~`, `\~`)
	return r.Replace(s)
}

// parseMarkdown reads Markdown into blocks.
func parseMarkdown(src string) []*Block {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(src, "\n")
	var out []*Block
	var para *Block // the paragraph being gathered
	flush := func() { para = nil }
	heading := regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	list := regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])\s+(.*)$`)
	task := regexp.MustCompile(`^\[( |x|X)\]\s+(.*)$`)
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```"):
			flush()
			fence := trimmed[:len(trimmed)-len(strings.TrimLeft(trimmed, "`"))]
			b := newBlock(Code, "")
			b.Lang = strings.TrimSpace(trimmed[len(fence):])
			var body []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), fence); i++ {
				body = append(body, lines[i])
			}
			b.Text = strings.Join(body, "\n")
			out = append(out, b)
		case trimmed == "":
			flush()
		case trimmed == "---" || trimmed == "***" || trimmed == "___":
			flush()
			out = append(out, newBlock(Divider, ""))
		case heading.MatchString(trimmed):
			flush()
			m := heading.FindStringSubmatch(trimmed)
			k := Heading1 + Kind(min(3, len(m[1]))-1)
			out = append(out, inlineBlock(k, m[2]))
		case list.MatchString(line):
			flush()
			m := list.FindStringSubmatch(line)
			k, text := Bullet, m[3]
			if m[2][0] >= '0' && m[2][0] <= '9' {
				k = Numbered
			}
			checked := false
			if t := task.FindStringSubmatch(text); t != nil && k == Bullet {
				k, text, checked = Todo, t[2], t[1] != " "
			}
			b := inlineBlock(k, hardBreaks(text))
			b.Checked = checked
			b.Depth = min(6, len(strings.ReplaceAll(m[1], "\t", "  "))/2)
			out = append(out, b)
			para = b
		case strings.HasPrefix(trimmed, ">"):
			flush()
			text := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
			if n := len(out); n > 0 && out[n-1].Kind == Quote && i > 0 && strings.HasPrefix(strings.TrimSpace(lines[i-1]), ">") && strings.HasSuffix(lines[i-1], "  ") {
				out[n-1].join(inlineBlock(Paragraph, "\n"+text))
				continue
			}
			out = append(out, inlineBlock(Quote, text))
		case para != nil:
			// A line that goes on the paragraph (or list item) above: a hard break
			// when that line ended in two spaces, else a space.
			sep := " "
			if strings.HasSuffix(lines[i-1], "  ") || strings.HasSuffix(lines[i-1], `\`) {
				sep = "\n"
			}
			para.join(inlineBlock(Paragraph, sep+trimmed))
		default:
			b := inlineBlock(Paragraph, unescapeLineStart(trimmed))
			out = append(out, b)
			para = b
		}
	}
	for _, b := range out {
		b.Text = strings.TrimRight(b.Text, " ")
		b.normalize()
	}
	return out
}

func hardBreaks(s string) string { return strings.TrimRight(s, " ") }

func unescapeLineStart(s string) string {
	if strings.HasPrefix(s, `\`) && lineStartMarks.MatchString(s[1:]) {
		return s[1:]
	}
	return s
}

// inlineBlock reads a line's inline Markdown into a block's text and marks.
// A delimiter opens only before a non-space and closes only after one, as
// CommonMark has it ("2 * 3" stays as it is); one never closed is put back as
// the literal it was.
func inlineBlock(k Kind, src string) *Block {
	b := newBlock(k, "")
	var sb strings.Builder
	n := 0 // UTF-16 length written so far
	type opening struct {
		at    int
		delim string
	}
	open := map[MarkKind]opening{}
	write := func(s string) { sb.WriteString(s); n += ui.UTF16Len(s) }
	space := func(j int) bool { return j < 0 || j >= len(src) || src[j] == ' ' || src[j] == '\t' }
	delim := func(k MarkKind, i, w int) {
		if o, ok := open[k]; ok && !space(i-1) {
			b.Marks = append(b.Marks, Mark{K: k, S: o.at, E: n})
			delete(open, k)
			return
		}
		if _, ok := open[k]; !ok && !space(i+w) {
			open[k] = opening{n, src[i : i+w]}
			return
		}
		write(src[i : i+w])
	}
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '\\' && i+1 < len(src) && strings.ContainsRune("\\*_`~#->[]", rune(src[i+1])):
			write(src[i+1 : i+2])
			i += 2
		case c == '`':
			end := strings.Index(src[i+1:], "`")
			if end < 0 {
				write("`")
				i++
				continue
			}
			s := n
			write(src[i+1 : i+1+end])
			b.Marks = append(b.Marks, Mark{K: InlineCode, S: s, E: n})
			i += end + 2
		case strings.HasPrefix(src[i:], "**") || strings.HasPrefix(src[i:], "__"):
			delim(Bold, i, 2)
			i += 2
		case strings.HasPrefix(src[i:], "~~"):
			delim(Strike, i, 2)
			i += 2
		case c == '*' || c == '_':
			if c == '_' && i > 0 && isWordByte(src[i-1]) && i+1 < len(src) && isWordByte(src[i+1]) {
				write("_") // snake_case
			} else {
				delim(Italic, i, 1)
			}
			i++
		default:
			j := i + 1
			for j < len(src) && !strings.ContainsRune("\\`*_~", rune(src[j])) {
				j++
			}
			write(src[i:j])
			i = j
		}
	}
	b.Text = sb.String()
	// Delimiters never closed go back in as text, the last first so the
	// earlier places stay right.
	var left []opening
	for _, o := range open {
		left = append(left, o)
	}
	sort.Slice(left, func(i, j int) bool { return left[i].at > left[j].at })
	for _, o := range left {
		b.insert(o.at, o.delim, []MarkKind{})
	}
	b.normalize()
	return b
}
