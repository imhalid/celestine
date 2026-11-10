# 04 Performance

Per-frame cost, idle cost, memory, start-up and storage. Open issues #14, #15, #16 and #70 are answered here as far
as the code allows.

Back to the [index](README.md). **Nothing in this report is a profile.** The app was not run. Each finding is a
reading of what the code does on every frame, with a count where one could be derived. The few measured numbers say
where they come from. Before acting on the order below, measure: the plan at the end says how.

## How a frame works

- `app.view` wraps `viewFrame` (`main.go:846`), which takes `a.mu` for the whole frame and builds the entire window:
  sidebar, header, panes, status bar, overlays, palette, dialogs, toasts. A pointer move, a hover, a caret blink or
  an animation re-runs all of it.
- A torn-off pane window has its own view (`panewin.go:52`) under the same lock.
- `a.do(fn)` locks, runs, unlocks, and asks for a frame. A goroutine does its slow part outside and commits through
  it. The lock is not reentrant.
- A paint can ask for another with `p.AnimationFrame()`: a full-window repaint at the display's rate
  (`mygo/ui/paint.go:136`). `c.Invalidate()` asks for the view function itself to run again.
- Caches in use: the palette's place index, the title index (keyed on `PagesGen`), query results (3 s), backlink
  lines, the per-page link index (keyed on `Modified` and `Writes`), the search index (pair bitmaps, incremental).
  These are the model for what is missing below.

## Findings

### PERF-01 The palette rebuilds everything on every frame (issue #16)
- **High.** Read by two reviewers independently; re-checked.
- Where: `palette.go:655`: `items := a.paletteItems(a.pal.query)` in `paletteView`, each frame, nothing kept.
- In command mode (⌘P) that is `a.commands()` (`palette.go:82-248`), which each frame:
  - builds every command with its closures, and runs `cmdCat` (a scan of about 80 prefixes) for each;
  - calls `backups(a.backupDirOf())` at `palette.go:202`, which is `os.ReadDir`, a stat and a regexp for every
    backup file (`backup.go:51-72`): **disk I/O on the window's thread per frame**;
  - builds all setting commands; for a choice setting `d.choices(a)` is called once per choice
    (`cmdreg.go:354-356`), and for the three font settings each call scans every installed family
    (`fonts.go:73-87`). With 12 remembered families and 800 installed that is about 350 000 string compares a frame;
  - with text typed, folds the query and both strings again for every command, four to six folds each
    (`palette.go:462-468`, `:589-595`).
- In page mode (⌘K) with nothing typed: `a.recent(10)` calls `a.places()` (`palette.go:289`), which builds an item
  with two closures for every page of the library, and all tags, each frame. The place index is bypassed.
- In page mode with two letters or more: `searchPages` each frame (PERF-02).
- Every item becomes a row (`palette.go:711-724`) inside a 300 px scroll: hundreds of rows laid out to show ten.
- A pointer move to another row asks for a frame (`palette.go:775-780`), so moving the mouse over the list pays all
  of it again per move. With PERF-04 the frames never stop while the pointer rests there.
- `docs/17` measures one palette query at 22 ms on 10 000 pages. That is per frame.
- Fix:
  1. Keep the items on `a.pal`, recomputed only when the query, the category, `PagesGen` or a settings generation
     changes.
  2. Build `commands()` once when the palette opens and after a command runs. Read `backups()` once at open, never
     in a frame.
  3. Fold each title and alias once when the list is built; use `matchFolded` as the place index does.
  4. `recent()` looks its ten places up in the place index.
  5. Hoist `d.choices(a)` out of the loop. Keep a set of folded family names built when the font scan finishes.
  6. Window the rows (`ui.List`, as the task view does) or cap them at about 40.
- The second half of #16, "scroll still triggers at the end", was **not found**. The only programmatic scroll is
  `ScrollIntoView` when the selection moves (`palette.go:783-786`). Whether MyGo passes an over-scroll at the end of
  the list on to the page under the dialog needs a look at its wheel routing with the palette open over a scrolled
  page.

### PERF-02 Search results are recomputed every frame, and a snippet folds the whole page
- **High.** Read.
- Where: `search.go:200` (the search screen, 80 hits), `views.go:214` (a dynamic collection, **limit 0, no cap**),
  `queryblock.go:174` by way of `slash.go:720` (the `/query` menu, only to show a count).
- `snippet` (`search.go:51-57`) folds the page's whole plain text and converts both the text and the folded text to
  `[]rune`, for every hit, though the folded text already exists in the index and dynamic collections and counts use
  no snippet at all. Every query also starts with a walk of every page (`searchindex.go:225`).
