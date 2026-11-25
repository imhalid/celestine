package main

import (
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui/richtext"
)

// newerBody is a page written by a newer version: an object of a kind this one does not know, with fields it does
// not know either.
const newerBody = `[{"r":[{"t":"before "},{"t":"` + string(richtext.ObjectReplacement) + `","o":{"k":"ai-answer","model":"x-1","answer":"hello","at":[1,2]}},{"t":" after"}]}]`

const newerObj = `{"k":"ai-answer","model":"x-1","answer":"hello","at":[1,2]}`

// An object of a kind this version does not know is read as a placeholder and written back as it came.
func TestBodyKeepsUnknownObjects(t *testing.T) {
	text := decodeBody(newerBody)
	if !strings.Contains(text.String(), "before") || !strings.Contains(text.String(), "after") {
		t.Fatalf("read: %q", text.String())
	}
	out := encodeBody(text)
	if !strings.Contains(out, newerObj) {
		t.Fatalf("written back:\n%s\nwant the object\n%s", out, newerObj)
	}
	if md := bodyMarkdown(text); strings.Contains(md, "ai-answer") || !strings.Contains(md, "before") {
		t.Fatalf("Markdown: %q", md)
	}
}

// A page with a block of a newer version, opened and edited elsewhere and saved, still has the block.
func TestEditingKeepsUnknownObjects(t *testing.T) {
	a, tt := newTestApp(t, "")
	tt.Advance(50 * time.Millisecond)
	p := a.pane().page
	if err := p.Save(newerBody); err != nil {
		t.Fatal(err)
	}
	a.takeOutside(p)
	tt.Frame()
	n := a.pane().doc.Text().Len()
	a.pane().doc.SetSelection(n, n)
	a.pane().doc.InsertText(" more")
	tt.Frame()
	a.save()
	body, err := p.Body()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, newerObj) || !strings.Contains(body, "more") {
		t.Fatalf("saved: %s", body)
	}
}

// A block of a newer version goes through Markdown as a mark no reader shows, and comes back as it was.
func TestUnknownBlockThroughMarkdown(t *testing.T) {
	md := bodyMarkdown(decodeBody(newerBody))
	if !strings.Contains(md, blockMarkStart) || strings.Contains(md, "ai-answer") {
		t.Fatalf("Markdown: %q", md)
	}
	back := encodeBody(importMarkdown(md))
	if !strings.Contains(back, newerObj) || !strings.Contains(back, "before") || !strings.Contains(back, "after") {
		t.Fatalf("read back:\n%s", back)
	}
}

// An agent that reads a page and writes it back whole, changed, keeps a block of a newer version (#65).
func TestAgentRewriteKeepsUnknownBlock(t *testing.T) {
	a, tt := newTestApp(t, "")
	tt.Advance(50 * time.Millisecond)
	if err := a.pane().page.Save(newerBody); err != nil {
		t.Fatal(err)
	}
	a.takeOutside(a.pane().page)
	cs := agent(t, a)
	id := a.idOf(a.pane().page.Dir)
	var read struct{ Markdown string }
	call(t, cs, "read_page", map[string]any{"page": id}, &read)
	call(t, cs, "update_page", map[string]any{"page": id, "markdown": strings.Replace(read.Markdown, "after", "after, changed", 1)}, nil)
	body, _ := a.pane().page.Body()
	if !strings.Contains(body, newerObj) || !strings.Contains(body, "changed") {
		t.Fatalf("rewritten: %s", body)
	}
}

func TestDecodeBodyKeepsUnsafeFileTargetButOpenRefuses(t *testing.T) {
	a, tt := newTestApp(t, "")
	tt.Advance(100 * time.Millisecond)
	var opened []string
	a.shellOpen = func(path string, reveal bool) {
		opened = append(opened, path)
	}
	body := `[{"r":[{"t":"￼","o":{"k":"file","target":"../../Applications/X.app","alias":"Invoice.pdf"}},{"t":"￼","o":{"k":"file","target":"/etc/passwd"}},{"t":"￼","o":{"k":"file","target":"."}},{"t":"￼","o":{"k":"file","target":"con.pdf"}},{"t":"￼","o":{"k":"file","target":"safe.pdf"}}]}]`
	txt := decodeBody(body)
	var files []string
	for _, r := range txt.Runs(0, txt.Len()) {
		if att := r.Attributes.Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok && ch.kind == "file" {
				files = append(files, ch.target)
			}
		}
	}
	want := []string{"../../Applications/X.app", "/etc/passwd", ".", "con.pdf", "safe.pdf"}
	if len(files) != len(want) {
		t.Fatalf("decoded file chips = %v, want %v", files, want)
	}
	for i := range want {
		if files[i] != want[i] {
			t.Fatalf("decoded file chips = %v, want %v", files, want)
		}
	}
	reencoded := encodeBody(txt)
	for _, target := range want {
		if !strings.Contains(reencoded, target) {
			t.Fatalf("reencoded body dropped %q: %s", target, reencoded)
		}
	}
	for _, bad := range []string{"../../Applications/X.app", "/etc/passwd", "."} {
		a.openAttachedFile(a.pane().page, bad, false)
	}
	a.openAttachedFile(a.pane().page, "safe.pdf", false) // the one that may be opened: the hook sees it at once
	if len(opened) != 1 || filepath.Base(opened[0]) != "safe.pdf" {
		t.Fatalf("opened %v, only safe.pdf may be", opened)
	}
}

func TestBlockMarksRejectKnownAndInvalidKinds(t *testing.T) {
	mark := func(raw string) string {
		return blockMarkStart + base64.RawStdEncoding.EncodeToString([]byte(raw)) + "-->"
	}
	md := strings.Join([]string{
		mark(`{"k":"page","target":"Home"}`),
		mark(`{"k":"date","date":"2026-01-01"}`),
		mark(`{"k":"embed","target":"Page"}`),
		mark(`{"k":"file","target":"../../Applications/X.app","alias":"Invoice.pdf"}`),
		mark(`{"k":"footnote","note":"hi"}`),
		mark(`{"k":"table","cols":[]}`),
		mark(`{"k":"query","source":"coll"}`),
		mark(`{"k":"image","target":"pic.png"}`),
		mark(`{"k":"divider"}`),
		mark(`{"k":"code","lang":"go"}`),
		mark(`{"k":"link","url":"https://example.com"}`),
		mark(`{"k":"raw","body":"x"}`),
		mark(`{"k":"fold","open":true}`),
		mark(`{"k":"zoom","scale":2}`),
		mark(`{"k":""}`),
		mark(`not-json`),
		mark(newerObj),
	}, "\n\n")
	out := encodeBody(importMarkdown(md))
	for _, forbidden := range []string{
		`"k":"page"`, `"k":"date"`, `"k":"embed"`, `"k":"file"`,
		`"k":"footnote"`, `"k":"table"`, `"k":"query"`, `"k":"image"`,
		`not-json`,
	} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("blockMarksIn allowed %s in %s", forbidden, out)
		}
	}
	for _, future := range []string{
		`"k":"divider"`, `"k":"code"`, `"k":"link"`,
		`"k":"raw"`, `"k":"fold"`, `"k":"zoom"`,
		newerObj,
	} {
		if !strings.Contains(out, future) {
			t.Fatalf("genuine unknown/future block %s lost: %s", future, out)
		}
	}
}
