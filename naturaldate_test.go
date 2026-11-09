package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestParseNaturalDay(t *testing.T) {
	now := time.Date(2026, 11, 9, 15, 30, 0, 0, time.Local) // a Monday
	for q, want := range map[string]string{
		"bugün": "2026-11-09", "Today": "2026-11-09", "yarın": "2026-11-10", "tomorrow": "2026-11-10",
		"dün": "2026-11-08", "yesterday": "2026-11-08", "öbür gün": "2026-11-11",
		"cuma": "2026-11-13", "Friday": "2026-11-13", "pazartesi": "2026-11-16", "mon": "2026-11-16",
		"gelecek cuma": "2026-11-20", "next friday": "2026-11-20", "haftaya salı": "2026-11-17",
		"bu cuma": "2026-11-13", "this sunday": "2026-11-15", "geçen cuma": "2026-11-06", "last monday": "2026-11-02",
		"Çarşamba": "2026-11-11", "perşembe": "2026-11-12",
		"3 gün sonra": "2026-11-12", "in 3 days": "2026-11-12", "3 days later": "2026-11-12", "3 days from now": "2026-11-12",
		"2 hafta önce": "2026-10-26", "2 weeks ago": "2026-10-26", "1 ay sonra": "2026-12-09", "in 1 year": "2027-11-09",
		"gelecek hafta": "2026-11-16", "next week": "2026-11-16", "geçen hafta": "2026-11-02", "next month": "2026-12-09",
		"2026-12-25": "2026-12-25", "  yarın  ": "2026-11-10",
	} {
		d, ok := parseNaturalDay(q, now)
		if !ok || d.Format("2006-01-02") != want || d.Hour() != 0 {
			t.Errorf("%q: %v %v, not %s", q, d, ok, want)
		}
	}
	for _, q := range []string{"", "yar", "cuma akşamı", "3 elma sonra", "in x days", "gelecek", "-3 gün sonra", "salı gelecek"} {
		if d, ok := parseNaturalDay(q, now); ok {
			t.Errorf("%q: %v", q, d)
		}
	}
}

// The date menu takes a day said in words: what was typed, first, and the day it is.
func TestMenuNaturalDay(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	a.clock = func() time.Time { return time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local) }
	days := func(q string) []string {
		var out []string
		for _, d := range a.dayChoices(q) {
			out = append(out, d.name+"="+d.d.Format("2006-01-02"))
		}
		return out
	}
	if got := days(""); len(got) != 4 {
		t.Fatalf("named: %v", got)
	}
	if got := days("yarın"); len(got) != 1 || got[0] != "Yarın=2026-11-10" {
		t.Fatalf("a name and the same day said: %v", got)
	}
	if got := days("3 gün sonra"); len(got) != 1 || got[0] != "3 gün sonra=2026-11-12" {
		t.Fatalf("a count: %v", got)
	}
	if got := days("next friday"); len(got) != 1 || got[0] != "Next friday=2026-11-20" {
		t.Fatalf("next friday: %v", got)
	}
	// in the menu
	tt.Key(ui.Cmd, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "/")
	tt.Frame()
	a.slash.sel = 6 // Tarih ya da saat
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	typeEach(tt, "gelecek cuma")
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if md := mdOf(a.doc); !strings.Contains(md, "[[2026-11-20]]") {
		t.Fatalf("no day: %q", md)
	}
}