- A dynamic collection whose query matches 2 000 pages folds and rune-converts 2 000 pages per frame. The search
  field's caret blink alone keeps the frames coming.
- Fix: cache `(query, limit) → hits`, invalidated by `links.built` and `PagesGen`; split `searchPages` into "find"
  and "snippets for these n"; in `snippet` take the folded text as an argument and work on a window of bytes around
  the first match.

### PERF-03 The default caret keeps the window repainting forever
- **Medium, and the app's idle cost.** Read; re-checked. Power not measured.
- Where: `slime.go:127` is `defer p.AnimationFrame()`, unconditional; the comment above it says "it never rests".
  Mizan does the same after 600 ms idle (`mizan.go:163-167`, `:183-187`). Five presets end their draw with an
  unconditional request (Tamagotchi, Kawaii, Neon, the mood face, the plant).
- The sleeper lowers its own request to 30 fps and lies still after 90 s (`caretextras.go:66-89`), but the caret's
  own request wins.
- Each painted frame also: re-shapes the caret's paragraph (`caret.go:73-84`; memoised only from 2 000 runes),
  scans the paragraph up to the caret for its style, allocates two closures, and runs every `DrawOver` pass again.
  Measured in a test binary: `locate` on one 1 900-rune paragraph is 0.70 ms a call, which is 4 to 8 % of a core at
  60 to 120 Hz for the caret's position alone.
- Scenario: click in the text and take your hands off. The app repaints at 60 to 120 fps until the editor loses the
  focus.
- Fix: let the slime settle; ask for a frame only while its spring is above an epsilon or within some milliseconds
  of a keystroke; idle breathing at 30 fps for a bounded time, then a timer. One shared `idle(p, busy)` helper for
  Slime, Mizan and the presets. Drop the 2 000-rune floor of the layout memo and key it on document version,
  paragraph start, width and font.

### PERF-04 The scrollbar asks for a frame forever while the pointer is over a scroll area
- **High.** Read; re-checked.
- Where: `funbar.go:291`. With the pointer over the area and not on the strip, `alpha` rests at 0.55
  (`funbar.go:226`), which is always inside the "still animating" test `alpha > 0.01 && alpha < 0.99`. So
  `AnimationFrame` is requested on every paint without end.
- `funbar.go:169`: while the pointer is on the 14 px strip, `c.Invalidate()` rebuilds the **view** every frame.
- Scenario: the pointer resting over the page, the palette list, the settings or the timeline is 60 to 120 full
  repaints a second for as long as it stays. This multiplies PERF-01 and is the likeliest reason menus and lists
  "feel slow under the mouse".
- Fix: snap when close (`if |want-alpha| < 0.01 { alpha = want }`) and ask for a frame only while a value is still
  moving; on hover ask for a repaint during the width transition only, not a view rebuild.

### PERF-05 Two scrollbars in split panes (issue #14)
- **High.** Read in the code; not reproduced on screen.
- The page is meant to scroll as a whole: the editor is given a height computed from the paragraph boxes laid out so
  far (`main.go:1087`, `:1258-1289`), so it never scrolls inside itself. Three things break that with more than one
  pane:
  1. The boxes are refreshed for the pane in use only (`main.go:1002-1003`, outside `withPane`). A split halves an
     inactive pane's width, its text wraps to more lines, the stale height is too small, and MyGo's editor scrolls
     inside its box and paints its own bar beside the app's. That is the issue as reported.
  2. `VisibleParagraphs` belongs to the document, not to an editor (`richtext/document.go:40-43`). A split shows the
     same page in both panes by default, so the pane in use may get the boxes of the other pane's editor, laid out
     at another width.
  3. A pane that was never in use has no boxes and takes the estimate branch every frame (`main.go:1272-1281`): it
     materialises the whole text to count newlines, and the result is a guess.
- Also `linksH`, the backlinks' height, is a field of `app`, not of the pane (`main.go:251`).
- Fix: keep the boxes per pane and per editor, read inside the pane's own build or paint; ask MyGo for the boxes of
  that editor element (a small general patch upstream); move `linksH` into `paneState`. Until the patch: turn the
  editor's own scrollbar off, and do not show one document in two editors.
- Related: `paneWindowView` (`panewin.go:52-66`) has no `recover`, unlike `view`, so a panic in a torn-off window's
  frame is not written to the crash log and takes the app down; and it never sets `a.frame`, so a frame asked for
  while that window is built goes to the main window.

