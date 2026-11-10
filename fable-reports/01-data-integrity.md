# 01 Data integrity

Every way found in which a page, a picture, a setting or a whole library can be lost or silently changed, outside
sync (for that see [02 Sync](02-sync.md)) and outside packaging (see PLAT-01 and PLAT-08 in
[07 Platforms and release](07-platforms-release.md)).

Back to the [index](README.md). Confidence words (reproduced, read, suspected, re-checked) are defined there.

## How a page is kept, in ten lines

- A page is `page.json`: `{"meta":{…},"body":[…]}`. The body is an array of paragraphs; `s` is MyGo's
  `richtext.ParagraphStyle` marshalled as it is, `r` the runs, `o` an object standing for one U+FFFC character
  (`body.go:25-56`).
- There is no block tree. What richtext has no style for is encoded in the paragraph's `ID`: `ryudit:hr`,
  `ryudit:code:<lang>`, `ryudit:raw:<kind>`, `ryudit:task:<kind>`.
- `Page.Text()` is the Markdown view of the body (`bodyMarkdown`, `markdown.go:214`). Search, tags, tasks,
  backlinks and MCP read it. `Page.SaveText(md)` converts Markdown back into a body and saves it.
- In the one-file library a write is `Store.WriteFile`: two checks, a version kept if none was taken in five
  minutes, one upsert, each its own implicit transaction. SQLite is in rollback-journal mode, `synchronous=FULL`.
- In a folder library a write is a temp file and a rename, with no `fsync`.
- An edit becomes `dirty` on the next frame (`track`, `main.go:1293`); `autosave` writes 0.7 s after the last edit;
  `flush` saves when a page is left and at quit.

## Findings

