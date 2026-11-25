package main

import (
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/internal/textnorm"
)

// Lists of the app's own look and tasks in the running text, in Markdown (#536). Markdown has no word for either
// (listmarks.go), so each is written as Markdown other readers show as it is meant, and Markdown read in turns it
// back:
//
//	Buy [ ] milk [x]                 a task in the running text, a chip: a box not escaped, in the running text
//	1. [ ] Call                      a list of tasks with a number before each box (task=number)
//	<!--celestine:list mark=%E2%AD%90-->
//	- First                          the look of the list under it, as its item's ID says it, in a comment no
//	                                 reader shows, on the line right over the list's first item, as far in
//
// A box is a task in the running text only where nothing else reads it (inlineBoxes); a chip anywhere else is
// written escaped, as it was before, and comes back as the text "[ ]". Markdown without a comment, or with a box
// escaped, reads as it did. A look read back is said by the first item of its list that has no ID: the first,
// unless it is a task of a kind (taskkinds.go), whose ID says its kind.
//
// While richtext reads or writes the text around them, each is a character of Unicode's private use in the text,
// as a held block is a line of them (markdown.go): an escape is what tells a box from text, and richtext drops it.

const (
	tokCheck    = '\uE002' // a task in the running text, not done
	tokChecked  = '\uE003' // and one done
	tokLook     = '\uE004' // a list's look: tokLook, its number among the page's looks, tokEnd
	tokNumbered = '\uE006' // an item of a numbered list of tasks: tokNumbered, its number in the list, tokEnd
	tokEnd      = '\uE005'
)

// listTokens are the characters that start a token of lists in the text, and listRunes all of its characters:
// a text that has one of them already, as an icon font's letter, is read and written as before, without tokens.
const (
	listTokens = "\uE002\uE003\uE004\uE006"
	listRunes  = "\uE002\uE003\uE004\uE005\uE006"
)

// listMarkStart starts the comment over a list that says its look.
const listMarkStart = textnorm.ListMarkStart

var (
	// listMarkLine is the comment over a list: what is before it (a quote's marks, the list's indent) and the look.
	listMarkLine = regexp.MustCompile(`^([> ]*)<!--celestine:list ([^\s<>]*)-->[ \t]*$`)
	// listLead is what is before a list item's marker: a quote's marks and the indent.
	listLead = regexp.MustCompile(`^[> ]*`)
	// bulletMarker is a list item's bullet, as richtext writes it, with what is before it.
	bulletMarker = regexp.MustCompile(`^([> ]*)[-*+] `)
	// numberedTaskLine is an item of a numbered list with a box.
	numberedTaskLine = regexp.MustCompile(`^\s*(?:>\s*)*\d+[.)]\s+\[.\](?:\s|$)`)
	// refDefX is a link's reference "[x]: …": a page that has one has its "[x]" as a link, not a task.
	refDefX = regexp.MustCompile(`(?mi)^ {0,3}\[x\]:`)
	// safeLook is a look the comment can say as the ID has it: nothing that ends the comment, no spaces.
	safeLook = regexp.MustCompile(`^[^\s<>]*$`)
)

// inlineBoxes are where in line l, of Markdown, a box "[ ]" or "[x]" is a task in the running text: not escaped,
// outside code, not a list item's own box, not in a link's text or a [[link]], after a space, "(" or an emphasis's
// mark or at the start, and before a space, a stop, ")" or an emphasis's mark or at the end. refX says the page
// has a reference "[x]:", so that "[x]" is its link. The importer reads only these as tasks, and the exporter
// writes a task bare only where this finds it, so the two agree.
func inlineBoxes(l string, refX bool) []int {
	if !strings.Contains(l, "[ ]") && !strings.Contains(l, "[x]") {
		return nil
	}
	own := -1 // the item's box, a task line's
	if m := taskBoxLine.FindStringSubmatchIndex(l); m != nil {
		own = m[4] - 1
	}
	var out []int
	code, depth := false, 0
	for i := 0; i < len(l); i++ {
		switch c := l[i]; {
		case c == '`' && (code || !escapedAt(l, i)): // an escaped one opens no code; in code no backslash escapes
			code = !code
		case code:
		case c == '[' && !escapedAt(l, i):
			if i != own && depth == 0 && boxAt(l, i, refX) {
				out = append(out, i)
				i += 2
				continue
			}
			depth++
		case c == ']' && !escapedAt(l, i) && depth > 0:
			depth--
		}
	}
	return out
}

