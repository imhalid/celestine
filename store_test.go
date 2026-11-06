package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fsLibraries is a folder and a file, as a library's ground: what is true of
// one is true of the other.
func fsLibraries(t *testing.T) map[string]struct {
	fs   FS
	root string
} {
	st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return map[string]struct {
		fs   FS
		root string
	}{
		"disk":  {osFS{}, t.TempDir()},
		"store": {st, storeRoot},
	}
}

// TestFSContract: both grounds behave alike for what the library asks of them.
func TestFSContract(t *testing.T) {
	for name, g := range fsLibraries(t) {
		t.Run(name, func(t *testing.T) {
			f, root := g.fs, g.root
			j := func(s ...string) string { return filepath.Join(append([]string{root}, s...)...) }
			if err := f.Mkdir(j("a")); err != nil {
				t.Fatal(err)
			}
			if err := f.Mkdir(j("a")); !errors.Is(err, fs.ErrExist) {
				t.Fatalf("a folder made twice: %v", err)
			}
			if err := f.WriteFile(j("a", "x.md"), []byte("bir")); err != nil {
				t.Fatal(err)
			}
			if err := f.WriteFile(j("nope", "x.md"), []byte("?")); err == nil {
				t.Fatal("a file in a folder that is not there")
			}
			f.MkdirAll(j("a", "b", "c"))
			f.WriteFile(j("a", "b", "c", "y.md"), []byte("iki"))
			if got, err := f.ReadFile(j("a", "x.md")); err != nil || string(got) != "bir" {
				t.Fatalf("read: %q %v", got, err)
			}
			if _, err := f.ReadFile(j("a", "yok.md")); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("a file that is not there: %v", err)
			}
			es, _ := f.ReadDir(j("a"))
			if len(es) != 2 || es[0].Name() != "b" || !es[0].IsDir() || es[1].Name() != "x.md" || es[1].IsDir() {
				t.Fatalf("listing: %v", es)
			}
			if info, err := f.Stat(j("a", "x.md")); err != nil || info.IsDir() || info.ModTime().IsZero() {
				t.Fatalf("stat: %v %v", info, err)
			}
			// a folder moves with all that is in it; not onto what is there
			f.Mkdir(j("z"))
			if err := f.Rename(j("a"), j("z")); err == nil && name == "store" {
				t.Fatal("a rename onto a folder")
			}
			f.Remove(j("z"))
			if err := f.Rename(j("a"), j("z")); err != nil {
				t.Fatal(err)
			}
			if got, err := f.ReadFile(j("z", "b", "c", "y.md")); err != nil || string(got) != "iki" {
				t.Fatalf("after the move: %q %v", got, err)
			}
			if _, err := f.Stat(j("a")); err == nil {
				t.Fatal("still where it was")
			}
			if err := f.Remove(j("z")); err == nil {
				t.Fatal("a folder with things in it removed")
			}
			if err := f.RemoveAll(j("z")); err != nil {
				t.Fatal(err)
			}
			if _, err := f.Stat(j("z", "x.md")); err == nil {
				t.Fatal("not removed")
			}
			// a sibling with a longer name is not under it
			f.Mkdir(j("p"))
			f.Mkdir(j("pq"))
			f.WriteFile(j("pq", "k.md"), []byte("k"))
			f.RemoveAll(j("p"))
			if _, err := f.ReadFile(j("pq", "k.md")); err != nil {
				t.Fatalf("pq went with p: %v", err)
			}
		})
	}
}

