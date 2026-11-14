package library

import (
	"strings"
	"testing"
)

// A cover's position is written to the page's front matter, read back, kept within its range, and left out when it is
// the middle (#524).
func TestCoverShiftRoundTrip(t *testing.T) {
	p := &Page{Title: "A", Cover: "kapak.png", CoverShift: 40}
	meta := map[string]string{"cover": "kapak.png", "covershift": "40"}
	var q Page
	q.ReadMeta(meta)
	if q.CoverShift != p.CoverShift {
		t.Fatalf("read %d, want %d", q.CoverShift, p.CoverShift)
	}
	for in, want := range map[string]int{"-250": -100, "250": 100, "x": 0, "": 0, "-30": -30} {
		var r Page
		r.ReadMeta(map[string]string{"covershift": in})
		if r.CoverShift != want {
			t.Errorf("covershift %q read as %d, want %d", in, r.CoverShift, want)
		}
	}
	var r Page
	r.ReadMeta(map[string]string{"covershift": "40"})
	r.ReadMeta(map[string]string{})
	if r.CoverShift != 0 {
		t.Errorf("a page without the field keeps %d", r.CoverShift)
	}
	if !strings.Contains(strings.Join(reserved, " "), "covershift") {
		t.Error("covershift is a property id: it must be reserved")
	}
}