// boxAt says whether l has, at i, a box with room around it to be a task (inlineBoxes).
func boxAt(l string, i int, refX bool) bool {
	box := l[i:min(i+3, len(l))]
	if box != "[ ]" && (box != "[x]" || refX) {
		return false
	}
	if i > 0 && !strings.ContainsRune(" \t(*_~", rune(l[i-1])) {
		return false
	}
	return i+3 == len(l) || strings.ContainsRune(" \t.,;!?)*_~", rune(l[i+3]))
}

// inlineChecksIn puts a token in the place of each box of line l that is a task in the running text.
func inlineChecksIn(l string, refX bool) string {
	at := inlineBoxes(l, refX)
	for k := len(at) - 1; k >= 0; k-- {
		tok := tokCheck
		if l[at[k]+1] == 'x' {
			tok = tokChecked
		}
		l = l[:at[k]] + string(tok) + l[at[k]+3:]
	}
	return l
}

// listMarkAt is the look the comment on line i says of the list under it; false when line i is not one, or what
// is under it is not an item of a list read as one, at the same place.
func listMarkAt(lines []string, i int) (string, bool) {
	m := listMarkLine.FindStringSubmatch(lines[i])
	if m == nil || i+1 >= len(lines) {
		return "", false
	}
	next := lines[i+1]
	if !listItemLine.MatchString(next) || listLead.FindString(next) != m[1] || hrLine.MatchString(next) {
		return "", false
	}
	if kind, _ := rawBlock(lines, i+1, false); kind != "" {
		return "", false
	}
	// as far in as a block of code, after an empty line, it is code (a list's comment is under its item)
	if len(m[1])-len(strings.TrimRight(m[1], " ")) >= 4 && (i == 0 || strings.TrimSpace(lines[i-1]) == "") {
		return "", false
	}
	return m[2], true
}

// withItemTokens puts toks in item line l's text, at its start: after its marker, and after its box if it has one.
func withItemTokens(l, toks string) string {
	if toks == "" {
		return l
	}
	if m := taskBoxLine.FindStringSubmatchIndex(l); m != nil {
		if m[6] == m[7] { // the box ends the line: a space after it, or it is no task's
			return l + " " + toks
		}
		return l[:m[1]] + toks + l[m[1]:]
	}
	if m := listItemLine.FindStringIndex(l); m != nil {
		return l[:m[1]] + toks + l[m[1]:]
	}
	return l
}

// listToken is a token of kind tok and number n.
func listToken(tok rune, n int) string { return string(tok) + strconv.Itoa(n) + string(tokEnd) }

