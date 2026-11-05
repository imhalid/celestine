# ryudit

A block-based, WYSIWYG Markdown editor on [MyGo](https://github.com/egoist/mygo)'s native UI.

The editor is MyGo's own rich text editor (`ui/richtext`). The document is blocks — paragraphs, headings, lists, tasks, quotes, code — and the Markdown you type turns
into structure as you type it: `# ` makes a heading and the `#` is gone, `**word**` makes the word bold and the stars
are gone. Markdown is never seen in the editor; it is only the file format, so any other tool can open what you write.

```bash
go run . [library]      # default: ./kitaplik, made and seeded when missing; saved as you type
```

The library is plain files, as [Locus](https://github.com/oneo-me/locus) keeps notes: a collection is a folder with a
`collection.json`, a page a folder with its `index.md` (title and creation time in the front matter, the body in
Markdown), a sub-page a page's folder inside another's. The sidebar lists collections and their pages as a tree: a
collection's + makes a page in it, a page's + a page under it, ⌃N a page in the open one's collection.

Files written elsewhere are safe here: what richtext doesn't know (a table, HTML, math, a footnote, an Obsidian
callout) shows as a raw block and is written back as it was, code blocks keep their language, the front matter is
YAML changed only where ryudit's own keys are (lists, comments and unknown keys stay), and the library folder is
watched: a change made outside shows at once, and if the open page has unsaved edits both versions are kept.

A collection has no properties until one is added, as in Thymer: **+ Özellik ekle** under a page's title adds a choice,
a text, a number, a date or a link to every page of its collection. Values are in the pages' front matter (front
matter ryudit doesn't know is kept); the table shows each property as a column, and the first choice gives the
collection a board of it.

| Type | Becomes |
| --- | --- |
| `# ` `## ` `### ` | heading 1–3 |
| `- ` `* ` `1. ` | bullet, numbered list |
| `[] ` `[x] ` | task |
| `> ` | quote |
| `**x**` `_x_` `` `x` `` `~~x~~` | bold, italic, code, strike |
| ```` ``` ```` | block of code |
| `---` | divider |

Keys are the editor's, a word processor's: ⌘B ⌘I ⌘U, ⌘K a link, Tab ⇧Tab indent; ⌘Z undoes a conversion and
gives the marks back. ⌘⇧M switches the caret (Slime, Mizan). ⌘K opens the palette (go to any page or collection by a
few letters of it, run any command), ⌘F finds and replaces in the page, ⌘[ ⌘] go back and forward. A block - a paragraph and what is under it - has a grip (⋮⋮) on its left: its menu
turns it into another kind, duplicates, moves or deletes it, and it drags; ⌘⇧↑ ⌘⇧↓ move it, `/` at the start of a
line turns it into a kind of block.

## MCP

Turned on in the settings (the sliders at the foot of the sidebar), the app serves its library to agents over MCP at
`http://127.0.0.1:7457/mcp`, with a token; the settings show the line that adds it to Claude Code:

```bash
claude mcp add --transport http ryudit http://127.0.0.1:7457/mcp --header "Authorization: Bearer <token>"
```

Its tools: `list_collections`, `list_pages`, `read_page`, `search`, `create_collection`, `create_page`,
`update_page` (title, text replaced or appended, property values), `add_property`, `move_page`, `trash_page`. Pages are named by their
folders (`kitaplar/dune`); text is Markdown. What an agent does shows in the window at once.

## Layout

| Path | What |
| --- | --- |
| `library.go` | The library on disk: collections, their views, pages, sub-pages, front matter; no UI |
| `views.go` | A collection's page: its views (a list grouped by date, a sortable table, a board of statuses on Ryugo's drag and drop), search, a new page |
| `main.go` | The window (its own title bar), the page (title and MyGo's rich text editor), autosave |
| `sidebar.go` | The title bar and the sidebar: folds on a spring (⌘\), a tree whose branches unfold and whose chevrons turn |
| `drag.go` | Dragging pages in the tree, as [Ryugo](https://github.com/imhalid/ryu-go)'s cards move: the row lifts, rows make room, the row it goes into glows, it glides into place |
| `blocks.go` | The divider and the code block at the third backtick, which richtext does not make |
| `trash.go` | The trash: put back where it was, delete for good; a toast offers back what was just thrown away |
| `markdown.go`, `frontmatter.go`, `watch.go` | Reading and writing Markdown and front matter without losing what other tools wrote; watching the folder |
| `collsettings.go`, `collicons.go` | A collection's settings (General, Properties, Views, Pages, Code; draft, preview, save), its icon and color |
| `properties.go`, `propsui.go` | Properties: the schema, the values, the page's properties section and the dialog adding one |
| `blockops.go`, `blockui.go`, `slash.go` | Blocks: a paragraph and what is under it; moving (⌘⇧↑↓), the grip with its menu and drag, the / menu |
| `palette.go`, `find.go`, `nav.go`, `menu.go` | The palette (⌘K), finding in the page (⌘F), back and forward, the menu bar |
| `mcp.go`, `settings.go` | The MCP server and the settings that turn it on |
| `icons.go` | The SVG icons |
| `caret.go` | Where richtext's caret is (it does not say): its paragraph laid out again as the editor lays it out; `caret_test.go` checks it against the editor's own caret on every rune |
| `slime.go`, `mizan.go` | The two carets |
| `docs/` | Research on Thymer and the step-by-step plan (Turkish) |
| `prototypes/slime-md` | The caret prototype |

Decisions: no plugin system and no outside code — only themes (data files) change from outside; MCP exposes the
app's own operations. See `docs/11-yol-haritasi.md` for the plan.

The gate: `gofmt -l .` empty, `go vet ./...`, `go test ./...`, and builds without cgo for darwin, linux and windows.
