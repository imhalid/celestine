package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pageOf is a page file of the blocks, each a paragraph of its text.
func pageOf(title string, texts ...string) string {
	var blocks []string
	for _, t := range texts {
		blocks = append(blocks, `{"r":[{"t":"`+t+`"}]}`)
	}
	return string(JoinPage(map[string]string{"title": title}, "["+strings.Join(blocks, ",")+"]"))
}

// mergeDevice is a device that keeps the bases of its pages, as the app's do.
func mergeDevice(t *testing.T) *device {
	d := newDevice(t)
	d.st.Bases = DirBases{Root: t.TempDir()}
	return d
}

func conflictCopies(t *testing.T, d *device) []string {
	var out []string
	filepath.WalkDir(d.root, func(p string, e os.DirEntry, err error) error {
		if err == nil && strings.Contains(p, "(çakışma ") {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// Two devices change different blocks of a page between syncs: the page is merged, with no conflict copy.
func TestSyncMergesDifferentBlocks(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	remote, err := NewEncRemote(inner, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	a, b := mergeDevice(t), mergeDevice(t)
	const page = "Not/Bir/page.json"
	a.write(page, pageOf("Bir", "one", "two", "three"))
	a.sync(remote)
	b.sync(remote)
	a.write(page, pageOf("Bir", "ONE", "two", "three"))
	b.write(page, pageOf("Bir", "one", "two", "THREE", "four"))
	a.sync(remote)
	r := b.sync(remote)
	if r.Merged != 1 || len(r.Conflicts) != 0 {
		t.Fatalf("not merged: %+v", r)
	}
	want := pageOf("Bir", "ONE", "two", "THREE", "four")
	if got := b.read(page); got != want {
		t.Fatalf("merged:\n%s\nwant\n%s", got, want)
	}
	if r := a.sync(remote); r.Pulled != 1 || a.read(page) != want {
		t.Fatalf("the merge did not come to A: %+v %s", r, a.read(page))
	}
	if r := b.sync(remote); r.Pushed+r.Pulled+r.Merged != 0 {
		t.Fatalf("a pass after the merge did something: %+v", r)
	}
	if c := append(conflictCopies(t, a), conflictCopies(t, b)...); len(c) > 0 {
		t.Fatalf("conflict copies: %v", c)
	}
	// and again from the merged base: both change other blocks once more
	a.write(page, pageOf("Bir", "ONE", "two!", "THREE", "four"))
	b.write(page, pageOf("Bir", "ONE", "two", "THREE", "four!"))
	a.sync(remote)
	if r := b.sync(remote); r.Merged != 1 {
		t.Fatalf("the second merge: %+v", r)
	}
	if got, want := b.read(page), pageOf("Bir", "ONE", "two!", "THREE", "four!"); got != want {
		t.Fatalf("merged again:\n%s\nwant\n%s", got, want)
	}
}

// The same block changed on both devices is a conflict: theirs stays, ours is kept beside, as before.
func TestSyncSameBlockConflicts(t *testing.T) {
	remote := DirRemote{Root: t.TempDir()}
	a, b := mergeDevice(t), mergeDevice(t)
	const page = "Not/Bir/page.json"
	a.write(page, pageOf("Bir", "one", "two"))
	a.sync(remote)
	b.sync(remote)
	a.write(page, pageOf("Bir", "one", "two by A"))
	b.write(page, pageOf("Bir", "one", "two by B"))
	a.sync(remote)
	r := b.sync(remote)
	if r.Merged != 0 || len(r.Conflicts) != 1 {
		t.Fatalf("not a conflict: %+v", r)
	}
	if b.read(page) != pageOf("Bir", "one", "two by A") {
		t.Fatalf("theirs did not stay: %s", b.read(page))
	}
	if c := conflictCopies(t, b); len(c) != 1 {
		t.Fatalf("conflict copies: %v", c)
	}
}

// A file that is not a page is never merged.
func TestSyncMergesOnlyPages(t *testing.T) {
	remote := DirRemote{Root: t.TempDir()}
	a, b := mergeDevice(t), mergeDevice(t)
	a.write("Not/collection.json", pageOf("x", "one", "two"))
	a.sync(remote)
	b.sync(remote)
	a.write("Not/collection.json", pageOf("x", "ONE", "two"))
	b.write("Not/collection.json", pageOf("x", "one", "TWO"))
	a.sync(remote)
	if r := b.sync(remote); r.Merged != 0 || len(r.Conflicts) != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestMergePage(t *testing.T) {
	base := pageOf("T", "a", "b", "c")
	for _, c := range []struct {
		name, mine, theirs, want string
	}{
		{"one side", pageOf("T", "a", "B", "c"), base, pageOf("T", "a", "B", "c")},
		{"both added at the end", pageOf("T", "a", "b", "c", "mine"), pageOf("T", "a", "b", "c", "theirs"), pageOf("T", "a", "b", "c", "mine", "theirs")},
		{"deleted and changed elsewhere", pageOf("T", "b", "c"), pageOf("T", "a", "b", "C"), pageOf("T", "b", "C")},
		{"changed next to each other", pageOf("T", "a", "c"), pageOf("T", "a", "b", "C"), ""}, // diff3's: neighbours clash
		{"the same change", pageOf("T", "a", "X", "c"), pageOf("T", "a", "X", "c"), pageOf("T", "a", "X", "c")},
		{"title and body", pageOf("U", "a", "b", "c"), pageOf("T", "a", "b", "c", "d"), pageOf("U", "a", "b", "c", "d")},
		{"deleted and changed", pageOf("T", "a", "c"), pageOf("T", "a", "B", "c"), ""},
		{"titles", pageOf("U", "a", "b", "c"), pageOf("V", "a", "b", "c"), ""},
	} {
		got, ok := MergePage([]byte(base), []byte(c.mine), []byte(c.theirs))
		if c.want == "" {
			if ok {
				t.Errorf("%s: merged, should conflict: %s", c.name, got)
			}
			continue
		}
		if !ok || string(got) != c.want {
			t.Errorf("%s: %v %s\nwant %s", c.name, ok, got, c.want)
		}
	}
}