// listTokensIn turns the tokens read from Markdown in t into what they say: the tasks in the running text into
// chips (their boxes again, in code), and a list's look, or its numbered tasks, into the ID of its first item
// without one. looks are the comments' looks, by the numbers in their tokens.
func listTokensIn(t richtext.Text, looks []string) richtext.Text {
	lookAt := map[int]string{} // a look, by the index of the paragraph it is over
	numbered := map[int]bool{} // the items of a numbered list of tasks, by index
	var paras []richtext.Paragraph
	i := 0
	for _, para := range t.Paragraphs() {
		code := para.Style.CodeBlock
		var runs []richtext.Run
		for _, r := range para.Runs {
			if r.Attributes.Attachment != nil || !strings.ContainsAny(r.Text, listTokens) {
				runs = append(runs, r)
				continue
			}
			var sb strings.Builder
			flush := func() {
				if sb.Len() > 0 {
					runs = append(runs, richtext.Run{Text: sb.String(), Attributes: r.Attributes})
					sb.Reset()
				}
			}
			s := r.Text
			for j := 0; j < len(s); {
				c, n := utf8.DecodeRuneInString(s[j:])
				switch c {
				case tokCheck, tokChecked:
					ch := &chip{kind: "check"}
					if c == tokChecked {
						ch.target = "x"
					}
					if code || r.Attributes.Code { // in code, the box it was
						sb.WriteString(ch.markdown())
						break
					}
					flush()
					attrs := r.Attributes
					attrs.Attachment = &richtext.Attachment{Name: ch.label(), Data: ch}
					runs = append(runs, richtext.Run{Text: string(richtext.ObjectReplacement), Attributes: attrs})
				case tokLook, tokNumbered:
					end := strings.IndexRune(s[j+n:], tokEnd)
					if end < 0 {
						sb.WriteRune(c)
						break
					}
					k, err := strconv.Atoi(s[j+n : j+n+end])
					switch {
					case err != nil || code:
					case c == tokLook && k >= 0 && k < len(looks):
						if _, ok := lookAt[i]; !ok {
							lookAt[i] = looks[k]
						}
					case c == tokNumbered:
						numbered[i] = true
					}
					n += end + utf8.RuneLen(tokEnd)
				default:
					sb.WriteRune(c)
				}
				j += n
			}
			flush()
		}
		para.Runs = runs
		paras = append(paras, para)
		i++
	}
	t = richtext.TextOf(paras...)
	if len(lookAt) == 0 && len(numbered) == 0 {
		return t
	}
	lm := listsOf(t)
	ids := map[*listGroup]string{}
	var groups []*listGroup
	say := func(p int, id string) {
		it := lm.all[p]
		if it == nil {
			return
		}
		if _, ok := ids[it.group]; !ok {
			ids[it.group] = id
			groups = append(groups, it.group)
		}
	}
	for _, p := range sortedKeys(lookAt) {
		if id := listIDPrefix + lookAt[p]; func() bool { _, ok := parseListSpec(id); return ok }() {
			say(p, id)
		}
	}
	for _, p := range sortedKeys(numbered) {
		if it := lm.all[p]; it != nil && it.group.kind == richtext.Tasks {
			say(p, listSpec{task: "number"}.id())
		}
	}
	for _, g := range groups {
		for _, at := range g.start {
			if t.ParagraphStyleAt(at).ID == "" {
				id := ids[g]
				t = t.FormatParagraphs(at, at, func(st *richtext.ParagraphStyle) { st.ID = id })
				break
			}
		}
	}
	return t
}

// sortedKeys are the keys of m, least first.
func sortedKeys[V any](m map[int]V) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

// listTokensOut writes t's chips as their Markdown (withoutChips), but a task in the running text, which is a
// token in the chip's place, for richtext to write; and at the end of an item, a token for its list's look (on its
// first item) and its number (on each item of a numbered list of tasks), put after withoutChips, which can make
// lines of an item's text. looks are what the comments say, by the numbers in their tokens; false when t has no
// tokens.
func listTokensOut(t richtext.Text) (richtext.Text, []string, bool) {
	has, hasLook := false, false
	for _, para := range t.Paragraphs() {
		hasLook = hasLook || strings.HasPrefix(para.Style.ID, listIDPrefix)
		for _, r := range para.Runs {
			if strings.ContainsAny(r.Text, listRunes) {
				return withoutChips(t), nil, false
			}
			if att := r.Attributes.Attachment; att != nil {
				if c, ok := att.Data.(*chip); ok && c.kind == "check" {
					has = true
				}
			}
		}
	}
	if !has && !hasLook {
		return withoutChips(t), nil, false
	}
	if has {
		var paras []richtext.Paragraph
		for _, para := range t.Paragraphs() {
			var runs []richtext.Run
			for _, r := range para.Runs {
				if att := r.Attributes.Attachment; att != nil {
					if c, ok := att.Data.(*chip); ok && c.kind == "check" {
						attrs := r.Attributes
						attrs.Attachment = nil
						tok := tokCheck
						if c.target == "x" {
							tok = tokChecked
						}
						runs = append(runs, richtext.Run{Text: string(tok), Attributes: attrs})
						continue
					}
				}
				runs = append(runs, r)
			}
			para.Runs = runs
			paras = append(paras, para)
		}
		t = richtext.TextOf(paras...)
	}
	t = withoutChips(t)
	if !hasLook {
		return t, nil, true
	}
	var paras []richtext.Paragraph
	for _, para := range t.Paragraphs() {
		paras = append(paras, para)
	}
	tail := map[int]string{} // the tokens at the end of a paragraph, by its index
	var looks []string
	lm := listsOf(t)
	seen := map[*listGroup]bool{}
	for _, p := range sortedKeys(lm.all) {
		g := lm.all[p].group
		if seen[g] || !g.has {
			continue
		}
		seen[g] = true
		raw := ""
		for _, q := range g.paras {
			if id := paras[q].Style.ID; strings.HasPrefix(id, listIDPrefix) {
				if _, ok := parseListSpec(id); ok {
					raw = strings.TrimPrefix(id, listIDPrefix)
					break
				}
			}
		}
		if !safeLook.MatchString(raw) || strings.Contains(raw, "--") || strings.HasSuffix(raw, "-") { // said again, safe
			raw = strings.ReplaceAll(strings.TrimPrefix(g.spec.id(), listIDPrefix), "-", "%2D")
		}
		numbered := g.kind == richtext.Tasks && g.spec.task == "number"
		if !(numbered && listIDPrefix+raw == (listSpec{task: "number"}).id()) && !paras[g.paras[0]].Style.CodeBlock {
			tail[g.paras[0]] += listToken(tokLook, len(looks))
			looks = append(looks, raw)
		}
		if numbered {
			for k, q := range g.paras {
				if !paras[q].Style.CodeBlock {
					tail[q] += listToken(tokNumbered, k+1)
				}
			}
		}
	}
	for i, s := range tail {
		paras[i].Runs = append(paras[i].Runs, richtext.Run{Text: s})
	}
	return richtext.TextOf(paras...), looks, true
}

