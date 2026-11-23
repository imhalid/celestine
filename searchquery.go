package main

import (
	"regexp"
	"slices"
	"strings"
	"time"
)

// The query language of the search, of the palette, of a dynamic collection and of MCP's search: words
// a page must all have (AND is the default), OR (VEYA) between them for either, -word, NOT (DEĞİL) for
// none, and parentheses. Besides words and "phrases" there are terms that look at the page itself:
// başlık:, koleksiyon:, #etiket, @todo and @done (an open or a ticked task), @today (written today),
// created: and modified: with a day, a range (2026-11-01..2026-11-30) or a side of one (>2026-11-01), and
// field:value or field>value on a property of the page's collection, by its name.
//
// A query is a tree (qnode). The words that every page found must have, those not under an OR or a
// NOT, are kept apart in query.words, .phrases and .title too: the search index looks for pages with
// them before anything is scored (searchindex.go), and nothing else can narrow that way.

type qkind int

const (
	qAnd qkind = iota
	qOr
	qNot
	qWord   // in the title or the text
	qPhrase // the same, whole
	qTitle  // in the title
	qColl   // in a collection whose name has it
	qTag    // a tag of the page
	qTodo   // an open task
	qDone   // a ticked task
	qToday  // made or written today
	qCreated
	qModified
	qProp // a property of the page's collection, or the word as written when it has none
)

// qnode is a part of a query.
type qnode struct {
	kind qkind
	kids []*qnode
	text string // a word, phrase, title or collection folded; a tag's key; a property's name folded
	op   string // a property's or a day's comparison: ":", ">", "<", ">=", "<="
	val  string // what it is compared with, as written
	from time.Time
	to   time.Time // a day's range, [from, to); zero ends are open
	bad  bool      // a day that could not be read: nothing is found
}

// key says what a term is, to drop the same term said twice.
func (n *qnode) key() string {
	return string(rune('A'+n.kind)) + n.text + "\x00" + n.op + "\x00" + n.val
}

// query is what was asked, taken apart: the tree, and the terms every page found has.
type query struct {
	root    *qnode
	words   []string // folded
	phrases []string
	title   []string
	colls   []string
	tags    []string
	hl      []string // the words to mark in a result
	days    bool     // it has a created:, modified: or @today, to be read with the clock (resolve)
}

var queryToken = regexp.MustCompile(`"([^"]*)"|[()]|[^\s()"]+`)

// queryKeyOp is a term with a key: başlık:x, created>2026-11-01, öncelik>=2.
var queryKeyOp = regexp.MustCompile(`^([^:<>=]+)(:>=|:<=|:>|:<|:|>=|<=|>|<|=)(.*)$`)

// qtok is a token of a query: a word as written, or a phrase.
type qtok struct {
	s      string
	phrase bool
}

// parseQuery takes q apart.
func parseQuery(q string) query {
	var toks []qtok
	for _, m := range queryToken.FindAllStringSubmatch(q, -1) {
		switch {
		case strings.HasPrefix(m[0], `"`):
			if t := strings.TrimSpace(m[1]); t != "" {
				// a key's value in quotes, as durum:"Devam ediyor", is one word with it
				if n := len(toks); n > 0 && !toks[n-1].phrase && strings.ContainsAny(toks[n-1].s[len(toks[n-1].s)-1:], ":<>=") {
					toks[n-1].s += t
					continue
				}
				toks = append(toks, qtok{t, true})
			}
		default:
			toks = append(toks, qtok{m[0], false})
		}
	}
	p := &qparser{toks: toks}
	var out query
	out.root = p.or()
	for p.i < len(p.toks) { // a ")" too many: the rest is read as more of the same
		p.i++
		if n := p.or(); n != nil {
			out.root = andOf(out.root, n)
		}
	}
	out.root = simplify(out.root)
	out.collect(out.root, true, false)
	return out
}

// andOf is a and b, either of which may be nil.
func andOf(a, b *qnode) *qnode {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	}
	return &qnode{kind: qAnd, kids: []*qnode{a, b}}
}

