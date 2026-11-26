package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// pageCorpus is the Markdown of the app's own tests, every string literal of them with a line or a mark in it, the
// page's own constructs, and random mixes of these: what the library's Markdown is held to the app's on (#595).
func pageCorpus(t *testing.T, n int) []string {
	files, _ := filepath.Glob("*_test.go")
	seen := map[string]bool{}
	var lits []string
	fs := token.NewFileSet()
	for _, f := range files {
		af, err := parser.ParseFile(fs, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(af, func(nd ast.Node) bool {
			if bl, ok := nd.(*ast.BasicLit); ok && bl.Kind == token.STRING {
				if s, err := strconv.Unquote(bl.Value); err == nil && len(s) < 4000 && strings.ContainsAny(s, "\n[#*`>-|$<") && !seen[s] {
					seen[s] = true
					lits = append(lits, s)
				}
			}
			return true
		})
	}
	own := []string{
		"- [*] star\n- [!] imp\n- [?] q\n- [/] prog\n- [w] wait\n- [$] bill\n- [e] err\n- [-] gone\n- [x] done",
		"Buy [ ] milk [x] eggs", "`[ ]` in code [ ] out", "[x]: http://ref\nsee [x]", "1. [ ] one\n2. [x] two",
		"<!--celestine:list mark=%E2%AD%90-->\n- First\n- Second", "<!--celestine:list num=roman-->\n1. a\n2. b",
		"<!--celestine:list task=number-->\n- [ ] a\n- [ ] b", "> <!--celestine:list mark=x-->\n> - q item",
		"```go\ncode\n```", "~~~\ntilde\n~~~", "$$\nx^2\n$$", "$$ inline $$", "| a | b |\n|---|---|\n| 1 | 2 |",
		"> [!note] Callout\n> body", "<div>\nhtml\n</div>", "text <span>html</span> here", "[^1]: one\n[^2]: two",
		"a[^1] b\n\n[^1]: note", "[[Page]] and [[Page|alias]] and [[_private]]", "![[Embed]]", "---", "***",
		"Title\n---", "- [ ]", "- [ ] ", "- a\n-", "\uE002 raw private", "# Heading #tag", "#tag start", "a_b_c",
		"wow!", "\\# not", "**bold** and _it_", "```\n```\n", "- item\n\n  ```js\n  x\n  ```", "    indented",
	}
	corpus := append(append([]string(nil), lits...), own...)
	rng := rand.New(rand.NewSource(1595))
	pool := append(append([]string(nil), own...), lits...)
	for range n {
		k := 1 + rng.Intn(4)
		var parts []string
		for range k {
			parts = append(parts, pool[rng.Intn(len(pool))])
		}
		corpus = append(corpus, strings.Join(parts, []string{"\n", "\n\n"}[rng.Intn(2)]))
	}
	return corpus
}