// listMarksOut writes the tokens of lists' looks and numbered tasks in lines, richtext's Markdown: a look as the
// comment over its list's first item, and a numbered task's number in place of its bullet. The tokens are at the
// end of an item's text, which can run over lines: the item's line is the nearest over them that starts an item.
func listMarksOut(lines []string, looks []string) []string {
	out := make([]string, 0, len(lines)+len(looks))
	for _, l := range lines {
		if !strings.ContainsRune(l, tokLook) && !strings.ContainsRune(l, tokNumbered) {
			out = append(out, l)
			continue
		}
		look, num := -1, ""
		var sb strings.Builder
		for j := 0; j < len(l); {
			c, n := utf8.DecodeRuneInString(l[j:])
			if c == tokLook || c == tokNumbered {
				if end := strings.IndexRune(l[j+n:], tokEnd); end >= 0 {
					v := l[j+n : j+n+end]
					if c == tokLook {
						look, _ = strconv.Atoi(v)
					} else {
						num = v
					}
					j += n + end + utf8.RuneLen(tokEnd)
					continue
				}
			}
			sb.WriteString(l[j : j+n])
			j += n
		}
		out = append(out, sb.String())
		at := len(out) - 1
		for at > 0 && !listItemLine.MatchString(out[at]) {
			at--
		}
		if !listItemLine.MatchString(out[at]) {
			continue
		}
		if m := bulletMarker.FindStringSubmatchIndex(out[at]); m != nil && num != "" {
			out[at] = out[at][:m[3]] + num + "." + out[at][m[3]+1:]
		}
		if look >= 0 && look < len(looks) {
			mark := listLead.FindString(out[at]) + listMarkStart + looks[look] + "-->"
			out = append(out[:at], append([]string{mark}, out[at:]...)...)
		}
	}
	return out
}

// checksOut writes the tasks in the running text of line l, of Markdown: a bare box where it is read back as a
// task (inlineBoxes), else escaped, as text. refX says the page has a reference "[x]:".
func checksOut(l string, refX bool) string {
	for {
		i := strings.IndexAny(l, "\uE002\uE003")
		if i < 0 {
			return l
		}
		c, n := utf8.DecodeRuneInString(l[i:])
		bare, esc := "[ ]", `\[ \]`
		if c == tokChecked {
			bare, esc = "[x]", `\[x\]`
		}
		cand := l[:i] + bare + l[i+n:]
		if slices.Contains(inlineBoxes(cand, refX), i) || inCode(l, i) { // or in code, not escaped
			l = cand
		} else {
			l = l[:i] + esc + l[i+n:]
		}
	}
}