// simplify flattens an AND in an AND and an OR in an OR, drops a term said twice in one, and takes a group of
// one for the one.
func simplify(n *qnode) *qnode {
	if n == nil || (n.kind != qAnd && n.kind != qOr && n.kind != qNot) {
		return n
	}
	var kids []*qnode
	seen := map[string]bool{}
	for _, k := range n.kids {
		k = simplify(k)
		if k == nil {
			continue
		}
		if k.kind == n.kind && n.kind != qNot {
			kids = append(kids, k.kids...)
			continue
		}
		kids = append(kids, k)
	}
	n.kids = kids[:0:0]
	for _, k := range kids {
		if k.kind >= qWord {
			if seen[k.key()] {
				continue
			}
			seen[k.key()] = true
		}
		n.kids = append(n.kids, k)
	}
	switch {
	case len(n.kids) == 0:
		return nil
	case len(n.kids) == 1 && n.kind != qNot:
		return n.kids[0]
	}
	return n
}

// collect fills the lists: the terms every page found must have (need: not under an OR or a NOT), and the
// words to mark (not under a NOT).
func (q *query) collect(n *qnode, need, neg bool) {
	if n == nil {
		return
	}
	add := func(list *[]string, w string) {
		if !slices.Contains(*list, w) {
			*list = append(*list, w)
		}
	}
	switch n.kind {
	case qAnd:
		for _, k := range n.kids {
			q.collect(k, need, neg)
		}
		return
	case qOr:
		for _, k := range n.kids {
			q.collect(k, false, neg)
		}
		return
	case qNot:
		for _, k := range n.kids {
			q.collect(k, false, !neg)
		}
		return
	case qToday, qCreated, qModified:
		q.days = true
	}
	if !neg && (n.kind == qWord || n.kind == qPhrase || n.kind == qTitle) {
		add(&q.hl, n.text)
	}
	if !need {
		return
	}
	switch n.kind {
	case qWord:
		add(&q.words, n.text)
	case qPhrase:
		add(&q.phrases, n.text)
	case qTitle:
		add(&q.title, n.text)
	case qColl:
		add(&q.colls, n.text)
	case qTag:
		add(&q.tags, n.text)
	}
}

// qparser reads the tokens of a query into a tree: or := and (OR and)*, and := unary+, unary := NOT unary |
// -unary | ( or ) | term.
type qparser struct {
	toks []qtok
	i    int
}

func (p *qparser) peek() (qtok, bool) {
	if p.i < len(p.toks) {
		return p.toks[p.i], true
	}
	return qtok{}, false
}

// isWord reports whether the token is one of the words of the language, written in capitals.
func (t qtok) isWord(ws ...string) bool { return !t.phrase && slices.Contains(ws, t.s) }

func (p *qparser) or() *qnode {
	var kids []*qnode
	for {
		if n := p.and(); n != nil {
			kids = append(kids, n)
		}
		t, ok := p.peek()
		if !ok || !t.isWord("OR", "VEYA", "|") {
			break
		}
		p.i++
	}
	switch len(kids) {
	case 0:
		return nil
	case 1:
		return kids[0]
	}
	return &qnode{kind: qOr, kids: kids}
}

func (p *qparser) and() *qnode {
	var kids []*qnode
	for {
		t, ok := p.peek()
		if !ok || t.isWord("OR", "VEYA", "|") || t.isWord(")") {
			break
		}
		if t.isWord("AND", "VE") {
			p.i++
			continue
		}
		if n := p.unary(); n != nil {
			kids = append(kids, n)
		}
	}
	switch len(kids) {
	case 0:
		return nil
	case 1:
		return kids[0]
	}
	return &qnode{kind: qAnd, kids: kids}
}

func (p *qparser) unary() *qnode {
	t, ok := p.peek()
	if !ok {
		return nil
	}
	p.i++
	switch {
	case t.isWord("NOT", "DEĞİL", "DEGIL", "-"):
		if n := p.unary(); n != nil {
			return &qnode{kind: qNot, kids: []*qnode{n}}
		}
		return nil
	case t.isWord("("):
		n := p.or()
		if t, ok := p.peek(); ok && t.isWord(")") {
			p.i++
		}
		return n
	case t.phrase:
		return &qnode{kind: qPhrase, text: foldText(t.s)}
	case strings.HasPrefix(t.s, "-") && len(t.s) > 1:
		if n := term(t.s[1:]); n != nil {
			return &qnode{kind: qNot, kids: []*qnode{n}}
		}
		return nil
	}
	return term(t.s)
}

