package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The pictures of the app for the site, the README and the release notes: the app draws its own view with a Tester,
// from a library made for them, so each picture is the same however many times it is made. Run with SITESHOTS=dir:
// dir/light and dir/dark get the whole windows (shotW x shotH points, at shotScale) and the close-ups (at detailScale,
// cropped to the detail). SITE_ONLY=name,name makes only the pictures whose names contain one of those, and
// SITE_THEME=light or dark makes one theme.

const (
	shotW, shotH = 1440, 900
	shotScale    = 2 // 2880x1800
	detailScale  = 4 // a close-up is cropped from a window drawn this sharp
	workScale    = 1 // what the app is stepped at: the pictures are drawn sharp once, as they are saved
)

// siteDay is the day of the pictures: the clock stands here.
var siteDay = time.Date(2026, 11, 13, 10, 30, 0, 0, time.Local)

// launchPlan is the page of the main picture: a little of everything.
const launchPlan = `Everything for the **v1.0** release, in one place. The schedule is in [[Roadmap]], the notes of the last review in [[Design review]].

## This week

- [x] Freeze the feature list
- [*] Record the demo video
- [!] Fix the sync conflict on slow networks
- [/] Write the release notes
- [?] Do we ship Windows on the first day?

> The installer is signed, but the updater key changes on Friday. Nothing to do until then.

## Where people come from

| Channel | Visits | Signups |
|---|---:|---:|
| Search | 12,480 | 1,310 |
| Newsletter | 4,920 | 880 |
| Friends | 2,175 | 640 |
`

// designReview is a page of lists, for the second pane.
const designReview = `Notes from Tuesday.

## What the review said

- The sidebar is calmer now
    - Keep the tree folded by default
    - Pin the collections people open daily
- The table needs wider dates
- Onboarding should take under a minute

## To do

1. Widen the date column
2. Write the onboarding copy
3. Ask Ece for the icons
`

// releaseChecks is a page with code in it, for a close-up.
const releaseChecks = `The check every build goes through before it leaves the machine.

---

` + "```go" + `
func ready(b Build) bool {
	return b.Tests.Passed && b.Signed
}
` + "```" + `

- [x] Tests pass on all three systems
- [x] The installer is signed
- [ ] The update feed points at the new build
`

