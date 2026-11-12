package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// changelog is what the site's /changelog page reads (cloudflare/sync/changelog.js): every release, newest first.
type changelog struct {
	Generated string    `json:"generated"`
	Releases  []release `json:"releases"`
}

// release is one version on the page: its number, the day it was tagged, whether it is a prerelease, and its
// changes in groups. First marks the first release, whose changes are not listed (they are the whole app).
type release struct {
	Version    string  `json:"version"`
	Date       string  `json:"date"`
	Prerelease bool    `json:"prerelease"`
	First      bool    `json:"first,omitempty"`
	Groups     []group `json:"groups"`
}

// group is the changes of one area ("Sync", "Tables"), as the commit titles name it before their colon. A title
// with no area is in the group with an empty Area, shown first.
type group struct {
	Area  string   `json:"area"`
	Items []string `json:"items"`
}

// tag is a version tag and when it was made.
type tag struct {
	Name string
	Date time.Time
}

// versionTag is a release tag: v1.2.3, or v1.2.0-beta.1.
var versionTag = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)(-[0-9A-Za-z.-]+)?$`)

// issueRef is a reference to an issue or a pull request: "(#412, #413)" at the end of a title, a bare "#12", or
// "owner/repo#34". The page is for people who cannot open them.
var (
	issueParens = regexp.MustCompile(`\s*\(\s*(?:[\w.-]+/[\w.-]+)?#\d+(?:\s*,\s*(?:[\w.-]+/[\w.-]+)?#\d+)*\s*\)`)
	issueBare   = regexp.MustCompile(`\s*(?:[\w.-]+/[\w.-]+)?#\d+\b`)
)

// internalAreas are the areas of work a user of the app does not see: the tests, the build, the servers, the
// repository's own notes. Their commits stay off the page.
var internalAreas = map[string]bool{
	"tests": true, "test": true, "perf test": true, "ci": true, "deps": true, "docs": true, "claude.md": true,
	"format": true, "release workflow": true, "releases": true, "release": true, "update server": true,
	"sync worker": true, "site": true, "store": true, "web store": true, "body": true, "build": true, "scripts": true,
	"nightly": true, "mygo": true, "ryugo": true, "readme": true, "android": true, "web": true, "admin": true,
}

// internalWords mark a title as internal wherever they are in it: names of the code, its tools and its servers,
// and the app's old name.
var internalWords = regexp.MustCompile(`(?i)\b(ryudit|mygo|ryugo|egoist|worker|cloudflare|r2|wrangler|halid\.dev|go\.mod|gofmt|govulncheck|fuzz|tests?|benchmarks?|nightly|workflow|ci|readme|claude\.md|fable|deploy|wasm|GO-\d{4}-\d+)\b|\.go\b|\bTest[A-Z]\w*|\b[a-z]\w*\.[A-Z]\w*|` + "`")

// parseVersion gives a version tag's numbers, and whether it is a prerelease: a v0.x, or one with a "-".
func parseVersion(name string) (nums [3]int, pre string, ok bool) {
	m := versionTag.FindStringSubmatch(name)
	if m == nil {
		return nums, "", false
	}
	for i := range nums {
		nums[i], _ = strconv.Atoi(m[i+1])
	}
	return nums, m[4], true
}

func isPrerelease(name string) bool {
	nums, pre, _ := parseVersion(name)
	return nums[0] == 0 || pre != ""
}

// sortTags puts the version tags in the order of their versions, oldest first, and drops any other tag. A
// prerelease comes before its release (v1.2.0-beta.1 before v1.2.0).
func sortTags(tags []tag) []tag {
	var out []tag
	for _, t := range tags {
		if _, _, ok := parseVersion(t.Name); ok {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, ap, _ := parseVersion(out[i].Name)
		b, bp, _ := parseVersion(out[j].Name)
		if a != b {
			for k := range a {
				if a[k] != b[k] {
					return a[k] < b[k]
				}
			}
		}
		if (ap == "") != (bp == "") {
			return ap != ""
		}
		return ap < bp
	})
	return out
}

