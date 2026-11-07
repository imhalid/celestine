package main

import (
	"errors"
	"log"
	"regexp"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// Quick capture: a small panel over everything, a global key away, to put a
// thought into the journal's day - or into a page of its own - without going
// to the app. ⌘↵ (Ctrl↵) adds it, Esc puts the panel away; ↵ breaks the line.

// the key tried first, then the next; the desktop may bind others (Wayland
// asks the user)
var quickShortcuts = []string{"CmdOrCtrl+Alt+N", "Alt+Shift+N"}

const (
	quickW, quickH = 580, 250
	quickDay       = 0 // into today's page of the journal
	quickPage      = 1 // as a page of its own
)

// quickNote is the panel: what is typed, and where it goes.
type quickNote struct {
	a        *app
	panel    *mygo.Window
	text     string
	mode     int
	input    ui.Handle
	shortcut string
	said     string // what happened to the last note, shown in the panel

	focusWanted bool // the text takes the focus when the panel is shown
}

// listItem starts a line that is already a list's or a task's.
var listStart = regexp.MustCompile(`^\s*([-*+]|\d+[.)])\s`)

// noteLines is text as the lines of a list: each a bullet, unless it is one,
// and "[]" a task.
func noteLines(text string) []string {
	var out []string
	for _, ln := range strings.Split(strings.TrimSpace(text), "\n") {
		ln = strings.TrimRight(ln, " \t\r")
		if strings.TrimSpace(ln) == "" {
			continue
		}
		switch {
		case strings.HasPrefix(ln, "[]"):
			ln = "- [ ]" + ln[2:]
		case strings.HasPrefix(ln, "[ ]"):
			ln = "- " + ln
		case !listStart.MatchString(ln):
			ln = "- " + ln
		}
		out = append(out, ln)
	}
	return out
}

// quickCollection is the collection a note as a page goes to: the one shown,
// or the first that is not the journal.
func (a *app) quickCollection() *Collection {
	if c := a.currentColl(); c != nil && !c.Journal {
		return c
	}
	for _, c := range a.lib.Collections {
		if !c.Journal {
			return c
		}
	}
	return nil
}

// captureNote puts text away: lines at the end of today's page of the
// journal, or a page of its own; it says what it did.
func (a *app) captureNote(mode int, text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", errors.New("boş not")
	}
	if mode == quickPage {
		return a.captureAsPage(text)
	}
	p, err := a.todayPage()
	if err != nil {
		return "", err
	}
	if p == a.page {
		a.flush()
	}
	body, _ := p.Text()
	if strings.TrimSpace(body) != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	body += strings.Join(noteLines(text), "\n") + "\n"
	if err := p.SaveText(body); err != nil {
		return "", err
	}
	a.takeOutside(p) // the page, if it is open, shows it
	a.linksStale()
	a.noticePage = p
	return "Günlüğe eklendi", nil
}

// captureAsPage makes a page of the note: its first line is the title.
func (a *app) captureAsPage(text string) (string, error) {
	c := a.quickCollection()
	if c == nil {
		return "", errors.New("sayfa eklenecek koleksiyon yok")
	}
	lines := strings.Split(strings.TrimSpace(text), "\n")
	title := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(lines[0]), "#- "))
	if title == "" {
		title = "Not"
	}
	if r := []rune(title); len(r) > 60 {
		title = strings.TrimSpace(string(r[:60])) + "…"
	}
	p, err := c.NewPage(nil, title)
	if err != nil {
		return "", err
	}
	rest := strings.TrimSpace(strings.Join(lines[1:], "\n"))
	if title != strings.TrimSpace(lines[0]) { // cleaned or cut: the text stays whole
		rest = strings.TrimSpace(text)
		rest, _ = splitTitle(rest, title) // but not the heading that is the title
	}
	if err := p.SaveText(rest); err != nil {
		return "", err
	}
	a.linksStale()
	a.noticePage = p
	return "“" + title + "” sayfası eklendi: " + c.Name, nil
}

