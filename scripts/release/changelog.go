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

// changelog is what the site's /changelog page reads (src/routes/changelog of the celestine-site repository): every release, newest first.
type changelog struct {
	Generated string    `json:"generated"`
	Releases  []release `json:"releases"`
}

// release is one version on the page: its number, the day it was tagged, whether it is a prerelease, a title and a
// summary, and its changes, the new and improved apart from the fixes. First marks the first release, whose changes
// are not listed (they are the whole app).
type release struct {
	Version    string   `json:"version"`
	Date       string   `json:"date"`
	Prerelease bool     `json:"prerelease"`
	First      bool     `json:"first,omitempty"`
	Title      string   `json:"title"`
	Summary    string   `json:"summary,omitempty"`
	Features   []change `json:"features"`
	Fixes      []change `json:"fixes"`
}

// change is one commit title as the page shows it: its area ("Sync", "Page history"), when the title names one
// before its colon, and what changed.
type change struct {
	Area string `json:"area,omitempty"`
	Text string `json:"text"`
}

// tag is a version tag, when it was made, and what its annotation says: the release's title on its first line and
// a summary after it (git tag -a v1.2.0 -m "Title" -m "Summary"). A light tag, or an annotation that only repeats
// the tag's name, says nothing.
type tag struct {
	Name    string
	Date    time.Time
	Subject string
	Body    string
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

// fixWords mark a change as a fix: what no longer happens, what is kept or survives now, what was wrong.
var fixWords = regexp.MustCompile(`(?i)\b(no longer|fix(es|ed)?|is refused|survives?|crash(es)?|freez(e|es|ing)|no blink|damaged|cannot be read|not dropped|not replaced|not lost|stale|wrong|broken)\b`)

// sortChanges cleans a release's commit titles and parts them into what is new or better and what was fixed, each
// in the order they were made (the oldest first). A title seen twice is shown once.
func sortChanges(titles []string) (features, fixes []change) {
	features, fixes = []change{}, []change{}
	seen := map[change]bool{}
	for _, t := range titles {
		area, text := cleanTitle(t)
		if text == "" {
			continue
		}
		c := change{area, capital(text)}
		if seen[c] {
			continue
		}
		seen[c] = true
		if fixWords.MatchString(text) {
			fixes = append(fixes, c)
		} else {
			features = append(features, c)
		}
	}
	return features, fixes
}

// madeTitle is a release's title when its tag gives none: the areas with the most changes ("Page history, Sync and
// Tables"), or with no areas named, a plain one.
func madeTitle(changes []change) string {
	count := map[string]int{}
	var order []string
	for _, c := range changes {
		if c.Area == "" {
			continue
		}
		if count[c.Area] == 0 {
			order = append(order, c.Area)
		}
		count[c.Area]++
	}
	sort.SliceStable(order, func(i, j int) bool { return count[order[i]] > count[order[j]] })
	if len(order) > 3 {
		order = order[:3]
	}
	switch len(order) {
	case 0:
		if len(changes) == 0 {
			return "Smaller fixes and improvements"
		}
		return "Improvements"
	case 1:
		return order[0]
	}
	return strings.Join(order[:len(order)-1], ", ") + " and " + order[len(order)-1]
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
		r := release{Version: t.Name, Date: t.Date.Format("2006-01-02"), Prerelease: isPrerelease(t.Name), Features: []change{}, Fixes: []change{}}
		if i == 0 {
			r.First = true
			r.Title = "The first release"
		} else {
			ts, err := titles(tags[i-1].Name, t.Name)
			if err != nil {
				return c, err
			}
			r.Features, r.Fixes = sortChanges(ts)
			r.Title = madeTitle(append(append([]change{}, r.Features...), r.Fixes...))
		}
		if subj := strings.TrimSpace(t.Subject); subj != "" && subj != t.Name {
			r.Title, r.Summary = subj, strings.TrimSpace(t.Body)
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
	// a record of each tag, ended by a NUL: name, date, kind, the annotation's subject and its body
	out, err := exec.Command("git", "for-each-ref", "refs/tags",
		"--format=%(refname:short)%09%(creatordate:iso-strict)%09%(objecttype)%09%(contents:subject)%09%(contents:body)%00").Output()
	if err != nil {
		return nil, fmt.Errorf("git for-each-ref: %w", err)
	}
	var tags []tag
	for _, rec := range strings.Split(string(out), "\x00") {
		f := strings.SplitN(strings.TrimLeft(rec, "\n"), "\t", 5)
		if len(f) < 5 {
			continue
		}
		d, err := time.Parse(time.RFC3339, f[1])
		if err != nil {
			return nil, fmt.Errorf("the date of %s: %w", f[0], err)
		}
		t := tag{Name: f[0], Date: d}
		if f[2] == "tag" { // a light tag's "subject" is its commit's title, not a word about the release
			t.Subject, t.Body = f[3], f[4]
		}
		tags = append(tags, t)
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
