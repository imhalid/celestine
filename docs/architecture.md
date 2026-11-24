# How celestine is made

The technical notes that were the README: the page format, the library on disk, the MCP tools and the file-by-file layout. The README is now for people who use the app.

A block-based, WYSIWYG editor with its own page format, on [MyGo](https://github.com/egoist/mygo)'s native UI.

The editor is MyGo's own rich text editor (`ui/richtext`). The document is blocks — paragraphs, headings, lists, tasks, quotes, code — and the Markdown you type turns
into structure as you type it: `# ` makes a heading and the `#` is gone, `**word**` makes the word bold and the stars
are gone. Markdown is never seen in the editor and is not how a page is kept: a page is a list of typed blocks (`body.go`), in a file of its own format. Markdown only comes in (an import, a file dropped, what an agent writes) and goes out (export, what an agent reads).

```bash
go run . [library]      # default: kitaplik.db in the data folder (one already beside the program stays there; never inside an app bundle); a folder given opens a folder library
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
gives the marks back. ⌘⇧M switches the caret (Slime, Mizan). ⌘P opens the palette of commands and every setting, ⌘K
goes to any page or collection by a few letters of it, ⌘F finds and replaces in the page, ⌘[ ⌘] go back and forward. A block - a paragraph and what is under it - has a grip (⋮⋮) on its left: its menu
turns it into another kind, duplicates, moves or deletes it, and it drags; ⌘⇧↑ ⌘⇧↓ move it, `/` at the start of a
line turns it into a kind of block. `@` links a page or a date, or sets one of the page's choices: links are chips
in the text and Obsidian's `[[Page]]`, `[[Page|alias]]`, `[[2026-11-05]]` in the file; a click on one opens its page,
or makes it.

## MCP

Turned on in the settings, the app serves its library to agents over MCP at
`http://127.0.0.1:7457/mcp`, with a token; the settings show the line that adds it to Claude Code:

```bash
claude mcp add --transport http celestine http://127.0.0.1:7457/mcp --header "Authorization: Bearer <token>"
```

That line sends the token itself to whatever holds the port, so a process that took it before the app would collect
it; the stdio bridge (`celestine mcp`) does not: it sends only a proof, once the server has proved it holds the token
too (`mcp_auth.go`).

Its tools: `list_collections`, `list_pages`, `read_page`, `search`, `create_collection`, `create_page`,
`update_page` (title, text replaced or appended, property values), `add_property`, `move_page`, `trash_page`,
`list_tags`, `list_tasks`, `set_task`, `list_versions`, `read_version`, `restore_version`, and `list_commands`/`run_command` (the palette's
commands, those of the page, panes, view and the like; not the ones that change where the library is kept or sent). Pages are named by their
folders (`kitaplar/dune`); text is Markdown. What an agent does shows in the window at once.

## Layout

| Path | What |
| --- | --- |
| `internal/library/`, `internal/library/library.go`, `internal/library/filter.go`, `internal/library/store.go`, `internal/library/store_sql.go`, `internal/library/store_js.go`, `internal/library/backup_sql.go`, `internal/library/history.go`, `internal/library/history_sql.go`, `internal/library/synctrash.go`, `internal/library/safepath.go`, `internal/library/bodyformat.go`, `internal/library/exportmd.go`, `core_alias.go` | The library, a package of its own that knows nothing of the window: collections, their views, pages, sub-pages (`library.go`), filters (`filter.go`), the store (`store.go`, `store_sql.go`, `store_js.go`, `backup_sql.go`), versions and history (`history.go`, `history_sql.go`), the trash, sync (`synctrash.go`), safe paths (`safepath.go`), body format (`bodyformat.go`), and Markdown export (`exportmd.go`). `core_alias.go` gives its names to the app unqualified; its tests run alone in a second or two (`go test ./internal/library/`) |
| `views.go`, `calendarview.go`, `bulk.go`, `sortlist.go` | A collection's page: its views (a list grouped by date, a sortable table, a board of statuses on Ryugo's drag and drop, a calendar), bulk selection, manual sorting, search, a new page |
| `main.go`, `embed.go`, `prewarm.go`, `mobile.go` | The window (its own title bar), `![[Page]]` embed cards (`embed.go`), startup prewarming, mobile layout, the page (title and MyGo's rich text editor), autosave |
| `sidebar.go`, `topbar.go`, `panes.go`, `panebar.go`, `panewin.go`, `sidepanel.go` | The sidebar, which folds on a spring (⌘\), a tree whose branches unfold and whose chevrons turn; the header row, split panes and each pane's bar (its title, menu and close; held, it moves the pane), detached pane windows, and the page-info side panel |
| `windowframe.go`, `windowcontrols.go`, `funbar.go`, `edges.go`, `statusbar.go`, `framemeter.go` | The window's frame: the content a card inside it, with MyGo's material around it (Liquid Glass on macOS 26, Acrylic on Windows 11), a plain color on Linux. Its top edge is the top bar (the sidebar's buttons, the panes' tabs once the window is split, custom `windowcontrols.go`), its bottom edge `statusbar.go`, custom scrollbars (`funbar.go`), sidebar/side-panel width drag strips (`edges.go`), and the debug frame-time meter (`framemeter.go`) |
| `hidetitle.go`, `titlesize.go`, `titlemenu.go`, `pagemenu.go`, `emptystate.go` | Page header controls: hiding/sizing the title, title and page menus, and empty-state placeholders |
| `drag.go` | Dragging pages in the tree, as [Ryugo](https://github.com/imhalid/ryu-go)'s cards move: the row lifts, rows make room, the row it goes into glows, it glides into place |
| `tables.go`, `tablemodel.go` | Tables: one object in the text, always a table (columns with a kind, alignment, width and a foot calculation); in the body a typed object. Markdown tables that come in are read as tables of ours |
| `tablefn.go` | What a table does with its rows: sort by a column, a filter the page keeps, CSV in and out (and tab-separated text pasted from a spreadsheet) |
| `wobble.go` | The jelly: the window itself shakes from side to side and hops when an agent (MCP) adds to a page (what is drawn in it, where the window cannot be moved); the settings can turn it off |
| `editing.go`, `writing.go`, `tab.go`, `focus.go`, `footnote.go` | How the text is set and kept: paragraph spacing, first-line indent, autosave on/off and its delay, ⌘S, ⌘= ⌘− ⌘0, writing stats/goal, Tab/⇧Tab indent key inside the editor (`tab.go`), focus mode, footnotes, and the white space drawn over the text |
| `cmdreg.go`, `appcmds.go`, `keys.go` | The settings and app commands as lists the palette reads: every setting and command (`appcmds.go`), custom shortcut keys (`keys.go`), and the test that holds `Settings` and the commands together |
| `fonts.go`, `appthemes.go`, `themebutton.go`, `codecolors.go`, `radius.go`, `motion.go`, `winzoom.go` | The typefaces (app, editor and code, presets and fallback), the window's zoom (⌘+, ⌘−, ⌘0; `winzoom.go`, MyGo's `SetZoom`), whole-window app themes coloring the page, sidebar, fields, lines, text, and accent (`appthemes.go`), theme switcher button, syntax highlighting colors, corner radii, and reduced-motion helpers |
| `fontsbuiltin.go` | The typefaces the app comes with, embedded and registered at start: Geist (variable, with its italics) and Departure Mono, the defaults when no family is chosen (`assets/fonts`, OFL) |
| `crash.go` | The crash log: a panic in a frame, a goroutine or the program is written to the config folder; the next start tells of a session that ended badly |
| `backup.go`, `backupview.go` | Backups of the library's file (daily, before an import or an update, by hand), pruning, restore at the next start, zip export, and backup preview window |
| `stats.go`, `statsscreen.go` | What is written and used, by the day, kept on this computer; the statistics screen, the library's health, CSV/JSON out |
| `kb.go`, `vim.go`, `help.go` | The keyboard's normal mode (Esc), rows chosen with j k, letters on everything that can be pressed (f), the leader (Space), marks, the command line (:), Vim's motions as a setting and the page of shortcuts |
| `update.go`, `update_other.go`, `update_windows.go`, `updateui.go`, `updatepanel.go`, `updhello.go`, `updtrial.go`, `version.go` | Updates: a signed manifest, a check of sha256 and ed25519, the file put in place, a rollback when the new version does not start, the sidebar update panel, hello/trial dialogs, and version info; `scripts/release` (`changelog.go`), `cloudflare/update`, `.github/workflows/release.yml`; each release also writes `changelog.json` (`scripts/release changelog`), which the site's `/changelog` draws (`src/routes/changelog` of the `celestine-site` repository) |
| `internal/library/sync.go`, `syncapp.go`, `netsync.go`, `account.go`, `profile.go` | Synchronization: three-way by file, conflict copies, tombstones, a folder or a server as the remote, end-to-end encryption, the server's change counter asked while the window is in front, waiting while the computer is offline and kept running out of sight (`netsync.go`, MyGo's `Network` and `Power`); account sign-in/registration and profile screen; `cloudflare/sync` |
| `internal/library/merge.go`, `internal/library/syncbase.go` | A page changed on two devices merged block by block from its base (the page as last synced), kept in the library file's meta or a folder |
| `internal/library/appdir.go` | The name of the app's folders (`AppName`) |
| `internal/library/format.go` | The library's format (`FormatVersion`), in the library file's meta or a folder's `.celestine-format` (not synced): an older one is brought up, a newer one is not opened, and the app says to update (`newerLibrary` in `main.go`) |
| `internal/library/linkmeta.go`, `internal/library/linkguard.go`, `internal/library/linkguard_js.go`, `linkcards.go` | Web links shown (#495): a page's title, Open Graph picture and icon read from its head (`FetchLinkMeta`, `FetchLinkImage`), only from public addresses, checked when each is connected to (`linkguard.go`; `linkguard_js.go` on the web) and after each redirect (#532); the site's icon in the room a link's span leaves before it (`Span.PaddingStart`, at the right of a right-to-left link), a card on hover with Edit and Remove at its foot (#509), a large preview in the bar's link field; off with `noLinkPreviews`, the icons alone with `noLinkIcons` |
| `linkfx.go` | The text a link is being made for, tinted with a shimmer while the bar's field is open, and the flourish over a link just set: a flash, a ring, sparks (#509) |
| `keychain.go`, `keychain_os.go`, `keychain_js.go` | The settings' secrets (the sync key and token, the MCP token) in the system's keychain, the settings file when there is none |
| `i18n.go`, `locales/`, `mygo.json` | The language (docs/24-i18n.md): a catalog per language in `locales/` (`en.json` the source, `tr.json`) through MyGo's i18n package, with the typed functions `mygo generate` writes (`catalog_gen.go`); `msg(id)` says a message a table keeps by its id, `names(id)` gives the words it is found by in every language; English is the default. `TestCatalogs`, `TestNoLiteralWords`, `TestNoIDShown`, `TestPaletteInEnglish` and `TestSettingsInEnglish` check that every language has every message, no word is left untranslatable and none shows in the other language |
| `perf_test.go` | Costs of a big library as a budget test |
| `blocks.go` | The divider and the code block at the third backtick, which richtext does not make |
| `trash.go` | The trash: put back where it was, delete for good; a toast offers back what was just thrown away |
| `caretfx*.go`, `caretextras.go`, `caretplay.go`, `caretstyle.go`, `cursorfx.go`, `cursorfx_rocket.go` | The carets and pointer effects: Slime and Mizan, and the presets (Pacman and the effects of [Cursor-Smith](https://github.com/Sadsnake1/cursor-smith), see `NOTICE.md`), caret playground/styling, and pointer effects |
| `body.go`, `bodyedit.go`, `internal/library/pagefile.go` | The page's own format: blocks and typed objects (the editor's, given to the library as its `BodyFormat`), body editing helpers, and the page file (meta, then body); a legacy `index.md` made one |
| `markdown.go`, `importmd.go`, `dropfiles.go`, `dockdrop.go`, `internal/library/frontmatter.go` | Markdown at the edges only: reading it in, writing it out, importing/dropping files and dock drops, and the plain text view the app searches (tags, tasks, links) |
| `watch.go` | Watching the folder |
| `collsettings.go`, `collicons.go`, `templates.go`, `snippets.go`, `extract.go`, `triage.go`, `dynamic.go` | A collection's settings (General, Properties, Views, Pages, Code; draft, preview, save), its icon and color, page templates and the abbreviations typed into text (`snippets.go`), blocks extracted to a page of their own (`extract.go`), a collection's pages sorted one at a time (`triage.go`), and dynamic collection views |
| `internal/library/properties.go`, `propsui.go`, `propkinds.go`, `tagfield.go` | Properties: the schema, kinds, tag fields, the values, the page's properties section and the dialog adding one |
| `chips.go`, `attachfile.go`, `images.go`, `limits.go`, `cover.go`, `autocover.go`, `autocover_scenes.go`, `banner.go`, `backlinks.go`, `mentions.go`, `links.go`, `headinglinks.go`, `relink.go`, `internal/library/relink.go` | Chips (page links and dates in the text), attached files, images (with the limits on what is read in, `limits.go`; #177), page covers and automatic covers, banners, backlinks and unlinked mentions (`mentions.go`), link resolution, links to a heading (`headinglinks.go`), and the links to a renamed page written to its new title (`relink.go`, the Markdown rewrite in `internal/library/relink.go`; #546) |
| `blockops.go`, `blockui.go`, `blockhints.go`, `blockmark.go`, `blocksel.go`, `alignkeys.go`, `movecmd.go`, `selbar.go`, `slash.go` | Blocks: a paragraph and what is under it; moving (⌘⇧↑↓, `movecmd.go`), alignment keys, block hints/marks/selection, floating selection bar, the grip with its menu and drag, the / menu |
| `palette.go`, `palgroups.go`, `find.go`, `nav.go`, `menu.go`, `ctxmenu.go`, `fieldmenu.go`, `menupanel.go`, `popmenu.go`, `confirm.go`, `toast.go`, `notify.go`, `recent.go`, `quick.go` | The palette (⌘K, `palgroups.go`), finding in the page (⌘F), back and forward, the menu bar, context/popup/panel menus, right-click menu on every text field and the editor (`fieldmenu.go`), confirmation dialogs, toasts, the system's notifications behind other apps, asked for first (`notify.go`, MyGo's `Notification`), the library in the system's recent documents and opened from there (`recent.go`), and quick-capture window |
| `mcp.go`, `mcp_server.go`, `mcp_extra.go`, `mcp_more.go`, `mcp_help.go`, `mcp_js.go`, `mcpsetup.go`, `settings.go`, `settingsx.go` | The MCP server, its tools, setup helpers (`mcpsetup.go`), what agents may do with each collection, and the settings screens (`settings.go`, `settingsx.go`) |
| `mcpbridge.go`, `mcp_auth.go` | `celestine mcp`: the server over stdio for agents that start one, carried to the app's own server (the app started, or its server turned on, when needed); the bridge and the server prove to each other that they hold the token, an HMAC over both sides' nonces, so the token never goes over the port (`mcp_auth.go`) |
| `agentedits.go` | What agents changed: each page kept as it was before their first write, a version before each write, and the status bar's one-click undo |
| `fold.go`, `zoom.go` | Folding a block behind a chip, and zooming into one (what is above and below behind two chips); the file is always written whole, with the blocks' IDs kept |
| `clipmodes.go` | Copy and paste as Markdown, plain text or HTML; ⌘C/⌘V inside the app keeping code languages, task kinds and dividers |
| `search.go`, `searchindex.go`, `searchquery.go`, `queryblock.go`, `libreplace.go`, `tags.go`, `tasks.go` | Search: the index, the query language (OR, NOT, groups, tasks, days, properties), the query block, find and replace across the library (`libreplace.go`), and dedicated tags/tasks views |
| `journal.go`, `dayextras.go`, `naturaldate.go`, `dates.go`, `taskkinds.go`, `subtasks.go` | The journal (a day's page, the day before and after, a day by name, and under a day its tasks and the same day in earlier years, `dayextras.go`), dates said in words and date helpers, the kinds of tasks, and a task's subtasks: the done/total chip under it that folds them and its box filling as they are done (`subtasks.go`) |
| `listmarks.go`, `listmd.go` | Lists of the app's own look (#493): a bullet that is an emoji or an icon, Roman numerals, a bullet or a number before each box of a list of tasks, said by the ID of one item (`celestine:list:`) for the whole list and drawn over richtext's markers; a task in the running text, a chip (`check`). In Markdown (`listmd.go`, #536): a task in the running text is a bare `[ ]`/`[x]` where nothing else reads it, a numbered list of tasks `1. [ ]`, and a list's look a comment `<!--celestine:list …-->` on the line over its first item |
| `lapseui.go`, `internal/library/timelapse.go` | A page's time-lapse: its words in the colors of the days they were written, from its versions |
| `timemachine.go`, `tmside.go`, `tmdiff.go`, `historyui.go`, `tmreveal.go` | A page's history: its copies as a stack in the page's room, the timeline of their days in the sidebar, where the tree fades out, history UI/reveal, and a copy's differences from the page now, side by side |
| `quicksettings.go` | The gear at the top left: a short menu of the settings used most, with the dialog of all of them at its end |
| `welcome.go`, `welcomelooks.go` | First-run welcome and look-selection screen |
| `sound.go` | The sounds: a chime under each notice, the welcome's chord and the window's, made in Go once and played through MyGo's audio, on every system and in the browser |
| `icons.go` | The SVG icons |
| `shown.go`, `caret.go` | Visible-paragraph slice and rune-to-box lookup over `a.boxes` (`shown.go`), and where richtext's caret is (`caret.go`, checked against the editor's own caret in `caret_test.go`) |
| `slime.go`, `mizan.go` | The two carets |
| `web/` | Web / PWA shell assets: manifest, icons, service worker (`sw.js`), and touch helpers (`touch.js`) |
| `android/` | Android Kotlin/Gradle wrapper project (`app/`, `build.gradle.kts`, Gradle scripts) |
| `cloudflare/` | Cloudflare Workers (`cloudflare/sync`: the sync, account and admin API at celestine.halid.dev, which hands every other path to the site Worker of the `celestine-site` repository through a service binding; `cloudflare/update`: the release manifest and downloads) |
| `scripts/` | Build, packaging, release/changelog (`scripts/build.sh`, `scripts/package.sh`, `scripts/dmg.sh` (the macOS DMG), `scripts/appimage.sh` (the Linux AppImages), `scripts/install.sh` (the install script the site serves), `scripts/worker-test.sh` (the sync Worker under `wrangler dev`, and the tests that speak to it; the nightly runs it), `scripts/release/`, `scripts/winicon/`), and platform helper scripts |
| `docs/` | What is still to do, decisions in force and open problems (Turkish); 00-OKU-BENI.md is the index |
| `prototypes/slime-md` | The caret prototype |

Decisions: no plugin system and no outside code — only themes (data files) change from outside; MCP exposes the
app's own operations.

The gate before a release: `gofmt -l .` empty, `go vet ./...`, `go test ./...`, and builds without cgo for darwin,
linux and windows, and for wasm. Before a commit, only the tests of what the change touches.
