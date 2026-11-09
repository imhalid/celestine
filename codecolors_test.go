package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/richtext"
)

// A block of code is drawn in its language's colors: its spans cut where its tokens are, the text the same, a keyword
// and a string in colors of their own; a paragraph of text is not colored.
func TestCodeColors(t *testing.T) {
	a := &app{codeTheme: editor.DefaultTheme(ui.LightTheme())}
	code := richtext.ParagraphStyle{CodeBlock: true, ID: codeIDPrefix + "go"}
	in := []ui.Span{{Text: `func main() { s := "hi" }`}}
	out := a.editorSpans(0, code, append([]ui.Span(nil), in...))
	var text strings.Builder
	colors := map[ui.Color]bool{}
	for _, s := range out {
		text.WriteString(s.Text)
		colors[s.Color] = true
	}
	if text.String() != in[0].Text {
		t.Fatalf("the text changed: %q", text.String())
	}
	if len(out) < 4 || len(colors) < 3 {
		t.Fatalf("not colored by its tokens: %d spans, %d colors", len(out), len(colors))
	}
	plain := a.editorSpans(0, richtext.ParagraphStyle{}, append([]ui.Span(nil), in...))
	if len(plain) != 1 || plain[0].Color != (ui.Color{}) {
		t.Fatalf("a paragraph of text was colored: %+v", plain)
	}
}

// A block's lines after its first, which alone holds the language, are colored in that language too.
func TestCodeColorsAfterTheFirstLine(t *testing.T) {
	a, tt := newTestApp(t, "Text.\n\n```go\nfunc main() {\n\treturn \"x\"\n}\n```\n")
	tt.Frame()
	if a.codeTheme == nil {
		t.Fatal("no colors of code")
	}
	i := -1
	for _, para := range a.doc.Text().Paragraphs() { // by where each starts: i counts them
		i++
		if !para.Style.CodeBlock || strings.HasPrefix(para.Style.ID, codeIDPrefix) {
			continue
		}
		var spans []ui.Span
		line := ""
		for _, r := range para.Runs {
			spans = append(spans, ui.Span{Text: r.Text})
			line += r.Text
		}
		out := a.editorSpans(i, para.Style, spans)
		colors := map[ui.Color]bool{}
		for _, s := range out {
			colors[s.Color] = true
		}
		if strings.Contains(line, "return") && len(colors) < 2 {
			t.Fatalf("line %d %q is not colored: %+v", i, line, out)
		}
	}
}
