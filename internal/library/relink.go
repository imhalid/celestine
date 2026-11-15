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
	if from == "" || !strings.Contains(md, "[[") {
		return md, 0
	}
	n := 0
	out := wikiLink.ReplaceAllStringFunc(md, func(link string) string {
		inside := link[2 : len(link)-2]
		target, alias, aliased := strings.Cut(inside, "|")
		if !strings.EqualFold(strings.TrimSpace(target), from) {
			return link
		}
		n++
		if aliased {
			return "[[" + to + "|" + alias + "]]"
		}
		return "[[" + to + "]]"
	})
	return out, n
}

// LinkableTitle says whether a link can name a page titled title: brackets, a "|" or a line break would end the
// link or be read as its alias.
func LinkableTitle(title string) bool {
	return strings.TrimSpace(title) != "" && !strings.ContainsAny(title, "[]|\n\r")
}
