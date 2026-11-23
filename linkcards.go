package main

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Links of the web, shown (#495): each with its site's icon before it in the text, room for which the span takes
// (Span.PaddingStart); a card with its page's picture and title when the pointer rests on it; the same in the bar's
// link field while an address is typed in. What a page says of itself is asked for once and kept for the session.
// Settings.NoLinkPreviews turns it all off: no site is asked for anything.

// linkInfo is what is known of a link's page.
type linkInfo struct {
	done   bool             // the page answered, or did not
	meta   library.LinkMeta // its title, picture and icon
	image  ui.ImageSource   // its picture, once read
	imaged bool             // its picture was asked for
}

// webLinks is what the app knows of the web pages its links go to, and the link the pointer rests on.
type webLinks struct {
	pages map[string]*linkInfo      // by address
	icons map[string]ui.ImageSource // by the icon's address; nil when it could not be read
	asked map[string]bool           // the icons asked for
	// the link under the pointer, since when, where it is drawn and where in the text it is
	hover    string
	hoverAt  time.Time
	hoverR   ui.Rect
	hoverPos int
	// the card is kept a moment after the pointer left the link, for it to get onto the card's buttons (#509), and
	// as long as it is on it
	leftAt time.Time
	onCard bool
}

var (
	// linkPreviews is whether the pages of web links are asked for and shown on their cards (Settings.NoLinkPreviews
	// off). Set each frame.
	linkPreviews = true
	// linkIcons is whether web links have their site's icon before them in the text (previews on, and
	// Settings.NoLinkIcons off, #509): the editor's spans leave the icon's room by it (codecolors.go, caret.go).
	linkIcons = true
	// linkFetch is whether pages are asked for: never in a test, which is not to reach the web, but one of its
	// own that sets it.
	linkFetch = !testing.Testing()
	// linkHoverWait is how long the pointer rests on a link before its card shows.
	linkHoverWait = 450 * time.Millisecond
	// linkLeaveWait is how long the card stays once the pointer left the link and is not on the card.
	linkLeaveWait = 300 * time.Millisecond
	// linkWait is how long a page or a picture of a link is waited for, from when it is asked for.
	linkWait = 15 * time.Second
)

// linkSlots are the pages and pictures of links asked for at once: a page of many links does not ask every site
// together (#559).
var linkSlots = make(chan struct{}, 4)

// fetchLink runs fetch when a slot is free, with linkWait from then: the time waited for the slot is not the fetch's,
// or a link behind others would run out before it is asked for, and be remembered as failed for the session.
func fetchLink(fetch func(ctx context.Context)) {
	linkSlots <- struct{}{}
	defer func() { <-linkSlots }()
	ctx, cancel := context.WithTimeout(context.Background(), linkWait)
	defer cancel()
	fetch(ctx)
}

var iconUnlink = icon(`<path d="M10 14a3.5 3.5 0 0 0 5 0l3-3a3.5 3.5 0 0 0-5-5l-1 1M14 10a3.5 3.5 0 0 0-5 0l-3 3a3.5 3.5 0 0 0 5 5l1-1M4 4l16 16"/>`)

var iconGlobe = icon(`<circle cx="12" cy="12" r="8.5"/><path d="M3.5 12h17M12 3.5c2.4 2.3 3.6 5.2 3.6 8.5s-1.2 6.2-3.6 8.5c-2.4-2.3-3.6-5.2-3.6-8.5s1.2-6.2 3.6-8.5z"/>`)

// webLink reports whether address goes to a web page.
func webLink(address string) bool {
	return strings.HasPrefix(address, "https://") || strings.HasPrefix(address, "http://")
}

// linkIconPad is the room a link to address leaves at its left for its icon, in text of size: none but for the web.
func linkIconPad(address string, size float32) float32 {
	if !linkIcons || !webLink(address) {
		return 0
	}
	return size * 1.2
}

// linkHost is the site of address, without its www.
func linkHost(address string) string {
	u, err := url.Parse(address)
	if err != nil {
		return address
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}

// linkPage is what is known of the page at address, asking for it the first time; nil when links are not shown or
// address is not the web's. The page's icon is asked for with it, and its picture when withImage says so.
func (a *app) linkPage(address string, withImage bool) *linkInfo {
	if !linkPreviews || !webLink(address) {
		return nil
	}
	w := &a.web
	info := w.pages[address]
	if info == nil {
		if w.pages == nil {
			w.pages = map[string]*linkInfo{}
		}
		info = &linkInfo{}
		w.pages[address] = info
		if !linkFetch {
			info.done = true
			return info
		}
		go func() {
			defer a.guard()
			var m library.LinkMeta
			fetchLink(func(ctx context.Context) { m, _ = library.FetchLinkMeta(ctx, address) })
			a.do(func() error {
				info.done, info.meta = true, m
				return nil
			})
		}()
	}
	if info.done && info.meta.Icon != "" {
		a.linkIcon(info.meta.Icon)
	}
	if withImage && info.done && info.meta.Image != "" && !info.imaged && linkFetch {
		info.imaged = true
		src := info.meta.Image
		go func() {
			defer a.guard()
			var img ui.ImageSource
			fetchLink(func(ctx context.Context) {
				if data, err := library.FetchLinkImage(ctx, src); err == nil {
					img, _ = ui.DecodeImage(data)
				}
			})
			a.do(func() error {
				info.image = img
				return nil
			})
		}()
	}
	return info
}

// linkIcon is the icon at address, asking for it the first time; nil until it is read, or when it cannot be.
func (a *app) linkIcon(address string) ui.ImageSource {
	w := &a.web
	if w.asked[address] {
		return w.icons[address]
	}
	if w.asked == nil {
		w.asked, w.icons = map[string]bool{}, map[string]ui.ImageSource{}
	}
	w.asked[address] = true
	if !linkFetch {
		return nil
	}
	go func() {
		defer a.guard()
		var img ui.ImageSource
		fetchLink(func(ctx context.Context) {
			if data, err := library.FetchLinkImage(ctx, address); err == nil {
				img, _ = ui.DecodeImage(data)
			}
		})
		a.do(func() error {
			w.icons[address] = img
			return nil
		})
	}()
	return nil
}

// siteIcon is the icon of the site of the link to address, as far as it is known: nil until it is read.
func (a *app) siteIcon(address string) ui.ImageSource {
	info := a.linkPage(address, false)
	if info == nil || !info.done || info.meta.Icon == "" {
		return nil
	}
	return a.web.icons[info.meta.Icon]
}

// drawLinkIcon draws the icon of the link to address in the room box: its site's, or a globe while there is none.
func (a *app) drawLinkIcon(p *ui.Painter, box ui.Rect, address string, muted ui.Color) {
	if img := a.siteIcon(address); img != nil {
		p.Clip(box, rad(3), func() { p.Image(img, box, ui.Contain) })
		return
	}
	p.Icon(iconGlobe, box, muted)
}

// linkIconBox is where the icon of a link goes, in the room before the line's piece of it drawn in rc, for text of
// size: at its left, or at its right when the link's text runs right to left, as Span.PaddingStart leaves the room.
func linkIconBox(rc ui.Rect, size float32, rtl bool) ui.Rect {
	pad := size * 1.2
	s := size * 0.9
	if rtl {
		return ui.Rect{X: rc.X + rc.W + (pad-s)/2 + size*0.08, Y: rc.Y + (rc.H-s)/2, W: s, H: s}
	}
	return ui.Rect{X: rc.X - pad + (pad-s)/2 - size*0.08, Y: rc.Y + (rc.H-s)/2, W: s, H: s}
}

// startsRTL reports whether text's first strong letter is of a script written right to left, which is the
// direction its run is laid out in.
func startsRTL(text string) bool {
	for _, r := range text {
		if unicode.In(r, unicode.Hebrew, unicode.Arabic, unicode.Syriac, unicode.Thaana, unicode.Nko) {
			return true
		}
		if unicode.IsLetter(r) {
			return false
		}
	}
	return false
}

// hoverLink keeps the link the pointer is on, (x, y) of the window, and since when: its card shows after
// linkHoverWait. Off the link, the card shown stays a moment, and as long as the pointer is on it.
func (a *app) hoverLink(c *ui.Context, x, y float32, over bool) {
	w := &a.web
	address, r, pos := "", ui.Rect{}, 0
	if over {
		for _, h := range a.linkSeen {
			if x >= h.r.X && x < h.r.X+h.r.W && y >= h.r.Y && y < h.r.Y+h.r.H {
				address, r, pos = h.url, h.r, h.pos
				break
			}
		}
	}
	now := c.Now()
	busy := a.sel.link || a.edPressed
	if address == "" || busy {
		if w.hover != "" && !busy && now.Sub(w.hoverAt) >= linkHoverWait {
			if w.leftAt.IsZero() || w.onCard {
				w.leftAt = now
			}
			if wait := linkLeaveWait - now.Sub(w.leftAt); wait > 0 {
				c.After(wait)
				return
			}
		}
		w.hover, w.onCard = "", false
		return
	}
	w.leftAt = time.Time{}
	if address != w.hover || pos != w.hoverPos {
		w.hover, w.hoverPos, w.hoverAt = address, pos, now
	}
	w.hoverR = r
	if wait := linkHoverWait - now.Sub(w.hoverAt); wait > 0 {
		c.After(wait)
	}
}

// linkCardView is the card of the link the pointer rests on, under the link: a web page's picture, title and site
// as they come, and at its foot the address with Edit and Remove (#509), for every link.
func (a *app) linkCardView(c *ui.Context) {
	w := &a.web
	if w.hover == "" || c.Now().Sub(w.hoverAt) < linkHoverWait || a.modal() || a.sel.link || a.doc == nil {
		w.onCard = false
		return
	}
	address := w.hover
	info := a.linkPage(address, true)
	rich := info != nil && info.done && (info.meta.Title != "" || info.image != nil)
	ww, wh := c.Size()
	const cardW = 300
	left := max(8, min(w.hoverR.X, ww-cardW-8))
	top := w.hoverR.Y + w.hoverR.H + 6
	cardH := float32(linkFootH)
	if rich {
		cardH += linkPreviewH(info, cardW) + 1
	}
	if top+cardH > wh-8 { // no room under the link: over it
		top = w.hoverR.Y - cardH - 6
	}
	tth := ryugo.From(c)
	cl := tth.Colors
	ui.Overlay(c, func() {
		card := ui.Column(c.Key("link-card")).Absolute().Left(left).Top(top).Width(cardW).Radius(rad(12)).Clip().
			Background(cl.Card).Border(1, cl.Hairline).KeepFocus().Label(locales.SelectionLinkCard()).Transition(rowEnter)
		tth.Shadows.ShadowPop.Apply(card)
		card.Children(func() {
			if rich {
				a.linkPreview(c, address, info, cardW)
				ui.Box(c.Key("link-card-line")).Height(1).Background(cl.Hairline)
			}
			a.linkCardFoot(c, address)
		})
		w.onCard = card.Hovered()
	})
}

// linkFootH is the height of a link card's foot.
const linkFootH = 38

// linkCardFoot is the foot of a link's card: where it goes, and the buttons that change it or take it away.
func (a *app) linkCardFoot(c *ui.Context, address string) {
	cl := ryugo.From(c).Colors
	ui.Row(c.Key("link-foot")).Height(linkFootH).Padding(0, 5, 0, 12).Gap(2).AlignItems(ui.Center).Children(func() {
		ui.Box(c.Key("link-foot-icon")).Width(13).Height(13).Draw(func(p *ui.Painter, r ui.Rect) {
			if webLink(address) && linkPreviews {
				a.drawLinkIcon(p, r, address, cl.Muted)
				return
			}
			p.Icon(iconLink, r, cl.Muted)
		})
		ui.Text(c.Key("link-foot-addr"), shownAddress(address)).FontSize(12).TextColor(cl.Muted).MaxLines(1).Grow(1).MinWidth(0).
			Padding(0, 6, 0, 7)
		btn := func(key string, glyph *ui.SVG, word, label string, danger bool, fn func()) {
			e := ui.Row(c.Key(key)).Height(26).Padding(0, 8, 0, 7).Gap(5).Radius(rad(7)).AlignItems(ui.Center).Label(label).
				Role(ui.RoleButton).Cursor(ui.CursorPointer).Transition(ui.ElementTransition{Duration: 100 * time.Millisecond, Colors: true})
			ryugo.Tooltip(c, e, label)
			col := cl.Text2
			if e.Hovered() {
				e.Background(cl.Fill)
				if danger {
					col = cl.Up
					e.Background(cl.Up.Alpha(0.1))
				}
			}
			e.Children(func() {
				ui.Icon(c, glyph).Width(13).Height(13).TextColor(col)
				ui.Text(c, word).FontSize(12).FontWeight(560).TextColor(col)
			})
			if e.Clicked() {
				fn()
			}
		}
		btn("link-edit", iconEdit, locales.SelectionEditLink(), locales.SelectionEditLinkLabel(), false, a.editHoveredLink)
		btn("link-remove", iconUnlink, locales.SelectionRemoveLink(), locales.SelectionRemoveLinkLabel(), true, a.removeHoveredLink)
	})
}

// shownAddress is a link's address as its card writes it: without the scheme the web's and e-mails' all have.
func shownAddress(address string) string {
	for _, pre := range []string{"https://", "http://", "mailto:", "page://"} {
		if strings.HasPrefix(address, pre) {
			return strings.TrimSuffix(strings.TrimPrefix(address, pre), "/")
		}
	}
	return address
}

// hoveredLinkRange is where the link of the card is in the text: every rune about hoverPos with its address. It
// reports false when the text no longer has it there.
func (a *app) hoveredLinkRange() (from, to int, ok bool) {
	w := &a.web
	t := a.doc.Text()
	pos := w.hoverPos
	if w.hover == "" || pos < 0 || pos >= t.Len() || t.AttributesAt(pos).Link != w.hover {
		return 0, 0, false
	}
	s, e := t.ParagraphRange(pos)
	from, to = pos, pos
	for from > s && t.AttributesAt(from-1).Link == w.hover {
		from--
	}
	for to < e && t.AttributesAt(to).Link == w.hover {
		to++
	}
	return from, to, true
}

// editHoveredLink opens the bar's link field on the link of the card, its address in it to change.
func (a *app) editHoveredLink() {
	from, to, ok := a.hoveredLinkRange()
	if !ok {
		return
	}
	a.doc.SetSelection(from, to)
	a.sel.link, a.sel.url, a.sel.linkSel = true, a.web.hover, [2]int{from, to}
	a.web.hover, a.web.onCard = "", false
}

// removeHoveredLink takes the link of the card away, its text left as it is.
func (a *app) removeHoveredLink() {
	from, to, ok := a.hoveredLinkRange()
	if !ok {
		return
	}
	a.doc.SetSelection(from, to)
	a.doc.SetLink("")
	a.doc.SetSelection(to, to)
	a.web.hover, a.web.onCard = "", false
	a.refocus = true
}

// linkPicH is the height of a page's picture shown width wide: as Open Graph pictures are made, 1.91 to 1.
func linkPicH(width float32) float32 { return min(width/1.91, 200) }

// linkPreviewH is about the height linkPreview takes, big, for where its card goes.
func linkPreviewH(info *linkInfo, width float32) float32 {
	h := float32(62)
	if info.image != nil {
		h += linkPicH(width)
	}
	return h
}

// linkPreview is what a link's card shows, the hover's and the link field's: the page's picture, large, over its
// title and site.
func (a *app) linkPreview(c *ui.Context, address string, info *linkInfo, width float32) {
	cl := ryugo.From(c).Colors
	if info.image != nil {
		ui.Box(c.Key("link-pic")).Width(width).Height(linkPicH(width)).Background(cl.Fill).Draw(func(p *ui.Painter, r ui.Rect) {
			p.Image(info.image, r, ui.Cover)
		})
	}
	ui.Column(c.Key("link-text")).Padding(10, 12).Gap(3).Children(func() {
		if info.meta.Title != "" {
			ui.Text(c.Key("link-title"), info.meta.Title).FontSize(13.5).FontWeight(620).TextColor(cl.Text).MaxLines(2)
		}
		ui.Row(c.Key("link-site")).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Box(c.Key("link-icon")).Width(14).Height(14).Draw(func(p *ui.Painter, r ui.Rect) {
				a.drawLinkIcon(p, r, address, cl.Muted)
			})
			ui.Text(c.Key("link-host"), linkHost(address)).FontSize(12).TextColor(cl.Muted).MaxLines(1)
		})
	})
}
