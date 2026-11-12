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

// TestGroupTitles puts the titles with no area and the areas of one change first, then the areas by size.
func TestGroupTitles(t *testing.T) {
	got := groupTitles([]string{
		"Sync: one (#1)",
		"A headline of its own (#2)",
		"Tables: alpha (#3)",
		"Sync: two (#4)",
		"Tests: hidden (#5)",
		"Sync: one (#6)",
		"Paste: Markdown comes in as blocks (#7)",
		"Tables: beta (#8)",
		"Tables: gamma (#9)",
	})
	want := []group{
		{Area: "", Items: []string{"A headline of its own", "Paste: Markdown comes in as blocks"}},
		{Area: "Tables", Items: []string{"Alpha", "Beta", "Gamma"}},
		{Area: "Sync", Items: []string{"One", "Two"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("groupTitles = %+v\nwant %+v", got, want)
	}
}

// TestBuildChangelog orders the versions newest first, marks the prereleases and the first release, and gives each
// release the titles since the tag before it.
func TestBuildChangelog(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 11, d, 12, 0, 0, 0, time.UTC) }
	tags := []tag{
		{"v1.0.0", day(5)}, {"v0.9.0", day(1)}, {"v1.0.0-beta.1", day(3)}, {"nightly", day(4)}, {"v1.0.1", day(6)},
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
	first, beta, stable := c.Releases[3], c.Releases[2], c.Releases[0]
	if !first.First || !first.Prerelease || len(first.Groups) != 0 {
		t.Errorf("v0.9.0 = %+v, want the first release, a prerelease, no groups", first)
	}
	if !beta.Prerelease || stable.Prerelease || stable.Date != "2026-11-06" {
		t.Errorf("beta %+v, stable %+v", beta, stable)
	}
	if want := []group{{Area: "", Items: []string{"Sync: in v1.0.1"}}}; !reflect.DeepEqual(stable.Groups, want) {
		t.Errorf("v1.0.1's groups %+v, want %+v", stable.Groups, want)
	}
	// a release with no changes the page shows still has a list, not null, for the page's script
	b, _ := json.Marshal(release{Groups: groupTitles([]string{"CI: x (#1)"})})
	if string(b) != `{"version":"","date":"","prerelease":false,"groups":[]}` {
		t.Errorf("an empty release is %s", b)
	}
}