// siteLibrary makes the library of the pictures: a place to write, a table of books, a board of projects, a journal.
func siteLibrary(t *testing.T) (*Library, *Page) {
	t.Helper()
	st, now := storeAt(t) // a store, for the page's history
	*now = siteDay.Add(-20 * 24 * time.Hour)
	lib, err := OpenLibraryIn(st, StoreRoot)
	if err != nil {
		t.Fatal(err)
	}
	day := 24 * time.Hour
	mk := func(c *Collection, parent *Page, title, body string, ago time.Duration, changed time.Duration) *Page {
		p, err := c.NewPage(parent, title)
		if err != nil {
			t.Fatal(err)
		}
		p.Created = siteDay.Add(-ago)
		if err := p.SaveText(body); err != nil {
			t.Fatal(err)
		}
		p.Modified = siteDay.Add(-changed)
		return p
	}
	notes, _ := lib.NewCollection("Notes")
	notes.Icon, notes.Color = "", ""
	notes.AddProperty("Status", PropChoice, []string{"Draft", "In review", "Done"})
	notes.AddProperty("Owner", PropText, nil)
	notes.AddProperty("Due", PropDate, nil)
	plan := mk(notes, nil, "Launch plan", launchPlan, 20*day, 2*time.Hour)
	plan.SetValue("status", "In review")
	plan.SetValue("owner", "Mira")
	plan.SetValue("due", "2026-11-20")
	// its history: three earlier copies, on three days, the store stamping each with the time of the computer
	week := strings.SplitAfter(launchPlan, "## This week")
	for _, s := range []string{
		"Everything for the **v1.0** release, in one place.\n",
		week[0] + "\n\n- [ ] Freeze the feature list\n- [ ] Record the demo video\n",
		strings.Split(launchPlan, "## Where people")[0],
		launchPlan,
	} {
		*now = now.Add(10 * time.Minute)
		plan.SaveText(s)
	}
	vs := st.Versions(plan.File())
	for i, d := range []int{1, 3, 6} {
		if i < len(vs) {
			st.DB.Exec(`UPDATE versions SET at = ? WHERE id = ?`, siteDay.Add(-time.Duration(d)*day).UnixNano(), vs[i].ID)
		}
	}
	plan.Modified = siteDay.Add(-2 * time.Hour)
	mk(notes, plan, "Press kit", "Logos, screenshots and a one-paragraph description.\n", 18*day, 3*day)
	rc := mk(notes, plan, "Release checks", releaseChecks, 12*day, 20*time.Hour)
	rc.SetValue("status", "Done")
	rc.SetValue("owner", "Deniz")
	rc.SetValue("due", "2026-11-18")
	mk(notes, plan, "Launch day checklist", "- [x] Tag the release\n- [ ] Publish the notes\n- [ ] Tell the beta list\n", 9*day, 26*time.Hour)
	mk(notes, nil, "Roadmap", "## Next\n\n- Sync across devices\n- Page history\n- A calendar for every collection\n", 60*day, 5*day)
	dr := mk(notes, nil, "Design review", designReview, 14*day, 4*day)
	dr.SetValue("status", "Draft")
	dr.SetValue("owner", "Ece")
	dr.SetValue("due", "2026-11-17")
	mk(notes, nil, "Meeting notes", "Weekly sync, Monday.\n", 30*day, 6*day)

	books, _ := lib.NewCollection("Reading list")
	books.AddProperty("Status", PropChoice, []string{"To read", "Reading", "Finished"})
	books.AddProperty("Author", PropText, nil)
	books.AddProperty("Year", PropNumber, nil)
	for _, b := range []struct {
		title, author, year, status string
		ago                         time.Duration
	}{
		{"The Dispossessed", "Ursula K. Le Guin", "1974", "Reading", 6},
		{"Piranesi", "Susanna Clarke", "2020", "Finished", 30},
		{"A Pattern Language", "Christopher Alexander", "1977", "Reading", 12},
		{"The Overstory", "Richard Powers", "2018", "To read", 3},
		{"Braiding Sweetgrass", "Robin Wall Kimmerer", "2013", "Finished", 45},
		{"Invisible Cities", "Italo Calvino", "1972", "To read", 2},
		{"The Left Hand of Darkness", "Ursula K. Le Guin", "1969", "Finished", 80},
	} {
		p := mk(books, nil, b.title, "Notes while reading.\n", b.ago*day, b.ago*day/2)
		p.SetValue("status", b.status)
		p.SetValue("author", b.author)
		p.SetValue("year", b.year)
	}
	books.Views = []View{
		{ID: "table", Label: "Table", Type: "table", Sort: "title", Columns: []string{"prop:status", "prop:author", "prop:year", "created"}},
		{ID: "list", Label: "List", Type: "list", Sort: "modified", Desc: true},
	}
	books.ActiveView = "table"
	books.Save()

	projects, _ := lib.NewCollection("Projects")
	projects.AddProperty("Stage", PropChoice, []string{"Idea", "Doing", "Review", "Shipped"})
	projects.AddProperty("Owner", PropText, nil)
	for _, b := range []struct{ title, stage, owner string }{
		{"Onboarding tour", "Doing", "Mira"},
		{"Offline mode", "Doing", "Deniz"},
		{"Share a page by link", "Idea", "Ece"},
		{"Dark theme polish", "Review", "Mira"},
		{"Import from Markdown", "Shipped", "Deniz"},
		{"Keyboard cheat sheet", "Review", "Ece"},
		{"Web clipper", "Idea", "Deniz"},
		{"Page history", "Shipped", "Mira"},
	} {
		p := mk(projects, nil, b.title, "", 10*day, 2*day)
		p.SetValue("stage", b.stage)
		p.SetValue("owner", b.owner)
	}
	projects.Views = []View{
		{ID: "board", Label: "Board", Type: "board", Sort: "rank", GroupBy: "stage", Fields: []string{"prop:owner"}},
		{ID: "table", Label: "Table", Type: "table", Sort: "title"},
	}
	projects.ActiveView = "board"
	projects.Save()

	journal, _ := lib.NewCollection("Journal")
	for _, e := range []struct {
		ago   int
		title string
	}{
		{0, "Standup notes"}, {1, "Fixed the sync bug at last"}, {1, "Evening walk"}, {3, "Review with the team"}, {3, "Shopping list"}, {3, "Release notes, a first draft"},
		{5, "Quiet morning, long walk"}, {7, "Recorded the demo"}, {7, "Reading: Piranesi"}, {9, "Rest day"}, {11, "Plan for the month"}, {12, "Meeting with Deniz"}, {12, "Ideas for the site"},
		{14, "Trip notes"}, {16, "Weekly review"}, {17, "Sketches for the onboarding"}, {19, "Books to buy"}, {23, "Tax papers"},
	} {
		mk(journal, nil, e.title, e.title+".\n", time.Duration(e.ago)*day, time.Duration(e.ago)*day)
	}
	journal.Views = []View{
		{ID: "calendar", Label: "Calendar", Type: "calendar", Sort: "created", DateField: "created"},
		{ID: "list", Label: "List", Type: "list", Sort: "created", Desc: true},
	}
	journal.ActiveView = "calendar"
	journal.Save()
	return lib, plan
}