// term is a word of a query: a filter, or a word to find.
func term(w string) *qnode {
	if strings.HasPrefix(w, "#") && len(w) > 1 {
		return &qnode{kind: qTag, text: tagKey(w[1:])}
	}
	if strings.HasPrefix(w, "@") {
		switch foldText(w[1:]) {
		case "todo", "yapilacak", "acik":
			return &qnode{kind: qTodo}
		case "done", "bitti", "tamam", "yapildi":
			return &qnode{kind: qDone}
		case "today", "bugun":
			return &qnode{kind: qToday}
		}
	}
	if m := queryKeyOp.FindStringSubmatch(w); m != nil && m[3] != "" {
		k, op, v := foldText(m[1]), strings.TrimPrefix(m[2], ":"), m[3]
		if op == "" || op == "=" {
			op = ":"
		}
		switch k {
		case "baslik", "title":
			if op == ":" {
				return &qnode{kind: qTitle, text: foldText(v)}
			}
		case "koleksiyon", "kol", "in":
			if op == ":" {
				return &qnode{kind: qColl, text: foldText(v)}
			}
		case "etiket", "tag":
			if op == ":" {
				return &qnode{kind: qTag, text: tagKey(v)}
			}
		case "created", "olusturma", "olusturuldu":
			return &qnode{kind: qCreated, op: op, val: v}
		case "modified", "degisti", "degistirildi", "duzenlendi", "yazildi":
			return &qnode{kind: qModified, op: op, val: v}
		default:
			return &qnode{kind: qProp, text: k, op: op, val: v}
		}
	}
	if w = foldText(w); w == "" {
		return nil
	}
	return &qnode{kind: qWord, text: w}
}

// marks are the words to mark in a result: the words, the phrases, the title's.
func (q query) marks() []string { return q.hl }

func (q query) empty() bool { return q.root == nil }

// queryWords are the words of a query every page found has, folded, once each.
func queryWords(q string) []string { return parseQuery(q).words }

// resolve reads the days of the query's created:, modified: and @today with the clock: "today" is now's.
func (q query) resolve(now time.Time) {
	if !q.days {
		return
	}
	var walk func(n *qnode)
	walk = func(n *qnode) {
		switch n.kind {
		case qToday:
			n.from = startOfDay(now)
			n.to = n.from.AddDate(0, 0, 1)
		case qCreated, qModified:
			n.from, n.to, n.bad = dayRange(n.op, n.val, now)
		}
		for _, k := range n.kids {
			walk(k)
		}
	}
	walk(q.root)
}

// dayRange is the days a created: or modified: asks for, [from, to): a day, from..to (either end may be left
// out), or a side of a day with op.
func dayRange(op, v string, now time.Time) (from, to time.Time, bad bool) {
	day := func(s string) (time.Time, bool) {
		if d, err := time.ParseInLocation("2006-01-02", s, now.Location()); err == nil {
			return d, true
		}
		if d, ok := parseNaturalDay(s, now); ok {
			return d, true
		}
		return time.Time{}, false
	}
	if a, b, ok := strings.Cut(v, ".."); ok && op == ":" {
		if a != "" {
			if from, ok = day(a); !ok {
				return from, to, true
			}
		}
		if b != "" {
			d, ok := day(b)
			if !ok {
				return from, to, true
			}
			to = d.AddDate(0, 0, 1)
		}
		return from, to, false
	}
	d, ok := day(v)
	if !ok {
		return from, to, true
	}
	switch op {
	case ">":
		return d.AddDate(0, 0, 1), time.Time{}, false
	case ">=":
		return d, time.Time{}, false
	case "<":
		return time.Time{}, d, false
	case "<=":
		return time.Time{}, d.AddDate(0, 0, 1), false
	}
	return d, d.AddDate(0, 0, 1), false
}

// inRange reports whether t is in [from, to), an end left zero being open.
func inRange(t, from, to time.Time) bool {
	return (from.IsZero() || !t.Before(from)) && (to.IsZero() || t.Before(to))
}

// qdoc is a page as a query looks at it.
type qdoc struct {
	page   *Page
	coll   *Collection
	ftitle string
	ftext  string
	fcoll  string
}