// view is the panel.
func (q *quickNote) view(c *ui.Context) {
	a := q.a
	ryugo.Root(c, ryugo.Options{Mode: []ryugo.Mode{ryugo.ModeInherit, ryugo.Light, ryugo.Dark}[a.settings.Theme%3]})
	t := ryugo.From(c)
	cl := t.Colors
	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	} else {
		c.Root().Background(cl.Card)
	}
	if c.Shortcut(0, ui.KeyEscape) && q.panel != nil {
		q.panel.Hide()
	}
	if c.Shortcut(ui.Cmd, ui.KeyEnter) {
		q.submit()
	}
	ui.Column(c).Fill().Padding(16).Gap(12).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ryugo.Segmented(c.Key("quick-mode"), &q.mode, []string{"Günlüğe", "Yeni sayfa"}, ryugo.SegmentedOptions{Label: "Nereye", Size: ryugo.Sm})
			ui.Text(c, q.where()).FontSize(12.5).TextColor(cl.Muted).Grow(1).MaxLines(1)
			ui.Text(c, T("Hızlı not")).FontSize(12.5).FontWeight(620).TextColor(cl.Muted)
		})
		ph := "Günlüğe bir not… [] ile görev, her satır bir madde"
		if q.mode == quickPage {
			ph = "İlk satır başlık olur…"
		}
		area := ryugo.Textarea(c.Key("quick-text"), &q.text, ryugo.TextareaOptions{Placeholder: ph, Label: "Not", Rows: 4, MaxRows: 4}).Bind(&q.input)
		if q.focusWanted {
			area.Focus()
			q.focusWanted = false
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			s := q.said
			if s == "" {
				s = "⌘↵ ekle   esc kapat"
			}
			ui.Text(c, s).FontSize(12).TextColor(cl.Muted).Grow(1)
			if ryugo.Button(c.Key("quick-add"), "Ekle", ryugo.ButtonOptions{Color: cl.Accent, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
				q.submit()
			}
		})
	})
}

// where says where the note will go.
func (q *quickNote) where() string {
	if q.mode == quickPage {
		if c := q.a.quickCollection(); c != nil {
			return "→ " + c.Name
		}
		return ""
	}
	if c, err := q.a.journal(); err == nil {
		return "→ " + c.Name + ", bugün"
	}
	return ""
}

// submit adds the note and puts the panel away.
func (q *quickNote) submit() {
	if strings.TrimSpace(q.text) == "" {
		return
	}
	var msg string
	err := q.a.do(func() error {
		var err error
		msg, err = q.a.captureNote(q.mode, q.text)
		if err == nil {
			q.a.notice = msg
		}
		return err
	})
	if err != nil {
		q.said = "Olmadı: " + err.Error()
		return
	}
	q.text, q.said = "", ""
	if q.panel != nil {
		q.panel.Hide()
	}
}

// newQuick makes the panel, hidden, and gives it its key.
func newQuick(a *app) *quickNote {
	q := &quickNote{a: a}
	q.panel = mygo.NewWindow(mygo.WindowOptions{
		Title:         "Hızlı not",
		Panel:         true,
		Hidden:        true,
		Frameless:     true,
		Transparent:   true,
		Vibrancy:      mygo.VibrancyGlass,
		CornerRadius:  16,
		Width:         quickW,
		Height:        quickH,
		DisableResize: true,
		Content:       ui.View(q.view),
	})
	// put away, not closed: it shows again as it was, a draft kept
	q.panel.OnClose(func(e *mygo.CloseEvent) {
		e.PreventDefault()
		q.panel.Hide()
	})
	q.panel.OnBlur(q.panel.Hide)
	keys := quickShortcuts
	if k := a.settings.QuickKey; k != "" { // the one chosen first
		keys = append([]string{k}, keys...)
	}
	for _, s := range keys {
		if err := mygo.GlobalShortcut.Register(s, q.toggle); err == nil {
			q.shortcut = s
			break
		} else {
			log.Printf("quick note, shortcut %s: %v", s, err)
		}
	}
	return q
}

// show brings the panel up on the display with the pointer, a third of the
// way down, with the text ready to type in.
func (q *quickNote) show() {
	d := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	w, h := q.panel.Size()
	q.panel.SetPosition(d.WorkArea.X+(d.WorkArea.Width-w)/2, d.WorkArea.Y+(d.WorkArea.Height-h)/3)
	q.said, q.focusWanted = "", true
	q.panel.Show()
	q.panel.Invalidate()
}

func (q *quickNote) toggle() {
	if q.panel.IsVisible() {
		q.panel.Hide()
	} else {
		q.show()
	}
}

// showQuick shows the panel; there is none in the tests.
func (a *app) showQuick() {
	if a.quick != nil && a.quick.panel != nil {
		a.quick.show()
	}
}

// quickKeys is the key of the panel, as the palette shows it.
func (a *app) quickKeys() string {
	if a.quick == nil {
		return ""
	}
	return strings.NewReplacer("CmdOrCtrl+", "⌘", "Alt+", "⌥", "Shift+", "⇧").Replace(a.quick.shortcut)
}

// setShortcut gives the panel another global key; the old one is kept when
// the new one cannot be had.
func (q *quickNote) setShortcut(acc string) error {
	if acc == "" || acc == q.shortcut {
		return nil
	}
	old := q.shortcut
	if old != "" {
		mygo.GlobalShortcut.Unregister(old)
	}
	if err := mygo.GlobalShortcut.Register(acc, q.toggle); err != nil {
		if old != "" {
			mygo.GlobalShortcut.Register(old, q.toggle)
		}
		return err
	}
	q.shortcut = acc
	return nil
}
