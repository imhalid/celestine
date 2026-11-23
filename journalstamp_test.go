package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/internal/library"
)

// The pages of the journal from before their days were kept get them in the background: the meta of each file alone,
// the time it was last written kept, so they do not become the pages changed last; and a page saved meanwhile keeps
// what was saved (#538). The store's are stamped in parts, and a page saved after the last part, before the days are
// in memory, is written without its day and stamped again (#565).
func TestJournalStampDaysKeepsModified(t *testing.T) {
	stampChunk = 2 // three pages: two changes of the store
	t.Cleanup(func() { stampChunk = 200 })
	for _, kind := range []string{"store", "folder"} {
		t.Run(kind, func(t *testing.T) {
			var (
				fsys FS
				root string
				age  func(file string, at time.Time)
			)
			if kind == "store" {
				st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { st.Close() })
				fsys, root = st, StoreRoot
				age = func(file string, at time.Time) {
					if _, err := st.DB.Exec(`UPDATE nodes SET mtime = ? WHERE path = ?`, at.UnixNano(), library.Clean(file)); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				fsys, root = OSFS{}, t.TempDir()
				age = func(file string, at time.Time) {
					if err := os.Chtimes(file, at, at); err != nil {
						t.Fatal(err)
					}
				}
			}
			// a journal of three days, written by a version that kept no days
			lib, err := OpenLibraryIn(fsys, root)
			if err != nil {
				t.Fatal(err)
			}
			c, err := lib.NewCollection("Journal")
			if err != nil {
				t.Fatal(err)
			}
			days := []string{"2025-01-10", "2025-01-11", "2025-01-12"}
			was := map[string]time.Time{}
			for i, s := range days {
				d, _ := time.ParseInLocation("2006-01-02", s, time.Local)
				p, err := c.NewPage(nil, s)
				if err != nil {
					t.Fatal(err)
				}
				p.Created = d.Add(9 * time.Hour)
				if err := p.SaveText("words of " + s + "\n"); err != nil {
					t.Fatal(err)
				}
				at := time.Date(2025, 2, 1+i, 8, 0, 0, 0, time.UTC)
				age(p.File(), at)
				was[s] = at
			}
			c.Journal = true // marked after its pages: a journal takes only today's
			if err := c.Save(); err != nil {
				t.Fatal(err)
			}

			lib, err = OpenLibraryIn(fsys, root)
			if err != nil {
				t.Fatal(err)
			}
			a := newApp(lib)
			release := make(chan struct{})
			held := make(chan struct{})
			a.stampHold = func() { close(held); <-release }
			dir := lib.Collections[0].Dir
			stamped := func(s string) bool {
				src, err := fsys.ReadFile(filepath.Join(dir, s, PageFileName))
				return err == nil && PageMeta(src)[journalDayKey] == s
			}

			// what a frame does: it asks for the journal, with the lock held, and returns before anything is written
			a.mu.Lock()
			j, err := a.journal()
			a.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			<-held
			for _, s := range days {
				if stamped(s) {
					t.Fatalf("%s written while the frame asked", s)
				}
			}
			// the window saves a page meanwhile
			var edited *Page
			for _, p := range j.All() {
				if p.Title == days[1] {
					edited = p
				}
			}
			a.mu.Lock()
			if err := edited.SaveText("edited meanwhile\n"); err != nil {
				t.Fatal(err)
			}
			editedAt := edited.Modified
			a.mu.Unlock()

			if kind == "store" {
				// the store's files are written without the lock: a frame that holds it does not wait for them
				a.mu.Lock()
				close(release)
				deadline := time.Now().Add(5 * time.Second)
				for !stamped(days[0]) || !stamped(days[1]) || !stamped(days[2]) {
					if time.Now().After(deadline) {
						a.mu.Unlock()
						t.Fatal("the days were not written while the lock was held")
					}
					time.Sleep(5 * time.Millisecond)
				}
			} else {
				close(release)
			}
			// the window saves another page while the days are written and not yet in memory: its file has no day
			var late *Page
			var lateAt time.Time
			if kind == "store" {
				for _, p := range j.All() {
					if p.Title == days[2] {
						late = p
					}
				}
				if err := late.SaveText("saved after the days\n"); err != nil {
					t.Fatal(err)
				}
				lateAt = late.Modified
				if stamped(days[2]) {
					t.Fatal("the save after the days kept the day: the test does not reach the case")
				}
				a.mu.Unlock()
			}
			a.stamping.Wait()

			a.mu.Lock()
			for _, p := range j.All() {
				if p.Props[journalDayKey] != p.Title {
					t.Errorf("%s: the day in memory is %q", p.Title, p.Props[journalDayKey])
				}
			}
			a.mu.Unlock()
			again, err := OpenLibraryIn(fsys, root)
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range again.Collections[0].All() {
				if p.Props[journalDayKey] != p.Title {
					t.Errorf("%s: the day in the file is %q", p.Title, p.Props[journalDayKey])
				}
				text, _ := p.Text()
				if p.Title == days[1] {
					if !strings.Contains(text, "edited meanwhile") || !p.Modified.Equal(editedAt) {
						t.Errorf("the page saved meanwhile: %q, modified %v, want %v", text, p.Modified, editedAt)
					}
					continue
				}
				if late != nil && p.Title == days[2] {
					if !strings.Contains(text, "saved after the days") || !p.Modified.Equal(lateAt) {
						t.Errorf("the page saved after the days: %q, modified %v, want %v", text, p.Modified, lateAt)
					}
					continue
				}
				if !p.Modified.Equal(was[p.Title]) || !strings.Contains(text, "words of "+p.Title) {
					t.Errorf("%s: modified %v, want %v; text %q", p.Title, p.Modified, was[p.Title], text)
				}
			}
		})
	}
}
