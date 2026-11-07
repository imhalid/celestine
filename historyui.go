package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The version history dialog: the copies kept of the page on the left, the
// one chosen on the right as a difference from the page now - what comes back
// green, what goes red - and the button to put it back.

var iconHistory = icon(`<path d="M4.5 12a7.5 7.5 0 1 0 2.2-5.3L4.5 9"/><path d="M4.5 4.5V9H9"/><path d="M12 8v4l2.5 1.5"/>`)

type histView struct {
	open   bool
	page   *Page
	vers   []Version
	sel    int
	ops    []diffOp
	opsFor int64  // the version the ops are of
	label  string // the name being typed for a copy to keep
	list   ui.ScrollState
	text   ui.ScrollState
}

// historyOf is where the page's versions are kept, if its library keeps any.
func historyOf(p *Page) (historian, bool) {
	h, ok := p.fs().(historian)
	return h, ok
}

// openHistory shows the versions of p.
func (a *app) openHistory(p *Page) {
	h, ok := historyOf(p)
	if !ok {
		a.notice = "Sürüm geçmişi yalnız tek dosya kitaplığında tutulur"
		return
	}
	a.flush()
	a.vhist = histView{open: true, page: p, vers: h.Versions(p.file()), opsFor: -1}
}

// histOps is the difference of the chosen version from the page now.
func (a *app) histOps() []diffOp {
	v := &a.vhist
	if v.sel >= len(v.vers) {
		return nil
	}
	if id := v.vers[v.sel].ID; id != v.opsFor {
		h, _ := historyOf(v.page)
		data, _ := h.VersionData(v.page.file(), id)
		_, oldBody := splitPage(data)
		old := bodyText(oldBody)
		now, _ := v.page.Text()
		v.ops = diffLines(strings.Split(strings.TrimRight(old, "\n"), "\n"), strings.Split(strings.TrimRight(now, "\n"), "\n"))
		pairWords(v.ops)
		v.opsFor = id
		v.text = ui.ScrollState{}
	}
	return v.ops
}

// putBack makes the page what the chosen version was; the text it had
// is kept as a version first, so that this too can be undone.
func (a *app) putBackVersion() {
	v := &a.vhist
	if v.sel >= len(v.vers) {
		return
	}
	p := v.page
	h, _ := historyOf(p)
	data, ok := h.VersionData(p.file(), v.vers[v.sel].ID)
	if !ok {
		return
	}
	if p == a.page {
		a.flush()
	}
	h.SnapshotNow(p.file())
	_, body := splitPage(data)
	if err := p.Save(body); err != nil {
		a.saveErr = err.Error()
		return
	}
	a.takeOutside(p)
	a.linksStale()
	a.notice = Tf("“%s” %s sürümüne döndü", p.Title, v.vers[v.sel].At.Format("2 Jan 15:04"))
	v.open = false
}

// historyView is the dialog.
func (a *app) historyView(c *ui.Context) {
	v := &a.vhist
	if !v.open || v.page == nil {
		return
	}
	cl := ryugo.From(c).Colors
	now := a.clock()
	ryugo.Dialog(c.Key("history"), &v.open, ryugo.DialogOptions{Title: "Sürüm geçmişi: " + v.page.Title, Width: 940, Footer: func() {
		hint := "Sayfanın bir süre önceki halleri; her 5 dakikada bir yeni kopya tutulur."
		if len(v.vers) == 0 {
			hint = "Henüz kopya yok: sayfa değiştikçe burada birikir."
		}
		ui.Text(c, hint).FontSize(12.5).TextColor(cl.Muted).Grow(1).MaxLines(2)
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			in := ryugo.Input(c.Key("hist-name"), &v.label, ryugo.InputOptions{Label: "Sürümün adı", Placeholder: "Şu hâli adla sakla…", Size: ryugo.Sm}).Width(190)
			keep := ryugo.Button(c.Key("hist-keep"), "Sakla", ryugo.ButtonOptions{Icon: iconHistory, Size: ryugo.Sm}).Clicked() || in.Submitted()
			if keep {
				a.saveNamedVersion()
			}
		})
		if len(v.vers) > 0 && ryugo.Button(c.Key("hist-back"), "Bu sürüme dön", ryugo.ButtonOptions{Icon: iconRestore, Color: cl.Accent, Variant: ryugo.Solid, Size: ryugo.Sm}).Clicked() {
			a.putBackVersion()
		}
	}}, func() {
		if len(v.vers) == 0 {
			ui.Column(c).FillWidth().Height(200).Justify(ui.Center).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Icon(c, iconHistory).Width(34).Height(34).TextColor(cl.Muted.Alpha(0.6))
				ui.Text(c, T("Bu sayfanın kopyası yok")).TextColor(cl.Muted)
			})
			return
		}
		ui.Row(c).FillWidth().Height(420).Gap(14).AlignItems(ui.Stretch).Children(func() {
			ui.Scroll(c.Key("hist-list")).TrackScroll(&v.list).Width(230).Shrink(0).Children(func() {
				ui.Column(c).FillWidth().Gap(2).Padding(0, 4, 0, 0).Children(func() {
					for i, ver := range v.vers {
						i, ver := i, ver
						row := ui.Column(c.Key("hv:"+strconv.FormatInt(ver.ID, 10))).FillWidth().Padding(7, 10).Radius(rad(9)).Gap(1).
							Label("Sürüm: " + ver.At.Format("2 Jan 15:04")).Role(ui.RoleButton).Cursor(ui.CursorPointer)
						switch {
						case i == v.sel:
							row.LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, From: ryugo.Mix(cl.Card, cl.Accent, 0.26), To: ryugo.Mix(cl.Card, cl.Accent, 0.14)}).Border(1, cl.Accent.Alpha(0.4))
						case row.Hovered():
							row.Background(cl.Fill)
						}
						row.Children(func() {
							head := ago(ver.At, now)
							if ver.Label != "" {
								head = ver.Label
							}
							ui.Text(c, head).FontSize(13.5).FontWeight(600).TextColor(cl.Text).MaxLines(1)
							ui.Text(c, ver.At.Format("2 Jan 2006, 15:04")+"  ·  "+sizeText(ver.Size)).FontSize(11.5).TextColor(cl.Muted)
						})
						if row.Clicked() {
							v.sel = i
						}
					}
				})
			})
			ui.Box(c).Width(1).Background(cl.Hairline)
			ops := a.histOps()
			ui.Scroll(c.Key("hist-text")).TrackScroll(&v.text).Grow(1).MinWidth(0).Children(func() {
				ui.Column(c).FillWidth().Gap(1).Padding(0, 8, 8, 4).Children(func() { a.diffRows(c, cl, ops) })
			})
		})
	})
}

