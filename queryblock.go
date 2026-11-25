package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Query blocks: a search kept in a page, its results as the library is now. It is a chip of its own kind
// (chips.go) laid out as a block, as an embed is (embed.go): the query at its head, and a row for each page
// it finds under it; a row opens its page, the head opens the search with the query. The page keeps the
// query alone (bodyObj.Q); its Markdown is a fenced block:
//
//	```query
//	#proje @todo
//	```

const (
	queryLang  = "query"
	queryHeadH = 36 // the head: the query, and how many pages it finds
	queryRowH  = 26
	queryRows  = 10 // rows shown at most; a last one says how many more there are
	queryTTL   = 3 * time.Second
)

// queryResult is what a query found, kept for a few seconds: the block is drawn every frame.
type queryResult struct {
	hits  []searchHit
	at    time.Time
	built time.Time // the index of links it was found with
}

// docQueries tracks the query block attachments in one document so their heights stay current.
type docQueries struct {
	ver   uint64
	built time.Time
	at    time.Time
	atts  []*richtext.Attachment
}

// docQueryAtts collects all query-block attachments in t.
func docQueryAtts(t richtext.Text) []*richtext.Attachment {
	var out []*richtext.Attachment
	for _, r := range t.Runs(0, t.Len()) {
		if att := r.Attributes.Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok && ch.kind == "query" {
				out = append(out, att)
			}
		}
	}
	return out
}

// refreshQueries updates query block attachment heights for the active document when the link
// index or cache TTL advances, bumping queryVer so richtext re-lays out changed blocks.
func (a *app) refreshQueries() {
	if a.pane().doc == nil {
		return
	}
	if a.queryDocs == nil {
		a.queryDocs = map[*richtext.Document]*docQueries{}
	}
	now := a.clock()
	dq := a.queryDocs[a.pane().doc]
	if dq == nil {
		dq = &docQueries{
			ver:   a.pane().doc.Version(),
			built: a.links.built,
			at:    now,
			atts:  docQueryAtts(a.pane().doc.Text()),
		}
		a.queryDocs[a.pane().doc] = dq
	} else {
		if len(dq.atts) == 0 && dq.built.Equal(a.links.built) {
			return
		}
		if !dq.built.Equal(a.links.built) || now.Sub(dq.at) >= queryTTL {
			if v := a.pane().doc.Version(); dq.ver != v {
				dq.atts = docQueryAtts(a.pane().doc.Text())
				dq.ver = v
			}
			dq.built = a.links.built
			dq.at = now
		} else {
			return
		}
	}
	changed := false
	for _, att := range dq.atts {
		ch, _ := att.Data.(*chip)
		if ch == nil || ch.kind != "query" {
			continue
		}
		if want := queryHeight(len(a.queryHits(ch.target))); att.Height != want {
			att.Height = want
			changed = true
		}
	}
	if changed {
		a.queryVer++
		sizedGen++
	}
}

// queryHits are the pages query q finds, from the cache while the library's index is the same and it is fresh.
func (a *app) queryHits(q string) []searchHit {
	now := a.clock()
	if r, ok := a.queryCache[q]; ok && r.built.Equal(a.links.built) && now.Sub(r.at) < queryTTL {
		return r.hits
	}
	if a.queryCache == nil || len(a.queryCache) > 100 {
		a.queryCache = map[string]queryResult{}
	}
	hits := a.searchPages(q, queryRows+1, false)
	if strings.TrimSpace(q) == "" {
		hits = nil
	}
	a.queryCache[q] = queryResult{hits, now, a.links.built}
	return hits
}

// queryHeight is the height of a query's block for n pages found: a row each, up to queryRows and a row for the
// rest, and a row that says there are none.
func queryHeight(n int) float32 {
	rows := max(1, min(n, queryRows+1))
	return 6 + queryHeadH + float32(rows)*queryRowH + 10
}

// queryAttachment is the block of a query, sized to its results and updated as they change.
func (a *app) queryAttachment(ch *chip) *richtext.Attachment {
	n := len(a.queryHits(ch.target))
	var att *richtext.Attachment
	att = &richtext.Attachment{Width: a.blockWidth(), Height: queryHeight(n), Name: locales.QueryNamed(ch.target), Data: ch,
		Draw: func(p *ui.Painter, r ui.Rect) { a.drawQuery(p, r, ch, att) }}
	return att
}

// queryRowAt is which row of the block drawn at r the height y is in: -1 the head, or past the rows.
func queryRowAt(r ui.Rect, y float32) int {
	y -= r.Y + queryHeadH
	if y < 0 {
		return -1
	}
	return int(y / queryRowH)
}

// clickQuery opens the page of the row clicked, or the search with the query when the head is.
func (a *app) clickQuery(pn *paneState, h *chipHit, y float32) {
	hits := a.queryHits(h.ch.target)
	if i := queryRowAt(h.r, y); i >= 0 && i < min(len(hits), queryRows) {
		a.openIn(pn, hits[i].page)
		return
	}
	a.openSearchIn(pn, h.ch.target)
}