// inCode says whether the byte at i of l is in inline code, as inlineBoxes reads it: a backtick opens and closes
// code, but for one escaped outside it, which richtext writes for a backtick of the text (#589).
func inCode(l string, i int) bool {
	code := false
	for k := 0; k < i; k++ {
		if l[k] == '`' && (code || !escapedAt(l, k)) {
			code = !code
		}
	}
	return code
}

// checksBare writes the tasks in the running text of s bare: in code, where nothing is escaped.
func checksBare(s string) string {
	if !strings.ContainsAny(s, "\uE002\uE003") {
		return s
	}
	return strings.NewReplacer(string(tokCheck), "[ ]", string(tokChecked), "[x]").Replace(s)
}

// taskSpot is a task in the running text as a page's Markdown has it: its line, where its box starts in the line
// (-1 when the line is read in another form, its box elsewhere), and whether it is done.
type taskSpot struct {
	line, col int
	done      bool
}

// inlineTaskSpots are the tasks in the running text of a page's Markdown md, in their order: the boxes parseBody
// makes chips of. It reads the lines as parseBody does, passing over what it holds (code, a raw block, a divider,
// a list's look), and a test holds the two to the same tasks (TestInlineTaskSpots). The task view lists them
// (tasksOf, #536).
func inlineTaskSpots(md string) []taskSpot {
	if (!strings.Contains(md, "[ ]") && !strings.Contains(md, "[x]")) || strings.ContainsAny(md, listRunes) {
		return nil // no box, or a text read as before, without tokens: none is a task
	}
	md = strings.ReplaceAll(strings.TrimPrefix(md, "\uFEFF"), "\r\n", "\n")
	lines := strings.Split(md, "\n")
	refX := refDefX.MatchString(md)
	blank := func(i int) bool { return i < 0 || strings.TrimSpace(lines[i]) == "" }
	var out []taskSpot
	afterHeld := false
	for i := 0; i < len(lines); {
		l := lines[i]
		t := strings.TrimSpace(l)
		indent := len(l) - len(strings.TrimLeft(l, " "))
		if indent <= 3 && (strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")) {
			n := 0
			for n < len(t) && t[n] == t[0] {
				n++
			}
			fence, lang := t[:n], strings.TrimSpace(t[n:])
			if !(fence[0] == '`' && strings.Contains(lang, "`")) {
				j := i + 1
				for ; j < len(lines); j++ {
					if c := strings.TrimSpace(lines[j]); strings.HasPrefix(c, fence) && strings.Trim(c, fence[:1]) == "" {
						break
					}
				}
				i, afterHeld = j+1, true
				continue
			}
		}
		if _, ok := listMarkAt(lines, i); ok {
			i++
			continue
		}
		if indent <= 3 {
			if _, n := rawBlockOf(lines, i, blank(i-1), true); n > 0 {
				i, afterHeld = i+n, true
				continue
			}
			if hrLine.MatchString(l) && (blank(i-1) || afterHeld || !strings.Contains(l, "-")) {
				i, afterHeld = i+1, true
				continue
			}
		}
		read := protectInline(l)
		boxes, own := inlineBoxes(read, refX), inlineBoxes(l, refX)
		for k, b := range boxes {
			col := -1
			if len(own) == len(boxes) {
				col = own[k]
			}
			out = append(out, taskSpot{i, col, read[b+1] == 'x'})
		}
		afterHeld = false
		i++
	}
	return out
}

// checkChips are where the tasks in the running text of t are, in their order, out of code, and whether each is done.
func checkChips(t richtext.Text) (at []int, done []bool) {
	for s, para := range t.Paragraphs() {
		if para.Style.CodeBlock {
			continue
		}
		pos := s
		for _, r := range para.Runs {
			if att := r.Attributes.Attachment; att != nil {
				if c, ok := att.Data.(*chip); ok && c.kind == "check" {
					at, done = append(at, pos), append(done, c.target == "x")
				}
			}
			pos += utf8.RuneCountInString(r.Text)
		}
	}
	return at, done
}