### DATA-01 Ordinary actions rewrite the whole page through Markdown
- **High.** Reproduced for what the conversion loses; the paths read and re-checked.
- Where: `tasks.go:152-173` (ticking a task in the Tasks view), `quick.go:96-102` (every quick note into today's
  journal page), `tags.go:404-413` (renaming a tag, for every page that has it), `mcp.go:414` and
  `mcp_server.go:312` (an agent's `update_page`). Each does `p.Text()`, edits lines, `p.SaveText(…)`.
- What one `body → Markdown → body` drops, each observed:
  - a done task's kind (`ryudit:task:star` is gone once the box is checked);
  - paragraph alignment and the app's own run mark `g`;
  - empty paragraphs between text (`[a, {}, {}, {}, b]` becomes `[a, b]`);
  - a code block's language when the block is inside a list or a quote, or indented;
  - a table's filter;
  - a file chip (it comes back as a plain link);
  - a link to a page whose title has `[` or `]`;
  - and everything in DATA-02 and DATA-08.
- Scenario: a page has a starred task that is done, an underlined word and a filtered table. Tick any other task of
  that page from the Tasks view. It is now a different page.
- Fix: change the body, not its Markdown. `toggleTask` decodes to `[]bodyPara`, flips `Checked` on the nth task
  paragraph and encodes. Quick capture appends paragraphs (there is already `appendLine`). `renameTag` rewrites run
  text in place. Keep `SaveText` for what really comes from outside. For agents either offer block-level edits or
  accept the loss openly (a version is already kept first). Then add the property test of DATA-10.

### DATA-02 Underline and literal `<…>` turn paragraphs and whole lists into raw HTML blocks
- **High.** Reproduced.
- Where: `markdown.go:49` (`inlineHTML`), `markdown.go:170-176` (`rawBlock`). richtext writes underline as `<u>` and
  falls back to HTML tags for bold or italic that Markdown cannot flank. The app's pre-pass then sees a tag and takes
  the whole paragraph away from richtext; `until(i, any)` runs to the next blank line, so for a list it takes every
  item.
- Observed: `bir <u>altı çizili</u> kelime` comes back as one code-styled paragraph showing the literal Markdown. A
  task list with one underlined word comes back as three code lines that are no longer tasks. Bold `Fiyat:` directly
  followed by `100` does the same. Text the user typed as `a <b> c` comes back as a raw block with a backslash in it.
- Fix: in `rawBlock`, skip a `<` preceded by an odd number of backslashes, and do not count the tags richtext itself
  reads (`u ins s strike del b strong i em br`). DATA-01's fix removes the trigger for pages that never leave the
  app.

### DATA-03 A page that cannot be read opens empty and is overwritten at the first keystroke
- **High.** Reproduced; re-checked.
- Where: `main.go:751-755` (`docFor` sets `a.saveErr` and goes on with an empty document), `main.go:733` (`open`
  clears `saveErr` on the next line that matters), `main.go:1375` (`track` clears it on every edit),
  `main.go:1616` (`autosave` holds back only while `saveErr != ""`).
- Observed: a `page.json` cut short opens with empty text and no error shown; one letter typed and saved replaces the
  file. The same happens one level down: `decodeBody` (`body.go:146-148`) returns the empty text for a body with one
  wrong-typed value, or with a future shape such as `{"v":2,"blocks":[…]}`.
- This undoes the care taken in the library: `SplitPageChecked` exists so that "what writes a page back must not take
  the empty body and write it over a file that can be mended" (`pagefile.go:34-36`).
- Fix: `docFor` returns the error. A page that failed to read opens read-only with a line that says so and offers the
  file and its versions; keep that in a `readErr` of its own, and have `save()` refuse while it is set. `decodeBody`
  returns `(Text, error)`.

### DATA-04 Opening such a page also deletes its pictures
- **High.** Reproduced; re-checked.
- Where: `main.go:756` calls `a.sweepPictures(p, body)` with the empty body; `images.go:297-299` removes every
  `resim-…` file the body does not name, with `Remove`, not the trash.
- Also, by reading: the sweep looks only at the current body, so pictures named only by older versions are deleted
  too; and with sync, a picture that arrives before the `page.json` that names it is deleted if the page is opened in
  between, which the next pass then sends on as a deletion (suspected).
- Fix: never sweep when the read failed; move swept files to the trash; skip files newer than a few minutes; count
  the page's versions as references.

### DATA-05 Moving, dragging or duplicating a block loses what is kept in paragraph IDs
- **High.** Reproduced.
- Where: `blockops.go:119-129` (`moveBlock`), `:154-176` (`moveBlockTo`), `:191-192` (`duplicateBlock`). MyGo's
  `Text.Replace` never copies IDs from the inserted text (`richtext/text.go:520-524`). Fold and zoom know this and
  use `replaceKeepIDs` (`zoom.go:106-133`); the block operations do not.
- Observed: a divider moved up becomes an ordinary empty line and the text beside it carries the divider's ID. A
  starred task moved down loses its star, which lands on its neighbour. A `go` code block loses its language. ⌘D on a
  code block makes one block of four lines, because a code block is split from the next only at an ID. Dragging a
  section loses all three at once.
- No test asserts an ID after a move (`blockops_test.go`, `blockui_test.go`).
- Fix: one helper for all four operations that collects `(offset, ID)` for each paragraph of the slice, replaces,
  then puts each ID back, the neighbour block included. A test that moves each kind of block and compares
  `encodeBody` before and after.

### DATA-06 A fold or zoom chip is one deletable character holding the hidden text
- **Medium, high while zoomed.** The end-of-text case reproduced; the rest read.
- "The file is always written whole" holds for every writer found, as long as the chip is still in the text.
- (a) A fold at the end of the text followed by Backspace on the empty last line: the first hidden line is glued onto
  the heading (`fold.go:46-52`, `:102-104`). Observed.
- (b) Deleting the chip deletes what it hides, without a word. While zoomed, ⌘A and any key removes both zoom chips:
  the rest of the page is gone, and autosaved 0.7 s later. Vim's `dd` on a folded heading does the same.
- (c) Typing before the top zoom chip makes `zoomChips` (`zoom.go:63`) stop seeing it; "Zoom out" then does nothing.
- (d) Folding inflates the writing figures: `save()` counts words on the folded text and `countWriting` only adds.
- Fix: in `track`, when an edit removed a fold or zoom chip, splice its hidden text back at that place (or refuse the
  edit); make ⌘A while zoomed select between the chips; count words on the unfolded text.

### DATA-07 A failed save is forgotten when the page is left
- **Medium.** Read; re-checked.
- Where: `main.go:725-733`: `open` calls `flush()` and then sets `dirty` and `saveErr` to their zero values whatever
  `flush` did. If the save failed (disk full, a locked database, a removed folder) the text survives only in the
  cache of 40 documents, which evicts on the stated assumption that "a page written is saved when it is left".
  `panes.go:682-690` makes the same assumption when a pane takes the use.
- Suspected: what is typed in a torn-off pane window is tracked and autosaved only by frames of the main window
  (`main.go:866`, `1002-1006`). To confirm: tear a pane off, minimise the main window, type, quit, reopen.
- Fix: `open` keeps `dirty` and the error when `flush` failed, and retries on a timer and at quit. `flush` decides by
  `doc.Version() != a.version || dirty`. Run `track` and `autosave` from whichever window builds the pane in use.

### DATA-08 Escaping bugs in the Markdown view: text that changes, grows or doubles
- **Medium.** Reproduced.
- (a) `\!` loses its backslash (`markdown.go:333-334`): `C:\!foo` becomes `C:!foo`.
- (b) `[^` gains a backslash on every round trip (`markdown.go:195-200`): `split on [^ ]+` becomes `\[^ ]+`, then
  `\\[^ ]+`. With DATA-01 that is one backslash per ticked task.
- (c) The sentinel `RYUDITBLOCK<n>X` (`markdown.go:41-43`) is plain text: a paragraph that says `RYUDITBLOCK0X`
  followed by a code block is exported as the code block twice.
- (d) `[[_private]]` and `[[a*b*c]]` become links to `private` and `abc` with italics between, because chips are
  found after richtext has parsed inline marks (`chips.go:142-160`). Going out, the backlink index reads `\_private`
  and finds no page.
- Fix: take chips, footnote references and held blocks out before richtext sees the text, as private-use
  placeholders with a per-call nonce, and put them back after; stop repairing richtext's output with regexps.

### DATA-09 Markdown import alters ordinary files
- **Medium.** Reproduced; the BOM case read.
- CRLF: `parseBody` splits on `\n` only (`markdown.go:56`), so `---\r` is not a divider; every file edited on Windows
  loses all its dividers and every code line keeps a `\r`.
- A divider directly after a closing fence is dropped (`markdown.go:63`, `:96`).
- A kinded task box inside an indented code block is rewritten, and because the counts then differ
  (`taskkinds.go:155`) every task on the page loses its kind.
- Front matter after a BOM is not seen (`frontmatter.go:22`) and becomes page text.
- Smaller: a hard break becomes a paragraph break, a link's title is dropped, `5.` lists restart at 1, `$$x$$` typed
  as text comes back as a math block.
- Fix: normalise once at the door (CRLF, BOM) in `SplitFrontMatter` and at the top of `parseBody`; track "the
  previous emitted line is blank or a held block"; run `taskKindsIn` with a fence state.

### DATA-10 The body is not forward-compatible below the object level, and nothing tests the direction that matters
- **Medium.** Reproduced.
- Unknown objects are kept (good, and tested). Unknown fields of a paragraph, a style or a run are dropped on save:
  `{"s":{"heading":1,"future":true},"r":[{"t":"x","color":"red"}]}` is written back without `future` and `color`.
- `appendLine` (`body.go:208-216`, used by bulk tagging) strips the data of an object kind it does not know, and for
  a body it cannot unmarshal it starts from nothing: the page is replaced by the tag.
- The paragraph style is MyGo's struct with MyGo's JSON tags: a rename upstream changes the file format silently.
- Why it matters now: the format marker is not synced, so an older app does open pages written by a newer one on
  another device.
- Tests: there is no fuzz or property test. `TestMarkdownKept` checks `md → text → md`, the old storage direction. A
  30 000-input random run found no panic, but 7.4 % of bodies changed after one `body → md → body`.
- Fix: own the style struct; give `bodyPara` and `bodyRun` an `extra map[string]json.RawMessage`; `appendLine`
  returns an error on a body it cannot read. Add `FuzzBodyRoundTrip` with the declared losses in one table, which
  then documents the format, and `FuzzParseBody` for panics and timeouts.

### DATA-11 An unreadable settings file is replaced by defaults without a word
- **High.** Read; re-checked.
- Where: `mcp.go:168`, `:171`, `:176`: three `json.Unmarshal(data, &a.settings)` with the error dropped. Then
  `:185-191` makes a new token and saves, which writes defaults over the file that could not be read.
- Lost with it: custom keys, macros, profiles, the writing figures, the sync mode and target, the per-collection
  agent permissions (which fall back to read-write), and the welcome plays again.
- Also: `main.go:652` and `appthemes.go:77` index with `% n`, which keeps the sign in Go, so `"theme": -1` from a
  file, an import or a profile panics on every frame.
- Fix: on an unmarshal error keep the bad file under another name, say so in a toast, and do not save until the user
  changes something. Add a `Version` field. One `normalize()` after every load, import and profile that clamps
  indexes.

### DATA-12 One damaged `collection.json` keeps the whole library from opening
- **Medium.** Reproduced.
- Where: `library.go:240-242` returns the error; `main.go:460-461` is `log.Fatal`. The app exits at start with a log
  line and no window. A damaged page is handled far better (it is listed, and `Body()` refuses to write over it).
- Fix: skip the collection, carry it on `Library` as damaged for the app to show, and never `log.Fatal` on library
  content. `main.go` has six `log.Fatal` calls before the window exists (`:444`, `:461`, `:470`, `:476`, `:609`,
  `:628`); each should become a window that says what is wrong.

### DATA-13 `JoinPage` turns an invalid body into a zero-byte page and reports no error
- **Medium.** The mechanism reproduced; no live trigger found.
- Where: `pagefile.go:85-86` drops the marshal error; `body.go:113` (`encodeBody`) drops its own. `p.Save` with a
  broken body returns nil, the file is empty, the title is gone, and `pagefile.go:38` reads the empty file as a valid
  empty page.
- Fix: `JoinPage` returns an error, or `Page.Save` checks `json.Valid` and refuses; `encodeBody` returns its error.

### DATA-14 No WAL: a reader on the library file blocks a save for five seconds, then fails it
- **Medium.** Reproduced (5.1 s, then `SQLITE_BUSY`).
- Where: `store_sql.go:40` sets only `busy_timeout(5000)`. `backup_sql.go:13-18` reads the whole file on a second
  connection (`VACUUM INTO`), before every sync pass and daily, while `a.save()` runs on the window's thread.
- Fix: `_pragma=journal_mode(WAL)`. `backup.go:237-238` already removes `-wal` and `-shm` on restore; check that the
  read-only snapshot connection still opens a WAL file.

### DATA-15 A library path with `#`, `?` or `%` opens a different file, or none
- **Medium.** Reproduced.
- Where: `store_sql.go:40` and `backup_sql.go:13` put the path unescaped into a `file:` URI. A folder named
  `My #1 notes` makes a stray database file named `My `; the `chmod 0600` misses it; a restore goes to the real path,
  which the app never reads; `50%25 done` fails to open and the app exits.
- Fix: escape `%`, `?`, `#` in one helper used by both; a test with such a folder name.

### DATA-16 Reordering rewrites every sibling in its own transaction and keeps a version of each
- **Medium.** Reproduced: one page moved to the top of 50 made 50 writes and 50 version rows.
- Where: `library.go:914-929` (`Renumber`), `properties.go:352-364` (`MoveOnBoard`), `:400-412` (`SetManual`).
  Not atomic (a crash leaves half renumbered), on the window's thread, and it fills the history and the time-lapse
  with copies whose text is identical. See also PERF-15.
- Fix: a batch on `Store` so one reorder is one transaction; in `snapshotBefore` skip the copy when only the meta
  changed; later, sparse ranks so only the moved page is written.

### DATA-17 Versions are orphaned by `Remove` and inherited by the next page at the same path
- **Low.** Reproduced (the inheritance).
- Where: `store_sql.go:264` deletes only from `nodes`. A page the sync moved to the trash loses its history on
  restore; a new page that gets the same folder name within seven days shows another page's text in its history.
  There is no cap per page or in bytes, and errors in `keep`, `SaveNamed` and `snapshotBefore` are dropped.
- Fix: delete or move the versions with the file; prune at open with a per-page cap; check the insert's error.

### DATA-18 A folder library's writes are not crash-safe as the interface promises
- **Low, folder libraries only.** Read.
- Where: `store.go:46-52`: `name + ".tmp"`, `os.WriteFile`, rename; no `fsync` of the file or the folder, and a fixed
  temp name that two writers share. After a power cut the page can be zero bytes, which reads as a valid empty page,
  and a folder library has no versions.
- Fix: `os.CreateTemp` beside the file, `Sync()` before the rename; treat a zero-byte `page.json` as damaged.

### DATA-19 Importing the old folder library is neither atomic nor retried
- **Low.** Read.
- Where: `main.go:435-447`: `synchronous = OFF`, `CopyTree` file by file, `log.Fatal` on the first error. A failed
  copy leaves a store file that exists, so the next start skips the import and opens part of the library without a
  word.
- Fix: copy into `<file>.importing` and rename when it is whole.

### DATA-20 A failed `Restore` leaves the item neither in the trash nor in the library
- **Low.** Read.
- Where: `library.go:734-741` moves the folder, removes the trash note, and only then reads `page.json`. Trash items
  without a `page.json` do occur (SYNC-15). Restoring a page whose file is damaged rewrites it empty
  (`library.go:742`).
- Fix: check before the move, or move back on failure; use `SplitPageChecked`.

### DATA-21 Smaller points in the store
- `Collection.Creating` and `Seen` lack `json:"-"` (`library.go:66`, `:84`): they are written to `collection.json`,
  so every save changes its bytes. Reproduced.
- `Slug` cuts at byte 60 in the middle of a character (`library.go:532-533`): a long Japanese title makes an invalid
  folder name and `Mkdir` fails on macOS. Reproduced.
- `Store.Stat` on a folder returns "does not exist" outside the start-up snapshot (`store_sql.go:305-308`). No
  caller today; a trap for the next one. Reproduced.
- `WriteFile` and `Rename` are check-then-write across statements while the sync goroutine uses the same store:
  orphan rows are possible. Read, not reproduced.
- `applyRestore` removes `-wal` and `-shm` but not `-journal`: in today's journal mode a hot journal of the replaced
  file would be applied to the restored one. Suspected.
- `bulkAddTag` on a body that does not parse replaces the page with the tag line (`bulk.go:153`). Read.

### DATA-22 Journal days follow the time zone of the moment
- **Low to medium.** Read.
- Where: a day's page is found by `Created` in the local zone at the time of looking (`journal.go:94-97`,
  `calendarview.go:38-39`). After travel a late-evening page moves to another day, `journalPage(today)` misses it,
  and a second page for today is made.
- Fix: store the day as a date string in the page's meta.

## What to add to the tests

Nothing in the suite covers: opening an unreadable page (DATA-03, DATA-04), IDs after a block move (DATA-05), a fold
at the end of the text (DATA-06), a failed save on navigation (DATA-07), CRLF input (DATA-09), unknown run and style
fields (DATA-10), a damaged settings file (DATA-11), a damaged `collection.json` at open (DATA-12), a store path with
URI characters (DATA-15), `Slug` at all, or anything in `store_js.go`. `TestFSContract` (`store_test.go:33`) is the
right home for the store ones.