// drawQuery draws the block: the query and how many it finds, and a row for each page.
func (a *app) drawQuery(p *ui.Painter, r ui.Rect, ch *chip, att *richtext.Attachment) {
	t := ryugo.Painting(p)
	cl := t.Colors
	r = ui.Rect{X: r.X, Y: r.Y + 3, W: r.W, H: r.H - 6}
	a.chipHits = append(a.chipHits, chipHit{r, ch})
	hover := a.hoverCh == ch
	hits := a.queryHits(ch.target)
	if att != nil {
		if want := queryHeight(len(hits)); att.Height != want {
			att.Height = want
			a.queryVer++
			sizedGen++
			if a.frame != nil {
				a.frame()
			}
		}
	}
	p.Fill(r, cl.WellBorder, rad(12))
	p.Fill(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}, cl.Well, rad(11))
	p.Clip(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}, rad(11), func() {
		size := a.fontSize() * 0.88
		count := locales.CommonPageCount(len(hits))
		if len(hits) > queryRows {
			count = locales.QueryPagesMore(queryRows)
		}
		cw, _ := p.MeasureText(0, ui.Span{Text: count, Size: 12})
		p.RichText(r.X+r.W-cw-14, r.Y+12, cw+2, ui.Span{Text: count, Size: 12, Color: cl.Muted})
		p.RichText(r.X+14, r.Y+10, r.W-cw-40,
			ui.Span{Text: "⌕ ", Size: size, Color: cl.Muted},
			ui.Span{Text: ch.target, Size: size, Weight: 650, Color: map[bool]ui.Color{true: cl.Accent, false: cl.Text}[hover]})
		p.Fill(ui.Rect{X: r.X + 12, Y: r.Y + queryHeadH - 3, W: r.W - 24, H: 1}, cl.Hairline, 0)
		y := r.Y + queryHeadH
		if len(hits) == 0 {
			p.RichText(r.X+16, y+5, r.W-32, ui.Span{Text: locales.CommonNoPageMatches(), Size: size, Color: cl.Muted})
			return
		}
		for i, h := range hits {
			if i == queryRows {
				p.RichText(r.X+16, y+5, r.W-32, ui.Span{Text: locales.QueryAndMoreOpenSearch(), Size: 12, Color: cl.Muted})
				break
			}
			where := h.page.Coll.Name
			if crumb := Parents(h.page); crumb != "" {
				where += " › " + crumb
			}
			ww, _ := p.MeasureText(0, ui.Span{Text: where, Size: 12})
			ww = min(ww, r.W*0.4)
			p.RichText(r.X+16, y+5, r.W-ww-48, ui.Span{Text: "▫ ", Size: size, Color: cl.Muted}, ui.Span{Text: h.page.Title, Size: size, Weight: 560, Color: cl.Text})
			p.RichText(r.X+r.W-ww-16, y+6, ww+2, ui.Span{Text: where, Size: 12, Color: cl.Muted})
			y += queryRowH
		}
	})
}

// queriesIn turns the fenced ```query blocks of t into query blocks.
func queriesIn(t richtext.Text) richtext.Text {
	ps := starts(t)
	for k := len(ps) - 1; k >= 0; k-- {
		s, e := t.ParagraphRange(ps[k])
		if st := t.ParagraphStyleAt(s); !st.CodeBlock || st.ID != codeIDPrefix+queryLang {
			continue
		}
		lines := []string{t.Slice(s, e).String()}
		for j := k + 1; j < len(ps); j++ { // the block's other lines, after its first
			s2, e2 := t.ParagraphRange(ps[j])
			if st2 := t.ParagraphStyleAt(s2); !st2.CodeBlock || st2.ID != "" {
				break
			}
			lines = append(lines, t.Slice(s2, e2).String())
			e = e2
		}
		q := strings.TrimSpace(strings.Join(strings.Fields(strings.Join(lines, " ")), " "))
		attrs := richtext.Attributes{Attachment: &richtext.Attachment{Name: locales.CommonQuery(), Data: &chip{kind: "query", target: q}}}
		t = t.ReplaceString(s, e, string(richtext.ObjectReplacement), attrs)
		t = t.FormatParagraphs(s, s, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{} })
	}
	return t
}

// insertQuery takes the "/…" away and puts the block of query q in.
func (a *app) insertQuery(start int, q string) {
	doc := a.pane().doc
	s := &a.slash
	_, caret := doc.Selection()
	doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(start, max(caret, start), richtext.Text{})
		ed.Select(start, start)
	})
	s.open = false
	a.putBlock(a.queryAttachment(&chip{kind: "query", target: strings.TrimSpace(q)}))
	delete(a.queryDocs, doc)
}

// queryItems are the lines of the / menu's Sorgu: the query typed, to be put in, and how many it finds now.
func (a *app) queryItems(q string) []menuItem {
	start := a.slash.start
	tq := strings.TrimSpace(q)
	if tq == "" {
		return nil
	}
	n := len(a.searchList(tq, 0, false))
	return []menuItem{{title: tq, hint: locales.CommonPageCount(n), glyph: iconSearch, run: func() { a.insertQuery(start, tq) }}}
}
