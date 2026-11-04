# ryudit

A block-based, WYSIWYG Markdown editor on [MyGo](https://github.com/egoist/mygo)'s native UI.

The editor is MyGo's own rich text editor (`ui/richtext`). The document is blocks — paragraphs, headings, lists, tasks, quotes, code — and the Markdown you type turns
into structure as you type it: `# ` makes a heading and the `#` is gone, `**word**` makes the word bold and the stars
are gone. Markdown is never seen in the editor; it is only the file format, so any other tool can open what you write.

```bash
go run . [file.md]      # default: belge.md here, saved as you type
```

| Type | Becomes |
| --- | --- |
| `# ` `## ` `### ` | heading 1–3 |
| `- ` `* ` `1. ` | bullet, numbered list |
| `[] ` `[x] ` | task |
| `> ` | quote |
| `**x**` `_x_` `` `x` `` `~~x~~` | bold, italic, code, strike |

Keys are the editor's, a word processor's: ⌘B ⌘I ⌘U, ⌘K a link, Tab ⇧Tab indent; ⌘Z undoes a conversion and
gives the marks back. ⌘⇧M switches the caret (Slime, Mizan). Not yet: typing ``` for a code block and `---` for a
divider (richtext has code blocks, but no shortcut for them).

## Layout

| Path | What |
| --- | --- |
| `main.go` | The window: MyGo's rich text editor (`ui/richtext`) on a page, Markdown file in and out, autosave |
| `caret.go` | Where richtext's caret is (it does not say): its paragraph laid out again as the editor lays it out; `caret_test.go` checks it against the editor's own caret on every rune |
| `slime.go`, `mizan.go` | The two carets |
| `docs/` | Research on Thymer and the step-by-step plan (Turkish) |
| `prototypes/slime-md` | The caret prototype |

Decisions: no plugin system and no outside code — only themes (data files) change from outside; MCP will expose the
app's own operations. See `docs/11-yol-haritasi.md` for the plan.

The gate: `gofmt -l .` empty, `go vet ./...`, `go test ./...`, and builds without cgo for darwin, linux and windows.
