package main

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	ryugo "github.com/imhalid/ryu-go"
)

// Links of the web, shown (#495): each with its site's icon before it in the text, room for which the span takes
// (Span.PaddingLeft); a card with its page's picture and title when the pointer rests on it; the same in the bar's
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
	// the link under the pointer, since when, and where it is drawn
	hover   string
	hoverAt time.Time
	hoverR  ui.Rect
}

var (
	// linkIcons is whether web links have their icons and cards (Settings.NoLinkPreviews off): the editor's spans
	// leave the icon's room by it (codecolors.go, caret.go). Set each frame.
	linkIcons = true
	// linkFetch is whether pages are asked for: never in a test, which is not to reach the web, but one of its
	// own that sets it.
	linkFetch = !testing.Testing()
	// linkHoverWait is how long the pointer rests on a link before its card shows.
	linkHoverWait = 450 * time.Millisecond
)

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
	if !linkIcons || !webLink(address) {
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
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			m, _ := library.FetchLinkMeta(ctx, address)
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
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			var img ui.ImageSource
			if data, err := library.FetchLinkImage(ctx, src); err == nil {
				img, _ = ui.DecodeImage(data)
			}
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
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var img ui.ImageSource
		if data, err := library.FetchLinkImage(ctx, address); err == nil {
			img, _ = ui.DecodeImage(data)
		}
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
// size.
func linkIconBox(rc ui.Rect, size float32) ui.Rect {
	pad := size * 1.2
	s := size * 0.9
	return ui.Rect{X: rc.X - pad + (pad-s)/2 - size*0.08, Y: rc.Y + (rc.H-s)/2, W: s, H: s}
}

// hoverLink keeps the link the pointer is on, (x, y) of the window, and since when: its card shows after
// linkHoverWait.
func (a *app) hoverLink(c *ui.Context, x, y float32, over bool) {
	w := &a.web
	address, r := "", ui.Rect{}
	if over {
		for _, h := range a.linkSeen {
			if x >= h.r.X && x < h.r.X+h.r.W && y >= h.r.Y && y < h.r.Y+h.r.H {
				address, r = h.url, h.r
				break
			}
		}
	}
	if !webLink(address) || !linkIcons {
		w.hover = ""
		return
	}
	if address != w.hover {
		w.hover, w.hoverAt = address, c.Now()
	}
	w.hoverR = r
	if wait := linkHoverWait - c.Now().Sub(w.hoverAt); wait > 0 {
		c.After(wait)
	}
}

// linkCardView is the card of the link the pointer rests on: its page's picture, its title and its site, under the
// link. It shows once the page has answered with something to show.
func (a *app) linkCardView(c *ui.Context) {
	w := &a.web
	if w.hover == "" || c.Now().Sub(w.hoverAt) < linkHoverWait || a.modal() {
		return
	}
	info := a.linkPage(w.hover, true)
	if info == nil || !info.done || (info.meta.Title == "" && info.image == nil) {
		return
	}
	ww, wh := c.Size()
	const cardW = 300
	left := max(8, min(w.hoverR.X, ww-cardW-8))
	top := w.hoverR.Y + w.hoverR.H + 6
	cardH := float32(62)
	if info.image != nil {
		cardH += 150
	}
	if top+cardH > wh-8 { // no room under the link: over it
		top = w.hoverR.Y - cardH - 6
	}
	tth := ryugo.From(c)
	ui.Overlay(c, func() {
		card := ui.Column(c.Key("link-card")).Absolute().Left(left).Top(top).Width(cardW).Radius(rad(12)).Clip().
			Background(tth.Colors.Card).Border(1, tth.Colors.Hairline).PassThrough().Transition(rowEnter)
		tth.Shadows.ShadowPop.Apply(card)
		card.Children(func() { a.linkPreview(c, w.hover, info, cardW, true) })
	})
}

// linkPreview is what a link's card shows, the hover's and the link field's: the page's picture over its title and
// site when big, beside them when not.
func (a *app) linkPreview(c *ui.Context, address string, info *linkInfo, width float32, big bool) {
	cl := ryugo.From(c).Colors
	if big && info.image != nil {
		ui.Box(c.Key("link-pic")).Width(width).Height(150).Background(cl.Fill).Draw(func(p *ui.Painter, r ui.Rect) {
			p.Image(info.image, r, ui.Cover)
		})
	}
	ui.Row(c.Key("link-words")).Padding(10, 12).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Column(c.Key("link-text")).Grow(1).MinWidth(0).Gap(3).Children(func() {
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
		if !big && info.image != nil {
			ui.Box(c.Key("link-thumb")).Width(64).Height(42).Radius(rad(6)).Clip().Background(cl.Fill).Draw(func(p *ui.Painter, r ui.Rect) {
				p.Image(info.image, r, ui.Cover)
			})
		}
	})
}