### PERF-06 The gear menu at the top left (issue #15): no cost of its own found
- **Medium.** Suspected. Two reviewers looked and neither pinned it down.
- The menu is seven rows drawn by `drawMenu` (`menupanel.go:75-172`). Nothing in it is expensive. What does cost:
  1. It is part of `viewFrame`: each hover that changes the lit row invalidates (`menupanel.go:128-135`), and that
     frame is the whole window. The menu can never be faster than the full frame.
  2. PERF-04 keeps the frames coming while the pointer is over anything that scrolls.
  3. Every choice in it changes the look: the theme changes `c.Key("look:" + look)` (`main.go:874-879`), which
     discards and rebuilds the entire element tree, and each choice writes the settings synchronously
     (`quicksettings.go:32`, `:55`, `:63`). One very long frame per click.
  4. `contextView` calls `Focus()` on the panel every frame (`ctxmenu.go:104`).
- To settle it: log `viewFrame`'s duration with the menu open and closed on the same page. If they match, the menu
  is innocent and the fix is PERF-01, PERF-03, PERF-04 and PERF-09. If a click is the slow part, apply a theme
  change without re-keying the tree and save through the debounce.

### PERF-07 Settings and pages are written to disk synchronously from frames
- **Medium to high.** Read.
- With two panes or more, `keepPanes` (`panes.go:634-639`) marshals the Dock layout to JSON every frame to compare
  it. The layout includes split sizes and each pane's title, so it changes on every frame of a splitter drag and on
  every keystroke in a title. It then calls `saveSettingsSoon`, but the debounce in `keepSettings`
  (`settings.go:61-71`) applies only while the settings screen is open: anywhere else "soon" is the next frame, a
  full marshal of all settings and a store write per frame.
- `sidebar.go:886`, `:92`, `sidepanel.go:25` and every line of the gear menu call `saveSettings()` directly.
- Every keystroke in a page's title calls `SetTitle`, which reads the page's file and writes the whole page back
  (`main.go:1213-1220`, `library.go:491-499`), and bumps `PagesGen` so the title index of every page is rebuilt.
  A collection's name does the same per keystroke (`views.go:64-71`).
- Fix: make the debounce unconditional, and write pending settings in `OnBeforeQuit`, which today flushes pages and
  usage but not settings. Compare panes by a counter bumped by the Dock's change callback. Set the title in memory
  at once and write it with the page's own autosave.

### PERF-08 The folder watcher scans the whole library on disk while holding the app's lock
- **High for folder libraries; not the default store.** Read; re-checked.
- Where: `watch.go:32-34` runs `a.do(func() { a.checkDisk() })` every 1.5 s, and `checkDisk` starts with
  `scanLibrary` (`watch.go:79`): a `ReadDir` of every folder and a `Stat` of every `page.json`. No frame can start
  while it runs. On a cloud-synced folder, the very case the watcher exists for, that is the slow case.
- It breaks the project's own rule that a goroutine does its slow part outside the lock.
- Fix: scan outside (it needs only `FS` and `Root`), then `a.do` to compare and apply; skip a tick while the
  previous scan is still running.

### PERF-09 A collection's page filters, sorts and builds every row on every frame
- **Medium.** Read.
- `views.go:151-155`: `filtered`, a delete pass and `sortPages` each frame. The comparator lower-cases both titles
  on every comparison (`views.go:247`, `:252`): about 250 000 allocations a frame at 5 000 pages for a title sort.
- The list, its groups, the gallery and the board build every page as a row or card; only the table (50 rows), the
  task list and the manual list are windowed. The journal is the collection that grows without bound.
- The board calls `coll.Column` per option (`views.go:1013`), each a full pass and a stable sort: options × pages
  per frame. The calendar rebuckets all pages per frame to show one month (`calendarview.go:67-76`).
- Fix: keep the filtered and sorted pages per view, keyed by `PagesGen`, the view's sort and filters, and the search
  text; sort on pre-folded keys; `ui.List` for the list and gallery; cap or window board columns; bucket the
  calendar by month once.

### PERF-10 Quadratic and per-frame work in tags and tasks
- **Medium.** Read.
- A `#tag` term is scored with `slices.Contains` over the pages that carry the tag, for every candidate
  (`searchquery.go:469-470`), and a tag term does not narrow the candidates. A query that is only `#proje`, the
  example in the docs, is pages × tagged pages. By PERF-02 it runs per frame.
- The task view does two passes over all tasks and allocates a slice of every shown task each frame
  (`tasks.go:182-273`); with a query typed it folds every task's text and title per frame.
