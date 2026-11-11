package main

import (
	"encoding/json"
	"github.com/imhalid/celestine/locales"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A collection's banner: a picture becomes a file in its folder, named in collection.json, kept through its settings
// and drawn across the top of its page; another takes its place, the old file going; removed, its file goes too.
func TestCollectionBanner(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	if err := a.setBanner(coll, []byte("not a picture")); err == nil {
		t.Fatal("not a picture became a banner")
	}
	if err := a.setBanner(coll, pngBytes(40, 20)); err != nil {
		t.Fatal(err)
	}
	first := coll.Banner
	if _, err := os.Stat(filepath.Join(coll.Dir, first)); err != nil {
		t.Fatalf("no banner file: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(coll.Dir, CollectionFile))
	var saved Collection
	json.Unmarshal(data, &saved)
	if saved.Banner != first {
		t.Fatalf("collection.json: %s", data)
	}
	tt.Advance(300 * time.Millisecond)
	if r, ok := tt.Find(locales.PageBanner()); !ok || r.H != bannerH {
		t.Fatalf("no banner on the page: %+v %v", r, ok)
	}
	a.openCollSettings(coll, 0) // the settings keep it
	a.saveCollSettings()
	if coll.Banner != first {
		t.Fatalf("the settings dropped the banner: %q", coll.Banner)
	}
	a.clock = func() time.Time { return time.Date(2026, 11, 5, 9, 0, 0, 0, time.Local) }
	if err := a.setBanner(coll, pngBytes(30, 30)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(coll.Dir, first)); !os.IsNotExist(err) || coll.Banner == first {
		t.Fatalf("the old banner's file stays: %v", err)
	}
	second := coll.Banner
	a.removeBanner(coll)
	if _, err := os.Stat(filepath.Join(coll.Dir, second)); !os.IsNotExist(err) || coll.Banner != "" {
		t.Fatalf("the removed banner's file stays: %v %q", err, coll.Banner)
	}
	again, _ := OpenLibrary(a.lib.Root)
	if again.Collections[0].Banner != "" || len(again.Collections[0].All()) != 4 {
		t.Fatal("the library read again has a banner, or a file of it taken for a page")
	}
}
