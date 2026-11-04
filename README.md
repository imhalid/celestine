# ryudit

A block-based, WYSIWYG Markdown editor on [MyGo](https://github.com/egoist/mygo)'s native UI.

The document is blocks — paragraphs, headings, lists, tasks, quotes, code, dividers — and the Markdown you type turns
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
| ```` ``` ```` | code block |
| `---` | divider |
| `**x**` `_x_` `` `x` `` `~~x~~` | bold, italic, code, strike |

Keys: ⌘B ⌘I ⌘E ⌘⇧X format, ⌘⌥1–3 / ⌘⌥0 headings / paragraph, Tab ⇧Tab indent, ⌘↵ tick a task, ⌘Z undoes a
conversion, ⌘⇧M switches the caret (Slime, Mizan).

## Layout

| Path | What |
| --- | --- |
| `model.go` | Blocks, marks over plain text, selection, undo |
| `rules.go` | Typing rules that turn Markdown into structure; Enter, Backspace, Tab |
| `markdown.go` | Reading and writing Markdown files |
| `view.go`, `input.go` | Layout, drawing, keys, pointer, text input, saving |
| `slime.go`, `mizan.go` | The two carets |
| `docs/` | Research on Thymer and the step-by-step plan (Turkish) |
| `prototypes/slime-md` | The caret prototype |

Decisions: no plugin system and no outside code — only themes (data files) change from outside; MCP will expose the
app's own operations. See `docs/11-yol-haritasi.md` for the plan.

The gate: `gofmt -l .` empty, `go vet ./...`, `go test ./...`, and builds without cgo for darwin, linux and windows.