- The tag view builds a pill for every tag and a row for every page of the tag (`tags.go:193-218`).
- After every save the whole link, tag and task index is rebuilt from the per-page entries, on the frame
  (`backlinks.go:45-75`).
- The backlink-line cache is thrown away whole past 800 entries (`backlinks.go:168`): a page with more than 800
  backlinks empties and refills it every frame, a file read and a regexp compile per backlink.
- `renameTag` (`tags.go:394-422`) rewrites every page with the tag synchronously in the frame that clicked Save; an
  error half-way leaves the rename half done.
- Fix: score a tag from the page's own entry; cache filtered task rows; patch the index for the one page that
  changed; make the cache an LRU; run `renameTag` in a goroutine in batches.

### PERF-11 Every page's text is kept twice in memory, and some of it is never released
- **Medium.** Read; sizes are estimates.
- `plainIdx` holds the plain and the folded text of every page (`backlinks.go:245-267`). The plain copy is used only
  to cut snippets for at most 80 hits.
- `links.plain` is never pruned: trashed pages keep both strings, and after a reload of a folder library
  (`watch.go:186` replaces every page object) the old pointers stay as keys, so each reload leaks the library's text
  twice and, through the keys, the old page graph. Several other pointer-keyed maps are likewise never cleared.
- The search index is `[65536][]uint64` rows extended to the highest page id that has the pair: a worst case of
  82 MB at 10 000 pages. Real text touches a fraction, but ids only grow.
- The prewarm holds the raw bytes of every page file at once (`prewarm.go:49-56`) on top of the parsed texts.
- Fix: drop the plain copy and cut snippets from the page read again for the few hits shown; rebuild `links.plain`
  with the kept set and clear the pointer-keyed maps on reload; stream the prewarm; sparse rows for rare pairs.

### PERF-12 Start-up loads every blob of the library into memory
- **Medium.** Reproduced: one 8 MB attachment, 8 388 608 bytes held.
- Where: `store_sql.go:387` selects `data` for every row into the snapshot used around `OpenLibraryIn`
  (`main.go:450`). Pictures and attachments are rows like any other, though only the JSON files are read. Together
  with PERF-11's prewarm this is the likely source of the 690 MB once seen at 7 700 pages.
- Fix: select the data only for `%.json` rows and fall through to the table for the rest.

### PERF-13 Quadratic and per-keystroke costs on big pages
- **Medium.** Reproduced and measured in a test binary; read the numbers as ratios.
- `starts()` prepends to a slice in a loop (`markdown.go:350-356`): 2.3 ms at 1 000 paragraphs, 46 ms at 10 000,
  141 ms at 20 000. It runs for every `Page.Text()`: the search index, tasks, tags, backlinks, embeds, MCP reads.
- On the window's thread per autosave of a 500 KB page: `encodeBody` 17 ms, `save()` 33 ms.
- Find re-folds the whole document on every change while the bar is open, an allocation per rune
  (`find.go:47-57`): 54 ms on that page.
- Whitespace marks (`editing.go:136-146`) call an O(n) conversion twice per rune and a `SelectionRects` per rune:
  quadratic per paragraph per frame, capped only at 4 000 runes. PERF-03 makes every frame happen.
- Vim mode converts the whole document to `[]rune` on every key (`vim.go:91`).
- Fix: append then reverse in `starts`; cache the Markdown view per page by `Modified` and `Writes`; fold once per
  document version for find; one prefix table per paragraph for the marks; work on the caret's paragraph in Vim.

### PERF-14 Page history and the time-lapse read on the window's thread
- **Medium.** Read. These files hold uncommitted work for issue #71.
- Each copy is read from the store up to three times in the frame (`timemachine.go:83`, `:246`, `:331`). Opening the
  history does about 13 version reads and three document parses in its first frame; holding ↓ parses a document per
  step. The maps of visited copies only grow while the history is open.
- `startLapse` (`lapseui.go:143-161`) reads every kept version under the lock, at every page open (the time-lapse
  is on by default) and again after each pause in typing. Only the word comparison is in the goroutine.
- `DiffLines` allocates 1 501 × 1 501 ints, about 18 MB, per comparison (`history.go:95-105`).
- Fix: load a copy once, in a goroutine, into one struct; keep an LRU of a few documents; move the time-lapse's
  reads into its goroutine.

### PERF-15 Store operations that cost more than they should
- **Low to medium.** Reproduced or read.
- Reordering writes every sibling separately with a version each (DATA-16).
- `below()` cannot use the primary key (`store_sql.go:191`): every trash, restore, move and delete reads all rows,
  and for a collection runs two statements per descendant. Use a range on the key and one statement per table.