// cleanTitle is a commit title as the page shows it, without its issue numbers, and its area when it names one, or
// "" when it is internal work.
func cleanTitle(s string) (area, text string) {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "Merge ") || strings.HasPrefix(s, "Revert ") {
		return "", ""
	}
	s = issueParens.ReplaceAllString(s, "")
	s = strings.TrimSpace(issueBare.ReplaceAllString(s, ""))
	if internalWords.MatchString(s) {
		return "", ""
	}
	if i := strings.Index(s, ": "); i > 0 {
		head := strings.TrimSpace(s[:i])
		if internalAreas[strings.ToLower(head)] {
			return "", ""
		}
		// an area is a name of one or two words ("Sync", "Page history"); a longer head is a headline of its own
		// ("Sync in seconds: …"), and the title stays whole
		if len(strings.Fields(head)) <= 2 && !strings.ContainsAny(head, ",") {
			area, s = head, strings.TrimSpace(s[i+2:])
		}
	}
	s = strings.TrimRight(s, " .")
	if s == "" {
		return "", ""
	}
	return area, s
}

// groupTitles groups a release's commit titles by area: the titles with no area first, then the areas with the most
// changes, and in each group the titles in the order they were made (the oldest first). An area with one change is
// not a group of its own: its title joins the first group whole ("Gear menu: its choices …"). A title seen twice is
// shown once.
func groupTitles(titles []string) []group {
	byArea := map[string]*group{}
	var order []string
	seen := map[string]bool{}
	for _, t := range titles {
		area, text := cleanTitle(t)
		if text == "" || seen[area+"\x00"+text] {
			continue
		}
		seen[area+"\x00"+text] = true
		g := byArea[area]
		if g == nil {
			g = &group{Area: area}
			byArea[area] = g
			order = append(order, area)
		}
		g.Items = append(g.Items, text)
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i], order[j]
		if (a == "") != (b == "") {
			return a == ""
		}
		return len(byArea[a].Items) > len(byArea[b].Items)
	})
	lead := group{}
	if g := byArea[""]; g != nil {
		for _, t := range g.Items {
			lead.Items = append(lead.Items, capital(t))
		}
	}
	var rest []group
	for _, a := range order {
		g := byArea[a]
		switch {
		case a == "":
		case len(g.Items) == 1:
			lead.Items = append(lead.Items, a+": "+g.Items[0])
		default:
			for i, t := range g.Items {
				g.Items[i] = capital(t)
			}
			rest = append(rest, *g)
		}
	}
	out := []group{}
	if len(lead.Items) > 0 {
		out = append(out, lead)
	}
	return append(out, rest...)
}

// capital gives a title a capital first letter, unless it starts with a name as the app writes it (open_page).
func capital(s string) string {
	if first, _, _ := strings.Cut(s, " "); strings.Contains(first, "_") {
		return s
	}
	r := []rune(s)
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r)
}

// buildChangelog makes the changelog from the version tags and the commit titles of each one (titles(prev, t), the
// titles since the tag before, oldest first).
func buildChangelog(tags []tag, titles func(prev, t string) ([]string, error), now time.Time) (changelog, error) {
	tags = sortTags(tags)
	c := changelog{Generated: now.UTC().Format(time.RFC3339), Releases: []release{}}
	for i, t := range tags {
		r := release{Version: t.Name, Date: t.Date.Format("2006-01-02"), Prerelease: isPrerelease(t.Name), Groups: []group{}}
		if i == 0 {
			r.First = true
		} else {
			ts, err := titles(tags[i-1].Name, t.Name)
			if err != nil {
				return c, err
			}
			r.Groups = groupTitles(ts)
		}
		c.Releases = append(c.Releases, r)
	}
	// newest first
	for i, j := 0, len(c.Releases)-1; i < j; i, j = i+1, j-1 {
		c.Releases[i], c.Releases[j] = c.Releases[j], c.Releases[i]
	}
	return c, nil
}

// gitTags lists the repository's tags and when each was made (an annotated tag's own date, a light one's commit's).
func gitTags() ([]tag, error) {
	out, err := exec.Command("git", "for-each-ref", "refs/tags", "--format=%(refname:short) %(creatordate:iso-strict)").Output()
	if err != nil {
		return nil, fmt.Errorf("git for-each-ref: %w", err)
	}
	var tags []tag
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name, date, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		d, err := time.Parse(time.RFC3339, date)
		if err != nil {
			return nil, fmt.Errorf("the date of %s: %w", name, err)
		}
		tags = append(tags, tag{name, d})
	}
	return tags, nil
}

// gitTitles is the commit titles between two tags, oldest first, without merges: what the release's notes list too.
func gitTitles(prev, t string) ([]string, error) {
	out, err := exec.Command("git", "log", "--no-merges", "--reverse", "--format=%s", prev+".."+t).Output()
	if err != nil {
		return nil, fmt.Errorf("git log %s..%s: %w", prev, t, err)
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return nil, nil
	}
	return strings.Split(s, "\n"), nil
}
