package main

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/i18n"
)

// Dates and times in the window's language, from MyGo's i18n package: the names of the months and the days,
// relative times and the day weeks start on come from CLDR's data by lang, not from lists of the app's own (#302).

// monthName is month m's name: whole ("March") or abbreviated ("Mar"), as it stands inside a date.
func monthName(m time.Month, short bool) string {
	w := i18n.Wide
	if short {
		w = i18n.Abbreviated
	}
	return i18n.Months(lang, w, false)[m-1]
}

// dayName is weekday d's name: whole ("Friday") or abbreviated ("Fri"), as it stands inside a date.
func dayName(d time.Weekday, short bool) string {
	w := i18n.Wide
	if short {
		w = i18n.Abbreviated
	}
	return i18n.Weekdays(lang, w, false)[d]
}

// firstWeekday is the day the window's weeks start on: Monday in Turkish, Sunday in English.
func firstWeekday() time.Weekday { return i18n.FirstWeekday(lang) }

// weekIndex is t's place in its week, 0 on the week's first day.
func weekIndex(t time.Time) int { return (int(t.Weekday()) - int(firstWeekday()) + 7) % 7 }

// weekStart is the first day of t's week, at t's time of day.
func weekStart(t time.Time) time.Time { return t.AddDate(0, 0, -weekIndex(t)) }

// weekdayHeads are the days of the week in the window's order, first day first, for a calendar's heading:
// abbreviated ("Mon"), or narrow ("M").
func weekdayHeads(narrow bool) [7]string {
	w := i18n.Abbreviated
	if narrow {
		w = i18n.Narrow
	}
	names := i18n.Weekdays(lang, w, true)
	var out [7]string
	for i := range out {
		out[i] = names[(int(firstWeekday())+i)%7]
	}
	return out
}

// shortDate is t's day, month and year, the month abbreviated: "2 Mar 2026", "Mar 2, 2026".
func shortDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return i18n.FormatDate(lang, t, i18n.DateOptions{Skeleton: "yMMMd"})
}

// dayMonth is t's day and month, the month abbreviated: "2 Mar", "Mar 2".
func dayMonth(t time.Time) string {
	return i18n.FormatDate(lang, t, i18n.DateOptions{Skeleton: "MMMd"})
}

// monthYear is t's month and year: "March 2026", or "Mar 2026" when short.
func monthYear(t time.Time, short bool) string {
	sk := "yMMMM"
	if short {
		sk = "yMMM"
	}
	return i18n.FormatDate(lang, t, i18n.DateOptions{Skeleton: sk})
}

// relative is n units from now, in the past when n is negative: "5m ago", "yesterday", "5 dk. önce", "dün".
func relative(n int, unit i18n.RelativeTimeUnit) string {
	return i18n.FormatRelativeTime(lang, float64(n), unit, i18n.RelativeTimeOptions{Style: i18n.RelativeNarrow, Auto: true})
}

// ago is how long ago t was, as a list says it: in minutes and hours today, in days this week, as a date before.
func ago(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return relative(0, i18n.RelativeSecond)
	case d < time.Hour:
		return relative(-int(d.Minutes()), i18n.RelativeMinute)
	case d < 24*time.Hour && sameDay(t, now):
		return relative(-int(d.Hours()), i18n.RelativeHour)
	case d < 7*24*time.Hour || sameDay(t, now.AddDate(0, 0, -1)):
		return relative(-daysBetween(t, now), i18n.RelativeDay)
	}
	return shortDate(t)
}

// dateGroup is the heading a list puts t under.
func dateGroup(t, now time.Time) string {
	switch {
	case sameDay(t, now) || t.After(now): // a clock ahead, a file touched elsewhere: today
		return T("Bugün")
	case sameDay(t, now.AddDate(0, 0, -1)):
		return T("Dün")
	case now.Sub(t) < 7*24*time.Hour:
		return T("Son 7 gün")
	case now.Sub(t) < 30*24*time.Hour:
		return T("Son 30 gün")
	case t.Year() == now.Year():
		return monthYear(t, true)
	}
	return strconv.Itoa(t.Year())
}

// duration says seconds as "2 h 5 min" or "40 s", "2 sa 5 dk" or "40 sn" in Turkish.
func duration(secs int) string {
	switch {
	case secs >= 3600:
		return strconv.Itoa(secs/3600) + " " + T("sa") + " " + strconv.Itoa(secs%3600/60) + " " + T("dk")
	case secs >= 60:
		return strconv.Itoa(secs/60) + " " + T("dk")
	}
	return strconv.Itoa(secs) + " " + T("sn")
}

// daysBetween is how many calendar days lie from a to b.
func daysBetween(a, b time.Time) int {
	day := func(t time.Time) time.Time { y, m, d := t.Date(); return time.Date(y, m, d, 12, 0, 0, 0, time.UTC) }
	return int(day(b).Sub(day(a)).Hours()+1) / 24
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// dateTime is t's day, month and time, its year too when withYear: "2 Mar 15:04", "Mar 2, 3:04 PM".
func dateTime(t time.Time, withYear bool) string {
	sk := "MMMdjm"
	if withYear {
		sk = "yMMMdjm"
	}
	return i18n.FormatDate(lang, t, i18n.DateOptions{Skeleton: sk})
}

// weekdayDate is t's weekday, day, month and year: "Pzt 2 Mar 2026", "Mon, Mar 2, 2026".
func weekdayDate(t time.Time) string {
	return i18n.FormatDate(lang, t, i18n.DateOptions{Skeleton: "yMMMEd"})
}