- A page save reads the old blob three times (`store_sql.go:135`, `history_sql.go:34`).
- `versions` has no index on `taken`, so each prune is a full scan.

### PERF-16 Slow or unbounded work under the app's lock
- **Medium.** Read.
- A cover or banner is read and fully decoded inside `a.do` (`cover.go:113-121`, `banner.go:68-72`). An attached
  file of any size is read and written into the store inside `a.do` (`propkinds.go:171-180`). Exporting the whole
  library as Markdown runs in the frame (`settings.go:560`, `palette.go:173`). The library's figures are computed on
  the frame every 10 s while the stats screen is open; `docs/17` has that at 0.54 s for 10 000 pages.
- Fix: read outside, commit inside; export in a goroutine with a progress line.

### PERF-17 The usage figures are counted wrongly and written whole every 20 seconds
- **Medium for the figures, low for the hitch.** Read.
- `stats.go:163-179`: a frame gap between 0.5 and 1 s is rounded up to a second and also added to the fraction, so
  it is counted twice; such gaps are routine (the caret's blink, the autosave timer). Time per page is added only
  for gaps of a second or more, so the time spent writing in a page is not counted for the page at all: the figure
  measures idle time.
- Every 20 s the whole usage record, up to 800 days, is marshalled and written from the frame.
- Fix: accumulate one duration for the day and one for the page and add whole seconds from it; write from a
  goroutine, or only the changed day.

### PERF-18 Why `TestPerfBudget` fails by a hair (issue #70)
- **Medium, for the test's worth.** The mechanism read; one run alone on this machine passed at 28.4 ms a key.
- What is measured: twenty `tt.Type("x"); tt.Frame()` by the wall clock, the mean against a fixed 60 ms
  (`perf_test.go:113-139`). `Tester.Frame` builds frames until the view stops asking for one, so "a key" is the
  keystroke's frame plus every frame it invalidates, each a full software rasterisation of the window.
- Why it flakes: the number is pure CPU time with less than twice the headroom; packages run in parallel; a mean of
  20 has no defence against one preempted sample; the app's clock in this test is the wall clock, and the index
  warm-up budgets itself by wall time, so how much index work rides in the timed frames depends on load.
- And it does not test what its comment says ("a key costs the same in any library"): an absolute time cannot tell
  a slower machine from a regression.
- Fix, in order of value:
  1. Test the invariant as a ratio in one process: the same 20 keys on a 10-page library and on the big one,
     interleaved, `big ≤ 1.5 × small`.
  2. The minimum or the median, not the mean.
  3. Wait for the warm-up deterministically before timing, and set `a.clock = tt.Now`.
  4. Add a guard that cannot flake: allocations per key, or pages touched per frame, equal for both sizes.
  5. Keep the absolute 60 ms behind an environment variable and in a benchmark.

### PERF-19 Per-frame work that is small alone
- The sidebar's census walks every page each frame, closed branches too (`sidebar.go:838-856`); windowing is off
  while anything is dragged, so a drag in a tree of thousands builds them all.
- `drawTags` re-shapes every visible paragraph that has a tag or a link, each frame (`tags.go:104-142`).
- An inline table with a filter folds every row once per column plus one, per frame, and parses each cell's
  Markdown twice (`tablemodel.go:245`, `tables.go:239`).
- The three font stacks are resolved each frame (`main.go:663-682`); the font list sorts every installed family with
  a match in the comparator while it is open; where there is no menu bar, every accelerator is parsed each frame
  (`appcmds.go:215`).
- The auto-cover's overlay repaints the window eight times a second while a cover is in view
  (`autocover.go:960`).
- `help.go:46` and `statsscreen.go:307` call `a.commands()` per frame, with PERF-01's disk read in it.

### PERF-20 Sync's cost
- A full gzip backup before every pass and a full listing and index download on every pass: SYNC-06 and SYNC-07.

## What to measure first

1. Add a frame timer behind an environment variable: the duration of `viewFrame`, and a counter of frames per
   second, written to the log. Then sit on a page with the hands off the keyboard and read the frame rate (PERF-03,
   PERF-04). It should be near zero.
2. With the timer on: open ⌘P and move the mouse over it (PERF-01); open the gear menu (PERF-06); split a pane
   (PERF-07); open a dynamic collection (PERF-02).
3. For memory: `GODEBUG=gctrace=1` at start on the stress library, and a heap profile after the prewarm
   (PERF-11, PERF-12).
4. Counters that cannot flake, as tests: palette items built per frame, pages touched per frame, store writes per
   reorder. These protect the fixes better than a time budget does.
