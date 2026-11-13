package main

import (
	"testing"
	"time"
)

// The English window says its times, dates and weeks in English, with no Turkish word in them, and its week
// starts on Sunday; the Turkish one on Monday (#302).
func TestDatesInEnglish(t *testing.T) {
	withLang(t, "en")
	now := time.Date(2026, 11, 4, 15, 0, 0, 0, time.Local) // a Wednesday
	for _, c := range []struct {
		t          time.Time
		ago, group string
	}{
		{now.Add(-30 * time.Second), "now", "Today"},
		{now.Add(-5 * time.Minute), "5m ago", "Today"},
		{now.Add(-3 * time.Hour), "3h ago", "Today"},
		{now.Add(-20 * time.Hour), "yesterday", "Yesterday"},
		{now.Add(-4 * 24 * time.Hour), "4d ago", "Last 7 days"},
		{time.Date(2026, 10, 10, 9, 0, 0, 0, time.Local), "Oct 10, 2026", "Last 30 days"},
		{time.Date(2026, 3, 2, 9, 0, 0, 0, time.Local), "Mar 2, 2026", "Mar 2026"},
	} {
		if got := ago(c.t, now); got != c.ago {
			t.Errorf("ago(%v) = %q, want %q", c.t, got, c.ago)
		}
		if got := dateGroup(c.t, now); got != c.group {
			t.Errorf("dateGroup(%v) = %q, want %q", c.t, got, c.group)
		}
	}
	if got := formatDay("D MMMM YYYY, dddd", time.Date(2026, 11, 6, 9, 0, 0, 0, time.Local)); got != "6 November 2026, Friday" {
		t.Errorf("formatDay: %q", got)
	}
	if got := duration(2*3600 + 5*60); got != "2 h 5 min" {
		t.Errorf("duration: %q", got)
	}
	if got := weekdayHeads(false); got[0] != "Sun" || got[6] != "Sat" {
		t.Errorf("week heads: %v", got)
	}
	if got := weekStart(now); got.Weekday() != time.Sunday || got.Day() != 1 {
		t.Errorf("week start: %v", got)
	}

	withLang(t, "tr")
	if got := weekdayHeads(false); got[0] != "Pzt" || got[6] != "Paz" {
		t.Errorf("Turkish week heads: %v", got)
	}
	if got := weekStart(now); got.Weekday() != time.Monday || got.Day() != 2 {
		t.Errorf("Turkish week start: %v", got)
	}
	if got := duration(40); got != "40 sn" {
		t.Errorf("Turkish duration: %q", got)
	}
}

// TestFormatDayEscape checks backslash literal escaping in custom journal formats without changing [YY].
func TestFormatDayEscape(t *testing.T) {
	withLang(t, "en")
	d := time.Date(2026, 11, 6, 9, 0, 0, 0, time.Local)
	for _, tc := range []struct {
		layout string
		want   string
	}{
		{`\Day D of \M\M\M YYYY`, "Day 6 of MMM 2026"},
		{`YYYY-\M\M-DD`, "2026-MM-06"},
		{`\D=D \M=M \Y\Y=YY \d=ddd \\`, `D=6 M=11 YY=26 d=Fri \`},
		{`[YY]`, "[26]"},
		{`\Ş YYYY\`, `Ş 2026\`},
	} {
		if got := formatDay(tc.layout, d); got != tc.want {
			t.Errorf("formatDay(%q) = %q, want %q", tc.layout, got, tc.want)
		}
	}
}