// diffRows draws the lines of a difference: a long run of lines both have
// is cut to the few at its edges.
func (a *app) diffRows(c *ui.Context, cl ryugo.Colors, ops []diffOp) {
	same, adds, dels := 0, 0, 0
	for _, o := range ops {
		switch o.kind {
		case '=':
			same++
		case '+':
			adds++
		default:
			dels++
		}
	}
	if adds+dels == 0 {
		ui.Text(c, T("Bu sürüm sayfanın şimdiki halinin aynısı.")).FontSize(13).TextColor(cl.Muted).Padding(8, 4)
		return
	}
	ui.Text(c, strconv.Itoa(adds)+" satır geri gelir, "+strconv.Itoa(dels)+" satır gider").FontSize(12).FontWeight(600).TextColor(cl.Muted).Padding(0, 4, 6, 4)
	const edge = 2
	for i := 0; i < len(ops); {
		if ops[i].kind == '=' {
			j := i
			for j < len(ops) && ops[j].kind == '=' {
				j++
			}
			if j-i > 2*edge+2 {
				for k := i; k < i+edge; k++ {
					a.diffRow(c, cl, k, ops[k])
				}
				ui.Text(c.Key("skip"+strconv.Itoa(i)), fmt.Sprintf("⋯ %d satır aynı", j-i-2*edge)).FontSize(11.5).TextColor(cl.Muted.Alpha(0.8)).Padding(3, 4)
				for k := j - edge; k < j; k++ {
					a.diffRow(c, cl, k, ops[k])
				}
			} else {
				for k := i; k < j; k++ {
					a.diffRow(c, cl, k, ops[k])
				}
			}
			i = j
			continue
		}
		a.diffRow(c, cl, i, ops[i])
		i++
	}
}

func (a *app) diffRow(c *ui.Context, cl ryugo.Colors, i int, o diffOp) {
	row := ui.Row(c.Key("dl"+strconv.Itoa(i))).FillWidth().Gap(8).Padding(2, 6).Radius(rad(5)).AlignItems(ui.Start)
	mark, col := " ", cl.Text2
	switch o.kind {
	case '+':
		mark, col = "+", cl.Down
		row.Background(cl.Down.Alpha(0.13))
	case '-':
		mark, col = "−", cl.Up
		row.Background(cl.Up.Alpha(0.11))
	}
	row.Children(func() {
		ui.Text(c, mark).FontSize(13).FontWeight(700).TextColor(col).Width(10).Shrink(0)
		text := o.text
		if strings.TrimSpace(text) == "" {
			text = " "
		}
		if len(o.words) > 0 { // the words that differ marked within the line
			var spans []ui.Span
			for k, w := range o.words {
				sp := ui.Span{Text: w.text, Size: 13, Color: cl.Text}
				if k > 0 {
					spans = append(spans, ui.Span{Text: " ", Size: 13})
				}
				if w.kind != '=' {
					sp.Weight, sp.Background, sp.Radius = 700, col.Alpha(0.28), 3
					sp.Strikethrough = o.kind == '-'
				}
				spans = append(spans, sp)
			}
			ui.RichText(c, spans...).Grow(1).MinWidth(0)
			return
		}
		t := ui.Text(c, text).FontSize(13).TextColor(map[bool]ui.Color{true: cl.Muted, false: cl.Text}[o.kind == '=']).Grow(1).MinWidth(0).Wrap()
		if o.kind == '-' {
			t.Strikethrough()
		}
	})
}

// sizeText is a size in bytes as a few words of it.
func sizeText(n int) string {
	if n < 1024 {
		return strconv.Itoa(n) + " B"
	}
	return strconv.FormatFloat(float64(n)/1024, 'f', 1, 64) + " KB"
}

// saveNamedVersion keeps the page as it is now under the name typed.
func (a *app) saveNamedVersion() {
	v := &a.vhist
	name := strings.TrimSpace(v.label)
	if name == "" {
		name = "Sakladığım sürüm"
	}
	h, ok := historyOf(v.page)
	if !ok {
		return
	}
	if v.page == a.page {
		a.flush()
	}
	if h.SaveNamed(v.page.file(), name) {
		v.vers, v.sel, v.opsFor, v.label = h.Versions(v.page.file()), 0, -1, ""
	}
}