// score reports whether page d meets n, and how well: a word in the title is worth more than in the text, and
// the more times it is in the text the more.
func (a *app) score(n *qnode, d qdoc) (int, bool) {
	switch n.kind {
	case qAnd:
		s := 0
		for _, k := range n.kids {
			ks, ok := a.score(k, d)
			if !ok {
				return 0, false
			}
			s += ks
		}
		return s, true
	case qOr:
		s, any := 0, false
		for _, k := range n.kids {
			if ks, ok := a.score(k, d); ok {
				s, any = s+ks, true
			}
		}
		return s, any
	case qNot:
		_, ok := a.score(n.kids[0], d)
		return 0, !ok
	case qWord:
		return wordScore(n.text, d, 50, 2)
	case qPhrase:
		return wordScore(n.text, d, 60, 3)
	case qTitle:
		return 60, strings.Contains(d.ftitle, n.text)
	case qColl:
		return 0, strings.Contains(d.fcoll, n.text)
	case qTag:
		// the page's own tags, a few: the pages of the tag were gone through for every page asked about (#195)
		if e := a.links.pages[d.page]; e != nil {
			return 20, slices.ContainsFunc(e.tags, func(name string) bool { return tagKey(name) == n.text })
		}
		return 20, false
	case qTodo, qDone:
		if e := a.links.pages[d.page]; e != nil {
			return 0, slices.ContainsFunc(e.tasks, func(t taskItem) bool { return t.done == (n.kind == qDone) })
		}
		return 0, false
	case qToday:
		return 0, inRange(d.page.Created, n.from, n.to) || inRange(d.page.Modified, n.from, n.to)
	case qCreated:
		return 0, !n.bad && inRange(d.page.Created, n.from, n.to)
	case qModified:
		return 0, !n.bad && inRange(d.page.Modified, n.from, n.to)
	case qProp:
		if pr := propFolded(d.coll, n.text); pr != nil {
			return 20, propMatches(pr, d.page.Value(pr), n.op, n.val)
		}
		// no such property here: it is a word, as written
		return wordScore(foldText(n.text+":"+n.val), d, 50, 2)
	}
	return 0, false
}

// wordScore is a word's or a phrase's: so much in the title, so much a time in the text, up to sxCountCap times.
func wordScore(w string, d qdoc, inTitle, each int) (int, bool) {
	t, c := strings.Contains(d.ftitle, w), countCap(d.ftext, w, sxCountCap)
	if !t && c == 0 {
		return 0, false
	}
	s := c * each
	if t {
		s += inTitle
	}
	return s, true
}

// propFolded is the property of c whose name, folded, is name, or nil.
func propFolded(c *Collection, name string) *Property {
	if c == nil {
		return nil
	}
	for i := range c.Properties {
		if pr := &c.Properties[i]; foldText(pr.Name) == name || pr.ID == name {
			return pr
		}
	}
	return nil
}

// propMatches reports whether a page's value val of property pr is as op and want ask: ":" has it (a text
// contains it, a tag is it, a choice, number or day is it), the others compare as a view's filter sorts
// (CompareValues): a choice by its options' order, a number as a number, a day as a day.
func propMatches(pr *Property, val, op, want string) bool {
	if op == ":" {
		switch pr.Type {
		case PropTags:
			return slices.ContainsFunc(SplitTags(val), func(t string) bool { return foldText(t) == foldText(want) })
		case PropNumber:
			return val != "" && CompareValues(pr, val, want) == 0
		case PropChoice, PropDate, PropPage, PropFile:
			return foldText(val) == foldText(want)
		}
		return strings.Contains(foldText(val), foldText(want))
	}
	if pr.Type == PropChoice { // the option as it is written, from any case and letters; none is no order
		i := slices.IndexFunc(pr.Options, func(o string) bool { return foldText(o) == foldText(want) })
		if i < 0 {
			return false
		}
		want = pr.Options[i]
	}
	if val == "" {
		return false
	}
	cmp := CompareValues(pr, val, want)
	switch op {
	case ">":
		return cmp > 0
	case ">=":
		return cmp >= 0
	case "<":
		return cmp < 0
	case "<=":
		return cmp <= 0
	}
	return false
}