// TestLibraryInAFile: the library's whole life - pages, order, moves, the
// trash and the way back - in a file, read again from it.
func TestLibraryInAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "k.db")
	st, err := OpenStore(file)
	if err != nil {
		t.Fatal(err)
	}
	lib, err := OpenLibraryIn(st, storeRoot)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("Notlar")
	a, _ := c.NewPage(nil, "Birinci")
	b, _ := c.NewPage(nil, "İkinci")
	a.SaveText("merhaba\n")
	if err := lib.MovePage(b, c, a, 0); err != nil {
		t.Fatal(err)
	}
	other, _ := lib.NewCollection("Öteki")
	c2, _ := other.NewPage(nil, "Gezgin")
	if err := lib.MovePage(c2, c, nil, 0); err != nil {
		t.Fatal(err)
	}
	item, err := lib.TrashPage(a) // with its sub-page
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Trash()) != 1 {
		t.Fatal("not in the trash")
	}
	if _, p, err := lib.Restore(item); err != nil || p.Title != "Birinci" || len(p.Children) != 1 {
		t.Fatalf("restored: %v %+v", err, p)
	}
	st.Close()

	st, err = OpenStore(file) // read again from the file
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	again, err := OpenLibraryIn(st, storeRoot)
	if err != nil {
		t.Fatal(err)
	}
	var got *Collection
	for _, x := range again.Collections {
		if x.Name == "Notlar" {
			got = x
		}
	}
	if got == nil || len(got.Pages) != 2 {
		t.Fatalf("read back: %+v", again.Collections)
	}
	var first *Page
	for _, p := range got.Pages {
		if p.Title == "Birinci" {
			first = p
		}
	}
	if body, _ := first.Text(); first == nil || body != "merhaba\n" || len(first.Children) != 1 || first.Children[0].Title != "İkinci" {
		t.Fatalf("the page: %+v", first)
	}
}

// TestStoreSettings: the settings, token and all, are kept in the file.
func TestStoreSettings(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, _ := newTestApp(t, "x")
	a.settingsStore = st
	a.loadSettings()
	token := a.settings.Token
	a.settings.Caret = 1
	a.saveSettings()
	b, _ := newTestApp(t, "x")
	b.settingsStore = st
	b.loadSettings()
	if token == "" || b.settings.Token != token || b.settings.Caret != 1 {
		t.Fatalf("settings: %+v", b.settings)
	}
}

// TestFolderIntoFile: a folder library is taken into a file whole, and an
// export gives the same folder back.
func TestFolderIntoFile(t *testing.T) {
	dir := t.TempDir()
	lib, _ := OpenLibrary(dir)
	c, _ := lib.NewCollection("Eski")
	p, _ := c.NewPage(nil, "Sayfa")
	p.SaveText("içerik\n")
	if !isLibraryFolder(dir) {
		t.Fatal("not a library folder")
	}
	st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := copyTree(osFS{}, dir, st, storeRoot); err != nil {
		t.Fatal(err)
	}
	in, err := OpenLibraryIn(st, storeRoot)
	if err != nil || len(in.Collections) != 1 || len(in.Collections[0].Pages) != 1 {
		t.Fatalf("in the file: %v %+v", err, in.Collections)
	}
	out := filepath.Join(t.TempDir(), "md")
	if err := copyTree(st, storeRoot, osFS{}, out); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(out, "eski", "sayfa", pageFileName))
	if err != nil || !strings.Contains(string(src), "içerik") {
		t.Fatalf("exported: %q %v", src, err)
	}
}

// One odd entry never stops the copy: a link back up the tree, a link to
// nothing, and a link to a file (which is copied as the file).
func TestCopyTreeSkipsLinks(t *testing.T) {
	src := t.TempDir()
	page := filepath.Join(src, "koleksiyon", "sayfa")
	if err := os.MkdirAll(page, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		filepath.Join(src, "koleksiyon", collectionFile): `{"name":"K","subPages":true}`,
		filepath.Join(page, pageFileName):                `{"meta":{"title":"S"},"body":[{"r":[{"t":"metin"}]}]}`,
		filepath.Join(page, "gercek.txt"):                "dosya",
	} {
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for link, target := range map[string]string{
		"dongu":   ".",             // to its own folder: would loop
		"yukari":  "../..",         // up the tree
		"yok":     "olmayan-dosya", // to nothing
		"dosyaya": "gercek.txt",    // to a file
	} {
		if err := os.Symlink(target, filepath.Join(page, link)); err != nil {
			t.Skip("no symlinks here:", err)
		}
	}
	st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := copyTree(osFS{}, src, st, storeRoot); err != nil {
		t.Fatal(err)
	}
	if b, err := st.ReadFile(filepath.Join(storeRoot, "koleksiyon", "sayfa", "dosyaya")); err != nil || string(b) != "dosya" {
		t.Fatalf("a link to a file is the file: %q %v", b, err)
	}
	for _, gone := range []string{"dongu", "yukari", "yok"} {
		if _, err := st.Stat(filepath.Join(storeRoot, "koleksiyon", "sayfa", gone)); err == nil {
			t.Errorf("%s was copied", gone)
		}
	}
}