// siteScene is a picture, or a few: it draws the app into s and saves them.
type siteScene struct {
	name string
	run  func(t *testing.T, s *siteShot)
}

// siteShot is the app being drawn for the pictures.
type siteShot struct {
	a    *app
	tt   *ui.Tester
	lib  *Library
	plan *Page
	dir  string
	dark bool
}

// tree folds the sidebar's tree but for the collection called name, and the Launch plan in Notes.
func (s *siteShot) tree(name string) {
	s.a.collapseAll()
	for _, c := range s.lib.Collections {
		c.Open = c.Name == "Notes" || (c.Name == name && name != "Journal") // the journal has a page a day: its tree is long
	}
	s.plan.Open = true
	s.tt.Frame()
}

// page opens the Launch plan.
func (s *siteShot) page() {
	s.a.open(s.plan)
	s.tree("Notes")
	s.tt.Advance(400 * time.Millisecond)
}

// collection opens the collection called name, its page.
func (s *siteShot) collection(t *testing.T, name string) {
	t.Helper()
	s.tree(name)
	if err := s.tt.Click("Collection: " + name); err != nil {
		t.Fatal(err)
	}
	s.tree(name)
	s.tt.Advance(600 * time.Millisecond)
}

// window saves the whole window, at shotScale.
func (s *siteShot) window(t *testing.T, name string) {
	s.park()
	s.crop(t, name, 0, 0, shotW, shotH, shotScale)
}

// card saves the page's card alone, at shotScale: the window without its sidebar and the frame round the panes, which the site
// puts in a frame of its own; h is how far down it goes, in points from its top.
func (s *siteShot) card(t *testing.T, name string, h float32) {
	t.Helper()
	s.park()
	x := s.a.sidebarWidth() + 16
	s.crop(t, name, x, 10, shotW-11-x, h, shotScale)
}

