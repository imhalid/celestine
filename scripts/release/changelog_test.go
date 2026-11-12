package main

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// TestCleanTitle takes the issue numbers off a title, splits off its area, and leaves internal work out.
func TestCleanTitle(t *testing.T) {
	for _, c := range []struct{ in, area, text string }{
		{"Settings: a plainer sync description, and no full list of shortcuts (#412, #413)", "Settings", "a plainer sync description, and no full list of shortcuts"},
		{"Sync for folder libraries too", "", "Sync for folder libraries too"},
		{"Page history: Restore sweeps a light down the page (#316)", "Page history", "Restore sweeps a light down the page"},
		{"Sync in seconds: the server's change counter (#99)", "", "Sync in seconds: the server's change counter"},
		{"Tables: fixes owner/repo#12 and #13.", "Tables", "fixes and"},
		{"Tests: the idle test's page has one cover (#318)", "", ""},
		{"CI: the nightly run reports what it found (#247)", "", ""},
		{"Deps: golang.org/x/net v0.62.0, for GO-2026-6860 (#300)", "", ""},
		{"Format palette.go (#186)", "", ""},
		{"Ryudit becomes Celestine (#321)", "", ""},
		{"TestTimeLapse no longer depends on the time of day (#48)", "", ""},
		{"Auto-cover: its moving parts wrap positions without math.Mod (#204)", "", ""},
		{"Updates: the server at dl.example.halid.dev (#231)", "", ""},
		{"Merge branch 'x'", "", ""},
	} {
		area, text := cleanTitle(c.in)
		if area != c.area || text != c.text {
			t.Errorf("cleanTitle(%q) = %q, %q; want %q, %q", c.in, area, text, c.area, c.text)
		}
	}
}

// TestSortChanges parts the titles into the new and the fixed, keeps their order, and shows a title once.
func TestSortChanges(t *testing.T) {
	features, fixes := sortChanges([]string{
		"Sync: one (#1)",
		"A headline of its own (#2)",
		"Tables: the wheel no longer scrolls the cell (#3)",
		"Tests: hidden (#5)",
		"Sync: one (#6)",
		"Pages: a body that is not JSON is refused (#7)",
		"open_page takes a path (#8)",
	})
	wantF := []change{{"Sync", "One"}, {"", "A headline of its own"}, {"", "open_page takes a path"}}
	wantX := []change{{"Tables", "The wheel no longer scrolls the cell"}, {"Pages", "A body that is not JSON is refused"}}
	if !reflect.DeepEqual(features, wantF) || !reflect.DeepEqual(fixes, wantX) {
		t.Errorf("sortChanges = %+v, %+v\nwant %+v, %+v", features, fixes, wantF, wantX)
	}
}

// TestMadeTitle names a release by its biggest areas.
func TestMadeTitle(t *testing.T) {
	for _, c := range []struct {
		in   []change
		want string
	}{
		{nil, "Smaller fixes and improvements"},
		{[]change{{"", "x"}}, "Improvements"},
		{[]change{{"Sync", "a"}}, "Sync"},
		{[]change{{"Sync", "a"}, {"Tables", "b"}, {"Tables", "c"}}, "Tables and Sync"},
		{[]change{{"A", "1"}, {"B", "1"}, {"B", "2"}, {"C", "1"}, {"C", "2"}, {"C", "3"}, {"D", "1"}}, "C, B and A"},
	} {
		if got := madeTitle(c.in); got != c.want {
			t.Errorf("madeTitle(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestBuildChangelog orders the versions newest first, marks the prereleases and the first release, and gives each
// release the titles since the tag before it.
func TestBuildChangelog(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 11, d, 12, 0, 0, 0, time.UTC) }
	tags := []tag{
		{Name: "v1.0.0", Date: day(5), Subject: "v1.0.0"}, {Name: "v0.9.0", Date: day(1)}, {Name: "v1.0.0-beta.1", Date: day(3)},
		{Name: "nightly", Date: day(4)}, {Name: "v1.0.1", Date: day(6), Subject: "Quieter sync", Body: "Sync asks less.\n"},
	}
	asked := map[string]bool{}
	titles := func(prev, cur string) ([]string, error) {
		asked[prev+".."+cur] = true
		return []string{"Sync: in " + cur + " (#1)"}, nil
	}
	c, err := buildChangelog(tags, titles, day(7))
	if err != nil {
		t.Fatal(err)
	}
	var versions []string
	for _, r := range c.Releases {
		versions = append(versions, r.Version)
	}
	if want := []string{"v1.0.1", "v1.0.0", "v1.0.0-beta.1", "v0.9.0"}; !reflect.DeepEqual(versions, want) {
		t.Fatalf("versions %v, want %v", versions, want)
	}
	for _, r := range []string{"v0.9.0..v1.0.0-beta.1", "v1.0.0-beta.1..v1.0.0", "v1.0.0..v1.0.1"} {
		if !asked[r] {
			t.Errorf("the titles of %s were not asked for", r)
		}
	}
	first, beta, one, stable := c.Releases[3], c.Releases[2], c.Releases[1], c.Releases[0]
	if !first.First || !first.Prerelease || first.Title != "The first release" || len(first.Features) != 0 {
		t.Errorf("v0.9.0 = %+v, want the first release, a prerelease, no changes", first)
	}
	if !beta.Prerelease || stable.Prerelease || stable.Date != "2026-11-06" {
		t.Errorf("beta %+v, stable %+v", beta, stable)
	}
	// the annotation names a release; one that repeats the tag's name does not
	if stable.Title != "Quieter sync" || stable.Summary != "Sync asks less." || one.Title != "Sync" || one.Summary != "" {
		t.Errorf("titles %q %q, %q %q", stable.Title, stable.Summary, one.Title, one.Summary)
	}
	if want := []change{{"Sync", "In v1.0.1"}}; !reflect.DeepEqual(stable.Features, want) {
		t.Errorf("v1.0.1's changes %+v, want %+v", stable.Features, want)
	}
	// a release with no changes the page shows still has lists, not null, for the page
	f, x := sortChanges([]string{"CI: x (#1)"})
	b, _ := json.Marshal(release{Features: f, Fixes: x})
	if string(b) != `{"version":"","date":"","prerelease":false,"title":"","features":[],"fixes":[]}` {
		t.Errorf("an empty release is %s", b)
	}
}
