package library

import (
	"regexp"
	"strings"
)

// wikiLink is a link to a page as Markdown writes it: [[Title]], [[Title|what it says]], and the embed ![[Title]]
// (whose "!" stays outside the brackets).
var wikiLink = regexp.MustCompile(`\[\[([^\[\]\n]+?)\]\]`)

// RenameLinks is md with its links to the page titled from written to the page titled to, and how many it changed.
// A link's title is matched as the app resolves it, whatever its case and the spaces around it; what a link says
// instead of the title (its alias, after "|") is kept as it was.
func RenameLinks(md, from, to string) (string, int) {
	from = strings.TrimSpace(from)
	if from == "" {
		return md, 0
	}
	return RenameLinksTo(md, map[string]string{strings.ToLower(from): to})
}

// RenameLinksTo is RenameLinks for several renames at once: to has each old title, trimmed and in lower case, and the
// title it is now. Each link is written once, by its title as md has it, so a rename of A to B and of B to C sends a
// link to A to B, not on to C.
func RenameLinksTo(md string, to map[string]string) (string, int) {
	if len(to) == 0 || !strings.Contains(md, "[[") {
		return md, 0
	}
	n := 0
	out := wikiLink.ReplaceAllStringFunc(md, func(link string) string {
		inside := link[2 : len(link)-2]
		target, alias, aliased := strings.Cut(inside, "|")
		now, ok := to[strings.ToLower(strings.TrimSpace(target))]
		if !ok {
			return link
		}
		n++
		if aliased {
			return "[[" + now + "|" + alias + "]]"
		}
		return "[[" + now + "]]"
	})
	return out, n
}

// LinkableTitle says whether a link can name a page titled title: brackets, a "|" or a line break would end the
// link or be read as its alias.
func LinkableTitle(title string) bool {
	return strings.TrimSpace(title) != "" && !strings.ContainsAny(title, "[]|\n\r")
}