// crop saves the part x, y, w, h of the window, in points, drawn at scale: a close-up.
func (s *siteShot) crop(t *testing.T, name string, x, y, w, h, scale float32) {
	t.Helper()
	s.tt.SetScale(scale)
	s.tt.Frame()
	img := s.tt.Image()
	r := image.Rect(int(x*scale), int(y*scale), int((x+w)*scale), int((y+h)*scale)).Intersect(img.Rect)
	out := img.SubImage(r)
	f, err := os.Create(filepath.Join(s.dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, out); err != nil {
		t.Fatal(err)
	}
	s.tt.SetScale(workScale)
	s.tt.Frame()
}

// around is the rectangle of the element called label, grown by pad points each way, kept in the window.
func (s *siteShot) around(t *testing.T, label string, pad float32) (x, y, w, h float32) {
	t.Helper()
	r, ok := s.tt.Find(label)
	if !ok {
		t.Fatalf("nothing called %q", label)
	}
	x, y = max(0, r.X-pad), max(0, r.Y-pad)
	return x, y, min(shotW-x, r.W+2*pad), min(shotH-y, r.H+2*pad)
}

var siteScenes = []siteScene{
	{"editor", func(t *testing.T, s *siteShot) {
		s.page()
		s.card(t, "editor", 770)
	}},
	{"table", func(t *testing.T, s *siteShot) {
		s.collection(t, "Reading list")
		s.card(t, "table", 520)
	}},
	{"board", func(t *testing.T, s *siteShot) {
		s.collection(t, "Projects")
		s.card(t, "board", 440)
	}},
	{"calendar", func(t *testing.T, s *siteShot) {
		s.collection(t, "Journal")
		s.card(t, "calendar", 690)
	}},
	{"calendar-day", func(t *testing.T, s *siteShot) {
		s.collection(t, "Journal")
		s.a.calDay, s.a.calOpen = siteDay.Add(-3*24*time.Hour), true
		s.tt.Advance(900 * time.Millisecond)
		s.card(t, "calendar-day", 690)
	}},
	{"palette", func(t *testing.T, s *siteShot) {
		s.page()
		s.a.openPalette()
		s.tt.Advance(500 * time.Millisecond)
		s.tt.Type("rel")
		s.tt.Advance(300 * time.Millisecond)
		s.card(t, "palette", 720)
	}},
	{"history", func(t *testing.T, s *siteShot) {
		s.page()
		s.a.openTimeMachine(s.plan)
		s.tt.Frame()
		s.tt.Advance(900 * time.Millisecond)
		s.tt.Key(0, ui.KeyLeft)
		s.tt.Advance(900 * time.Millisecond)
		s.window(t, "history")
	}},
	{"timelapse", func(t *testing.T, s *siteShot) {
		s.a.settings.NoLapse = false
		s.page()
		for range 200 {
			s.tt.Advance(20 * time.Millisecond)
			if len(s.a.lapse.days) == 4 && !s.a.lapse.busy {
				break
			}
		}
		s.tt.Advance(300 * time.Millisecond)
		s.card(t, "timelapse", 770)
	}},
	{"settings", func(t *testing.T, s *siteShot) {
		s.page()
		s.a.openSettings()
		s.tt.Advance(600 * time.Millisecond)
		s.card(t, "settings", shotH-37)
	}},
	{"split", func(t *testing.T, s *siteShot) {
		s.page()
		s.a.openInSide(place{kind: "page", dir: s.named(t, "Design review").Dir})
		s.tt.Advance(900 * time.Millisecond)
		s.card(t, "split", shotH-37)
	}},
}

// TestSiteShots draws the pictures of siteScenes, light and dark: run with SITESHOTS=dir.
func TestSiteShots(t *testing.T) {
	root := os.Getenv("SITESHOTS")
	if root == "" {
		t.Skip()
	}
	only := strings.Split(os.Getenv("SITE_ONLY"), ",")
	wanted := func(name string) bool {
		if os.Getenv("SITE_ONLY") == "" {
			return true
		}
		for _, o := range only {
			if o != "" && strings.Contains(name, o) {
				return true
			}
		}
		return false
	}
	toastShaders = true
	t.Cleanup(func() { toastShaders = false })
	setLang("en")
	t.Cleanup(func() { setLang("tr") })
	for _, theme := range []string{"light", "dark"} {
		if th := os.Getenv("SITE_THEME"); th != "" && th != theme {
			continue
		}
		for _, sc := range append(append([]siteScene{}, siteScenes...), detailScenes...) {
			if !wanted(sc.name) {
				continue
			}
			t.Run(theme+"/"+sc.name, func(t *testing.T) {
				dir := filepath.Join(root, theme)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				lib, plan := siteLibrary(t)
				a := newApp(lib)
				a.noSound = true
				a.clock = func() time.Time { return siteDay }
				a.settings.Theme = map[string]int{"light": 1, "dark": 2}[theme]
				a.settings.NoLapse = true // words colored by day only in the picture of the time-lapse
				tt := ui.NewTester(a.view, shotW, shotH)
				tt.SetScale(workScale)
				a.clipRead = tt.Clipboard
				tt.Advance(500 * time.Millisecond)
				s := &siteShot{a: a, tt: tt, lib: lib, plan: plan, dir: dir, dark: theme == "dark"}
				sc.run(t, s)
			})
		}
	}
}

// named is the page of Notes called title.
func (s *siteShot) named(t *testing.T, title string) *Page {
	t.Helper()
	var find func(ps []*Page) *Page
	find = func(ps []*Page) *Page {
		for _, p := range ps {
			if p.Title == title {
				return p
			}
			if q := find(p.Children); q != nil {
				return q
			}
		}
		return nil
	}
	p := find(s.lib.Collections[0].Pages)
	if p == nil {
		t.Fatalf("no page %q", title)
	}
	return p
}

// park moves the pointer out of the way, to the window's corner, and lets what it was over settle.
func (s *siteShot) park() {
	s.tt.Move(shotW-24, shotH-24)
	s.tt.Advance(400 * time.Millisecond)
}

// block is where the paragraph s of the open page is, in the window, in points: its box across the editor.
func (s *siteShot) block(t *testing.T, text string) ui.Rect {
	t.Helper()
	start := at(s.a.doc, text)
	if start < 0 {
		t.Fatalf("no paragraph %q", text)
	}
	return s.blockAt(t, start)
}

// blockAt is where the paragraph that starts at rune start is, in the window.
func (s *siteShot) blockAt(t *testing.T, start int) ui.Rect {
	t.Helper()
	r := s.a.editorR
	for _, b := range s.a.boxes {
		if b.Start == start {
			return ui.Rect{X: r.X, Y: r.Y + b.Top, W: r.W, H: b.Height}
		}
	}
	t.Fatalf("no box at %d", start)
	return ui.Rect{}
}

// blockEnd is the rune the paragraph text ends at.
func (s *siteShot) blockEnd(t *testing.T, text string) int {
	t.Helper()
	start := at(s.a.doc, text)
	if start < 0 {
		t.Fatalf("no paragraph %q", text)
	}
	return start + len([]rune(text))
}

// hover leaves the pointer over the middle of r for a moment.
func (s *siteShot) hover(r ui.Rect) {
	s.tt.Move(r.X+60, r.Y+r.H/2)
	s.tt.Frame()
	s.tt.Advance(250 * time.Millisecond)
}

// detailScenes are the close-ups.
var detailScenes = []siteScene{
	{"tasks", func(t *testing.T, s *siteShot) {
		s.page()
		s.park()
		a, b := s.block(t, "Freeze the feature list"), s.block(t, "Do we ship Windows on the first day?")
		s.crop(t, "tasks", a.X-24, a.Y-12, 520, b.Y+b.H-a.Y+9, detailScale)
	}},
	{"properties", func(t *testing.T, s *siteShot) {
		s.page()
		s.park()
		x, y, _, _ := s.around(t, "Add a property", 0)
		s.crop(t, "properties", x-20, y-168, 440, 210, detailScale)
	}},
	{"chips", func(t *testing.T, s *siteShot) {
		s.page()
		s.park()
		a := s.blockAt(t, 0)
		s.crop(t, "chips", a.X-16, a.Y-12, 640, a.H+24, detailScale)
	}},
	{"table-block", func(t *testing.T, s *siteShot) {
		s.page()
		s.park()
		a := s.blockAt(t, at(s.a.doc, "Where people come from")+len("Where people come from")+1)
		s.crop(t, "table-block", a.X-16, a.Y-3, 480, a.H+19, detailScale)
	}},
	{"grip", func(t *testing.T, s *siteShot) {
		s.page()
		b := s.block(t, "Record the demo video")
		s.hover(b)
		s.hover(b)
		s.crop(t, "grip", b.X-90, b.Y-50, 520, b.H+100, detailScale)
		g, ok := s.tt.Find("Block handle")
		if !ok {
			t.Fatal("no block handle")
		}
		s.tt.Move(g.X+g.W/2, g.Y+g.H/2)
		s.tt.Frame()
		if err := s.tt.Click("Block handle"); err != nil {
			t.Fatal(err)
		}
		s.tt.Advance(400 * time.Millisecond)
		s.crop(t, "block-menu", b.X-90, b.Y-50, 420, 280, detailScale)
	}},
	{"slash", func(t *testing.T, s *siteShot) {
		s.page()
		end := s.blockEnd(t, "The installer is signed, but the updater key changes on Friday. Nothing to do until then.")
		s.a.doc.SetSelection(end, end)
		s.tt.Frame()
		s.tt.Key(0, ui.KeyEnter)
		s.tt.Frame()
		typeEach(s.tt, "/")
		s.tt.Advance(300 * time.Millisecond)
		r, _ := s.tt.TextCaret()
		s.crop(t, "slash-menu", r.X-40, r.Y-350, 520, 390, detailScale)
	}},
	{"at-menu", func(t *testing.T, s *siteShot) {
		s.page()
		end := s.blockEnd(t, "The installer is signed, but the updater key changes on Friday. Nothing to do until then.")
		s.a.doc.SetSelection(end, end)
		s.tt.Frame()
		s.tt.Key(0, ui.KeyEnter)
		s.tt.Frame()
		typeEach(s.tt, "Next, see @")
		s.tt.Advance(300 * time.Millisecond)
		r, _ := s.tt.TextCaret()
		s.crop(t, "at-menu", r.X-140, r.Y-350, 560, 390, detailScale)
	}},
	{"code", func(t *testing.T, s *siteShot) {
		s.page()
		s.a.open(s.named(t, "Release checks"))
		s.tt.Advance(500 * time.Millisecond)
		s.park()
		a, z := s.block(t, "func ready(b Build) bool {"), s.block(t, "}")
		a.H = z.Y + z.H - a.Y
		s.crop(t, "code", a.X-20, a.Y-18, 560, a.H+28, detailScale)
	}},
	{"toast", func(t *testing.T, s *siteShot) {
		s.page()
		s.park()
		toastShaders = false
		s.a.toastOf(toastOK, "Page moved to the trash", "Undo", func() {})
		s.tt.Frame()
		s.tt.Advance(1300 * time.Millisecond)
		toastShaders = true
		s.crop(t, "toast", shotW-460, shotH-130, 460, 130, 3)
	}},
	{"sidebar", func(t *testing.T, s *siteShot) {
		s.page()
		s.park()
		s.crop(t, "sidebar", 0, 0, 230, 380, detailScale)
	}},
	{"statusbar-lapse", func(t *testing.T, s *siteShot) {
		s.a.settings.NoLapse = false
		s.page()
		for range 200 {
			s.tt.Advance(20 * time.Millisecond)
			if len(s.a.lapse.days) == 4 && !s.a.lapse.busy {
				break
			}
		}
		s.park()
		s.crop(t, "statusbar-lapse", shotW-520, shotH-34, 520, 34, detailScale)
	}},
}

// caretShot is the page with a new line written in it by the caret called name, drawn close at its end.
func caretShot(name string) siteScene {
	return siteScene{"caret-" + name, func(t *testing.T, s *siteShot) {
		s.page()
		end := s.blockEnd(t, "The installer is signed, but the updater key changes on Friday. Nothing to do until then.")
		s.a.doc.SetSelection(end, end)
		s.tt.Frame()
		s.tt.Key(0, ui.KeyEnter)
		s.tt.Frame()
		s.a.caretKind = caretNamed(t, name)
		for _, r := range "Ship it on Friday, and then rest." {
			s.tt.Type(string(r))
			s.tt.Advance(55 * time.Millisecond)
		}
		s.tt.Advance(120 * time.Millisecond)
		r, _ := s.tt.TextCaret()
		s.crop(t, "caret-"+name, r.X-300, r.Y-52, 420, 88, detailScale)
	}}
}

func init() {
	for _, name := range []string{"Slime", "torch", "firefly", "wind"} {
		detailScenes = append(detailScenes, caretShot(name))
	}
}
