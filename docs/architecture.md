# How ryudit is made

The technical notes that were the README: the page format, the library on disk, the MCP tools and the file-by-file layout. The README is now for people who use the app.

A block-based, WYSIWYG editor with its own page format, on [MyGo](https://github.com/egoist/mygo)'s native UI.

The editor is MyGo's own rich text editor (`ui/richtext`). The document is blocks — paragraphs, headings, lists, tasks, quotes, code — and the Markdown you type turns
into structure as you type it: `# ` makes a heading and the `#` is gone, `**word**` makes the word bold and the stars
are gone. Markdown is never seen in the editor and is not how a page is kept: a page is a list of typed blocks (`body.go`), in a file of its own format. Markdown only comes in (an import, a file dropped, what an agent writes) and goes out (export, what an agent reads).

```bash
go run . [library]      # default: ./kitaplik, made and seeded when missing; saved as you type
```

The library is a tree of folders, as [Locus](https://github.com/oneo-me/locus) keeps notes: a collection is a folder with a
`collection.json`, a page a folder with its `page.json` (what is known of the page - title, creation time, place, the
values of its properties - and its body as blocks), a sub-page a page's folder inside another's. A library of the time of
`index.md` files is made `page.json` files once, when it is opened. The sidebar lists collections and their pages as a tree: a
collection's + makes a page in it, a page's + a page under it, ⌃N a page in the open one's collection.

Blocks richtext has no kind for - a link to a page, a day, a page's card, a table, a picture - are typed objects in the
body. Markdown that comes in with what richtext does not know (HTML, math, a footnote, an Obsidian callout) shows as a
raw block. The library folder is watched: a page file changed outside shows at once, and if the open page has unsaved
edits both versions are kept.

A collection has no properties until one is added, as in Thymer: **+ Özellik ekle** under a page's title adds a choice,
a text, a number, a date or a link to every page of its collection. Values are in the pages' meta; the table shows each property as a column, and the first choice gives the
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
line turns it into a kind of block. `@` links a page or a date, or sets one of the page's choices: links are chips
in the text and Obsidian's `[[Page]]`, `[[Page|alias]]`, `[[2026-11-05]]` in the file; a click on one opens its page,
or makes it.

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
| `internal/library/` | The library, a package of its own that knows nothing of the window: collections, their views, pages, sub-pages, the page file, front matter, properties, the store in one file (`store_sql.go`, `store_js.go`), versions, the trash, sync, the Markdown export. `core_alias.go` gives its names to the app unqualified; its tests run alone in a second or two (`go test ./internal/library/`) |
| `views.go` | A collection's page: its views (a list grouped by date, a sortable table, a board of statuses on Ryugo's drag and drop), search, a new page |
| `main.go` | The window (its own title bar), the page (title and MyGo's rich text editor), autosave |
| `sidebar.go` | The title bar and the sidebar: folds on a spring (⌘\), a tree whose branches unfold and whose chevrons turn |
| `drag.go` | Dragging pages in the tree, as [Ryugo](https://github.com/imhalid/ryu-go)'s cards move: the row lifts, rows make room, the row it goes into glows, it glides into place |
| `tables.go`, `tablemodel.go` | Tables: one object in the text, always a table (columns with a kind, alignment, width and a foot calculation); in the body a typed object. Markdown tables that come in are read as tables of ours |
| `tablefn.go` | What a table does with its rows: sort by a column, a filter the page keeps, CSV in and out (and tab-separated text pasted from a spreadsheet) |
| `wobble.go` | The jelly: the window itself shakes from side to side and hops when an agent (MCP) adds to a page (what is drawn in it, where the window cannot be moved); the settings can turn it off |
| `editing.go` | How the text is set and kept: paragraph spacing, first-line indent, autosave on/off and its delay, ⌘S, ⌘= ⌘− ⌘0, and the white space drawn over the text |
| `cmdreg.go` | The settings as one list the palette reads: every setting is a command (toggle, number, choice), commands with a value ("yazı boyutu 18"), and the test that holds `Settings` and the commands together |
| `fonts.go` | The typefaces: app, editor and code, from the families installed, each in its own letters, shown on the page as they are passed over, with presets, stars and a fallback |
| `crash.go` | The crash log: a panic in a frame, a goroutine or the program is written to the config folder; the next start tells of a session that ended badly |
| `backup.go` | Backups of the library's file (daily, before an import or an update, by hand), pruning, restore at the next start, zip export |
| `stats.go`, `statsscreen.go` | What is written and used, by the day, kept on this computer; the statistics screen, the library's health, CSV/JSON out |
| `kb.go`, `vim.go`, `help.go` | The keyboard's normal mode (Esc), rows chosen with j k, letters on everything that can be pressed (f), the leader (Space), marks, the command line (:), Vim's motions as a setting, the page of shortcuts and the first-run tour |
| `update.go`, `updateui.go` | Updates: a signed manifest, a check of sha256 and ed25519, the file put in place, a rollback when the new version does not start; `scripts/release`, `cloudflare/update`, `.github/workflows/release.yml` |
| `internal/library/sync.go`, `syncapp.go` | Synchronization: three-way by file, conflict copies, tombstones, a folder or a server as the remote, end-to-end encryption; `cloudflare/sync` |
| `i18n.go` | The language: Turkish as written, English from a dictionary for the main surfaces |
| `perf_test.go`, `scripts/build.sh` | Costs of a big library as a budget test; the build of the five targets with sizes and a budget |
| `blocks.go` | The divider and the code block at the third backtick, which richtext does not make |
| `trash.go` | The trash: put back where it was, delete for good; a toast offers back what was just thrown away |
| `caretfx*.go` | The carets: Slime and Mizan, and the presets (Pacman and the effects of [Cursor-Smith](https://github.com/Sadsnake1/cursor-smith), see `NOTICE.md`): each a caret that is always as it is, with nothing to set |
| `body.go`, `internal/library/pagefile.go` | The page's own format: blocks and typed objects (the editor's, given to the library as its `BodyFormat`), and the page file (meta, then body); a legacy `index.md` made one |
| `markdown.go`, `internal/library/frontmatter.go` | Markdown at the edges only: reading it in, writing it out, and the plain text view the app searches (tags, tasks, links) |
| `watch.go` | Watching the folder |
| `collsettings.go`, `collicons.go` | A collection's settings (General, Properties, Views, Pages, Code; draft, preview, save), its icon and color |
| `internal/library/properties.go`, `propsui.go` | Properties: the schema, the values, the page's properties section and the dialog adding one |
| `chips.go` | Chips: page links and dates in the text, as richtext attachments; `[[…]]` in the file |
| `blockops.go`, `blockui.go`, `slash.go` | Blocks: a paragraph and what is under it; moving (⌘⇧↑↓), the grip with its menu and drag, the / menu |
| `palette.go`, `find.go`, `nav.go`, `menu.go` | The palette (⌘K), finding in the page (⌘F), back and forward, the menu bar |
| `mcp.go`, `settings.go` | The MCP server and the settings that turn it on |
| `quicksettings.go` | The gear at the top left: a short menu of the settings used most, with the dialog of all of them at its end |
| `icons.go` | The SVG icons |
| `caret.go` | Where richtext's caret is (it does not say): its paragraph laid out again as the editor lays it out; `caret_test.go` checks it against the editor's own caret on every rune |
| `slime.go`, `mizan.go` | The two carets |
| `docs/` | Research on Thymer and the step-by-step plan (Turkish) |
| `prototypes/slime-md` | The caret prototype |

Decisions: no plugin system and no outside code — only themes (data files) change from outside; MCP exposes the
app's own operations. See `docs/11-yol-haritasi.md` for the plan.

The gate: `gofmt -l .` empty, `go vet ./...`, `go test ./...`, and builds without cgo for darwin, linux and windows.
