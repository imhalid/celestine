package main

import (
	"strconv"
	"strings"
	"time"
)

// Days by name, as they are said: bugün, yarın, dün and today, tomorrow, yesterday; a weekday (cuma, friday:
// the next one after today), with gelecek/next for the one of next week, bu/this for this week's and
// geçen/last for the one before today; "3 gün sonra", "in 3 days", "3 days from now", "2 hafta önce", "2 weeks ago"; gelecek
// hafta/ay, next week/month (a week, a month from today), geçen hafta, last week. A day as 2026-11-10 is read
// too. The date menus (slash.go), the search's created: and modified: (searchquery.go) and the journal's
// Tarihe git (journal.go) read days with it.

// weekdayNames are the days of the week by name, folded, in Turkish and English, short ones too.
var weekdayNames = map[string]time.Weekday{
	"pazartesi": time.Monday, "sali": time.Tuesday, "carsamba": time.Wednesday, "persembe": time.Thursday,
	"cuma": time.Friday, "cumartesi": time.Saturday, "pazar": time.Sunday,
	"monday": time.Monday, "tuesday": time.Tuesday, "wednesday": time.Wednesday, "thursday": time.Thursday,
	"friday": time.Friday, "saturday": time.Saturday, "sunday": time.Sunday,
	"mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday, "thu": time.Thursday, "fri": time.Friday,
	"sat": time.Saturday, "sun": time.Sunday,
}

// dayUnits are the units a count of days is said in, folded, as days: a month is counted by the calendar.
var dayUnits = map[string]string{
	"gun": "day", "day": "day", "days": "day", "hafta": "week", "week": "week", "weeks": "week",
	"ay": "month", "month": "month", "months": "month", "yil": "year", "year": "year", "years": "year",
}

// parseNaturalDay is the day q names, counted from now's, at its start; false when q names none.
func parseNaturalDay(q string, now time.Time) (time.Time, bool) {
	today := startOfDay(now)
	s := strings.Join(strings.Fields(fold(q)), " ")
	if s == "" {
		return time.Time{}, false
	}
	if d, err := time.ParseInLocation("2006-01-02", s, now.Location()); err == nil {
		return d, true
	}
	switch s {
	case "bugun", "today", "now", "simdi":
		return today, true
	case "yarin", "tomorrow":
		return today.AddDate(0, 0, 1), true
	case "dun", "yesterday":
		return today.AddDate(0, 0, -1), true
	case "obur gun", "ertesi gun", "day after tomorrow", "the day after tomorrow":
		return today.AddDate(0, 0, 2), true
	case "gelecek hafta", "haftaya", "next week":
		return today.AddDate(0, 0, 7), true
	case "gecen hafta", "last week":
		return today.AddDate(0, 0, -7), true
	case "gelecek ay", "next month":
		return today.AddDate(0, 1, 0), true
	case "gecen ay", "last month":
		return today.AddDate(0, -1, 0), true
	}
	words := strings.Fields(s)
	// a weekday, alone or after next, this or last
	if wd, ok := weekdayNames[words[len(words)-1]]; ok && len(words) <= 2 {
		after := (int(wd) - int(today.Weekday()) + 7) % 7 // days to the next one, 0 today
		first := weekStart(today)
		inWeek := (int(wd) - int(firstWeekday()) + 7) % 7 // the week's first day 0
		if len(words) == 1 {
			if after == 0 {
				after = 7
			}
			return today.AddDate(0, 0, after), true
		}
		switch words[0] {
		case "gelecek", "next", "haftaya":
			return first.AddDate(0, 0, 7+inWeek), true
		case "bu", "this":
			return first.AddDate(0, 0, inWeek), true
		case "gecen", "last", "onceki":
			before := (int(today.Weekday()) - int(wd) + 7) % 7
			if before == 0 {
				before = 7
			}
			return today.AddDate(0, 0, -before), true
		}
		return time.Time{}, false
	}
	// a count: 3 gün sonra, 2 hafta önce; in 3 days, 3 days later, 2 weeks ago
	sign, n, unit := 0, 0, ""
	switch {
	case len(words) == 3 && (words[2] == "sonra" || words[2] == "later"), len(words) == 4 && words[2] == "from" && words[3] == "now":
		sign, unit = 1, words[1]
		n, _ = strconv.Atoi(words[0])
	case len(words) == 3 && (words[2] == "once" || words[2] == "ago"):
		sign, unit = -1, words[1]
		n, _ = strconv.Atoi(words[0])
	case len(words) == 3 && words[0] == "in":
		sign, unit = 1, words[2]
		n, _ = strconv.Atoi(words[1])
	}
	if u, ok := dayUnits[unit]; ok && n > 0 && n < 10000 {
		n *= sign
		switch u {
		case "week":
			return today.AddDate(0, 0, 7*n), true
		case "month":
			return today.AddDate(0, n, 0), true
		case "year":
			return today.AddDate(n, 0, 0), true
		}
		return today.AddDate(0, 0, n), true
	}
	return time.Time{}, false
}

// dayChoices are the days a date menu offers for what is typed, q: the days by name that match it, and the day
// q itself names, first, when it is none of them.
func (a *app) dayChoices(q string) []namedDay {
	today := a.clock()
	tq := strings.TrimSpace(q)
	var out []namedDay
	for _, d := range []namedDay{{"common_today", today}, {"dates_tomorrow", today.AddDate(0, 0, 1)}, {"common_yesterday", today.AddDate(0, 0, -1)}, {"dates_nextWeek", today.AddDate(0, 0, 7)}} {
		if q != "" && match(q, names(d.name)) < 0 && !strings.HasPrefix(d.d.Format("2006-01-02"), tq) {
			continue
		}
		out = append(out, d)
	}
	if d, ok := parseNaturalDay(tq, today); ok {
		for _, o := range out {
			if sameDay(o.d, d) {
				return out
			}
		}
		name := shortDate(d)
		if _, err := time.Parse("2006-01-02", tq); err != nil { // as it was said, its first letter large
			r := []rune(tq)
			name = strings.ToUpper(string(r[:1])) + string(r[1:])
		}
		out = append([]namedDay{{name, d}}, out...)
	}
	return out
}
