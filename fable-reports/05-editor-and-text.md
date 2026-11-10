# 05 Editor and text

The keyboard layers, Vim, undo, paste, find, and what in the code stands in the way of issue #23 (Japanese,
Korean, Chinese, Arabic). What the editor can lose is in [01 Data integrity](01-data-integrity.md) (DATA-01 to
DATA-10); what it costs per frame is in [04 Performance](04-performance.md) (PERF-03, PERF-13).

Back to the [index](README.md). Nothing here was run with an IME, right-to-left or CJK text, or a non-US layout; the
findings that depend on those say so and name a five-minute manual check.

## How the editor is put together

- **Typing shortcuts are not edit hooks.** `track` (`main.go:1293`) compares the document's version, length and
  caret with the previous frame. Exactly one rune added before the caret runs the `/` menu, the block shortcuts,
  overtype and the chip shortcut; a larger growth is treated as a paste.
- **The caret is the app's.** MyGo's own caret is made invisible (`main.go:671`). Each painted frame `locate`
  (`caret.go:43`) finds the caret's paragraph, shapes it again with a copy of the editor's layout rules, and asks
  the layout for the caret's rectangle. Paragraphs over 30 000 runes are skipped.
- **Keyboard layers**, outermost first: app commands with accelerators (`appcmds.go`), user bindings (`keys.go`),
  the editor's `OnKey` hook (`clipmodes.go:70`), Tab (`tab.go`), MyGo's own keymap, then normal mode (`kb.go`): Esc
  parks the focus on an invisible anchor and single keys become commands; with the Vim setting `vim.go` adds
  motions.

## Findings

### EDIT-01 Vim: commands on the empty last line hit the paragraph above; motions split characters
- **Medium.** `dd` reproduced; the rest read.
- Where: `vim.go:93`, `:199` clamp the caret to `Len()-1` before asking for the paragraph. At the end of a text
  that ends with a newline, that position is the newline, which belongs to the paragraph before.
- Observed: text `abc\ndef\n`, caret on the empty last line, `dd`: the line `def` is deleted. The same range feeds
  `x`, `0`, `$`, `o`, `O`, `A`, `I`, `p`, `yy`.
- `h`, `l` and `x` move and delete by rune, so the caret can stop inside an emoji family, a flag, or between a
  letter and its combining mark, and `x` removes half of it. `j` and `k` move by paragraph, not by visual line, so a
  long wrapped paragraph cannot be walked. `w b e` treat a run of CJK as one word.
- `dd` or `x` on a line with a fold chip deletes the hidden block (DATA-06).
- Fix: `ParagraphRange(caret)`, which already clamps; step with the layout's cluster boundaries, as `caret.go:108`
  does; send `j` and `k` to the editor's own up and down.

### EDIT-02 Normal mode and the punctuation shortcuts assume a US keyboard; Esc has no opt-out
- **Medium.** The decoding read; what each layout produces is suspected.
- Where: `kbLetter` (`kb.go:368-441`) rebuilds characters from (modifier, key) pairs of the US layout: `?` is
  Shift+Slash, `:` is Shift+Semicolon, `$` is Shift+4.
- On Turkish Q: `/` is Shift+7, so it arrives with Shift and is decoded as `?` (help opens instead of find). `'`,
  `$`, `[`, `]` have no match. Letters the switch does not list (`ş ğ ü ö ç ı`, and every non-Latin script) map to
  an unknown key, so on a non-Latin layout normal mode understands only arrows, Enter and Esc.
- The accelerators on punctuation have the same problem: ⌘[ and ⌘] (back, forward), ⌘\ (sidebar), ⌘/ (fold)
  (`appcmds.go:72`, `:97`, `:130-131`). On Turkish Q these characters exist only behind Option or Shift.
- To confirm on a Mac with Turkish Q: Shift+7 in normal mode (expect find, get help); ⌘⌥8 (expect back).
- Esc from the text always enters normal mode (`main.go:945-950`), where unmodified letters act: `n` a new page,
  `t` tasks, `s` settings, `u` undo, and with Vim on `x dd p` edit. A writer who presses Esc by habit and keeps
  typing gets a new page and an undo.
- Fix: in normal mode read the typed character from the text-input event instead of decoding key and modifier; give
  back and forward a layout-independent alternate (⌘⌥← and →); make "Esc enters normal mode" a setting, on by
  default only with Vim motions.

### EDIT-03 Custom shortcuts (issue #61): what exists and what is missing
- **Medium.** Read.
- `keys.go` already binds keys to commands. Its limits:
  - the key must be ASCII `a-z`, `0-9` or US punctuation (`keys.go:55-63`);
  - built-in accelerators cannot be changed, only refused as conflicts (`:84-97`);
  - at least one modifier is required, so normal-mode keys and the leader map cannot be rebound;
  - bindings are typed as text; there is no "press the key" capture;
  - the list of reserved accelerators is kept by hand (`:74-81`) and has drifted from MyGo's editor keymap, which
    binds ⌘E, ⌘K, ⌘⇧X, 7, 8 and L (`richtext/input.go:425-470`);
  - `Settings.Keys` is keyed by the command's translated title (`keys.go:16`), so switching the language orphans
    every binding (see UX-04);
  - map iteration order decides which binding wins when two match (`:207`).
- For #61: give every command a stable id; store overrides by id; a capture field; let built-ins be rebound; derive
  the reserved list from the real keymaps; one table in the settings that lists every command with its key.

### EDIT-04 Issue #23: what stands in the way of CJK and right-to-left text
- **Medium.** Read; nothing was run with an IME or RTL text.
- **Composition and the caret.** richtext keeps the composition string outside the document (`richtext/ime.go`).
  `locate` lays out the paragraph from the document only, and the editor's own caret is invisible. So while
  composing, the app's caret stays where composition began, drawn as a block over the first composing character,
  and on the wrong line once the pre-edit text wraps. This is the first thing a CJK user sees. Confirm by typing
  with a Japanese IME.
- **Menus need an ASCII space.** `/` and `@` open only at a paragraph start or after a space, tab or `(`
  (`slash.go:542`); `#` only after a space or tab (`:533`). In text without spaces between words they never open
  mid-sentence. The full-width forms `／ ＠ ＃` are not recognised.
- **Words.** Word counts are `strings.Fields` (`main.go:1637`, `:1651`, `writing.go:21`): a Chinese sentence is one
  word, so the status bar, writing goals and Mizan's pace mean nothing there. Combining marks are not word runes
  (`caret.go:348`), so whole-word matching breaks Arabic and Devanagari words apart.
- **Right to left.** The caret's width is taken from the next cluster only if it lies to the right
  (`caret.go:108-112`); in RTL the block is drawn over the previous letter. Letter effects skip glyphs whose next
  edge is not to the right (`caret.go:313`). The erase logic assumes Backspace eats what is left of the caret.
- **Letter spacing** is applied to every span (`codecolors.go:35-45`), which breaks Arabic joining when on.
- **Layout.** The app uses none of MyGo's direction support: 52 `.Left(` / `.Right(` / left-or-right text
  alignments and four-value paddings throughout. See UX-04.
- **Fonts.** The stacks are `"<family>, <default>"` (`fonts.go:107-122`) and the presets are Latin faces; CJK and
  Arabic rendering depends on MyGo's system fallback, which was not tested.
- **Tests.** `caret_test.go`'s sample is Turkish and Latin: no emoji, no combining mark, no RTL, no CJK.
- Order to do it in:
  1. Get the composition from MyGo (its range and text, or just "composing" and the editor's own caret rectangle)
     and draw the plain caret from that while composing.
  2. Accept any non-letter, or a CJK letter, before `/ @ #`, plus the full-width forms.
  3. A word segmenter (UAX #29) for counts and motions.
  4. Mirror the caret box for RTL clusters.
  5. Add Japanese, Chinese, Korean, Arabic and emoji lines to the caret test.
  6. Then the layout direction and the language catalog (UX-04).

### EDIT-05 Undoing a paste conversion converts again and discards redo
- **Medium.** Reproduced.
- `track` treats any growth that ends at the caret as a paste (`main.go:1346`). An undo that restores text looks the
  same.
- Observed: insert ` [[A]] and [[B]]`, which makes two chips. ⌘Z restores both as text; the tail is at once
  converted again, which is a new edit and clears redo. A second ⌘Z is needed. The same mechanism reopens the `/`
  menu when an undo restores a single `/`.
- Also: two keystrokes that land in one frame (a slow frame, key repeat) are a growth of two, so `---`, the third
  backtick, `[*] ` and `]]` are not recognised and stay as typed.
- Fix: skip the shortcuts when the change came from undo or redo. Best is a hook in MyGo for "text inserted by
  typing or paste" with its range and cause; until then remember the last conversion's range and do not convert it
  again right after it was undone.

### EDIT-06 Pasting as Markdown, and ⌘V of an in-app copy, insert objects that are not drawn
- **Medium.** Reproduced.
- Objects read from Markdown carry data only until `dress` gives them a size and a draw function
  (`chips.go:163-204`). Three inserts skip it: `clipmodes.go:53`, `clipmodes.go:112` (`pasteOwn`),
  `fieldmenu.go:154`.
- Observed: paste as Markdown of `a [[Dune]] link`: the chip is a 1×1 box with no draw function. It is in the text
  and will be saved, but is invisible and cannot be clicked until the page is reopened. `pasteOwn` is the ⌘V path
  whenever the copied selection held a divider, a code block or a kinded task, so copying a section inside the app
  and pasting it makes its links and tables "disappear".
- Fix: `a.doc.Insert(a.dress(a.page, importMarkdown(md)))` in all three, and a test that pastes a link and a table
  and checks they draw.

### EDIT-07 Find and replace: what it does not see
- **Low.** Read.
- `findAll` searches the document's string (`find.go:71`): table cells, query results, embedded pages and
  everything behind a fold or outside a zoom are not found and not replaced, and the count says nothing about it.
- Matching is always case- and accent-folded, and so is replacing: "replace all `on`" also rewrites `ön` and `ÖN`.
  There is no "match case" or whole-word switch.
- Fix: unfold before a replace-all, or say "n more in folded blocks"; search table cells; add "match case", and
  replace exactly when the query has an upper-case letter or a diacritic.

### EDIT-08 Caret edge cases
- A paragraph over 30 000 runes has **no caret at all**: the app's is skipped (`caret.go:69-71`) and the editor's
  is invisible. Over that size, make the editor's caret visible again.
- The caret is always asked for with downstream affinity (`caret.go:84`): at a soft wrap after End, and at bidi
  boundaries, it can differ from where the editor believes the caret is. Suspected.
- The caret's colours are fixed hex values chosen for light and dark (`main.go:1527-1530`), not theme tokens.
- Nothing reads MyGo's reduce-motion preference: UX-06.

### EDIT-09 Smaller correctness points near the editor
- Recurring tasks drift: `AddDate(0, n, 0)` (`tasks.go:485`) moves a monthly task on the 31st to the 3rd, where it
  stays. `comesAgain` compiles its regexp on every call (`tasks.go:508`).
- `formatDay` replaces any `D`, `M`, `YY` in the literal part of a custom journal format; there is no escape
  (`journal.go:61-75`).
- A query block's height is fixed when it is made (`queryblock.go:62-66`); results that grow later are drawn past
  the block's box.
- The ✓ in a sub-menu does not move after a choice until the menu is reopened (`ctxmenu.go:65-70`).
- A restored tag pane comes back empty when the index is not yet built at start (`nav.go:99-104`).

### EDIT-10 Code quality in this area
- No `TODO`, `FIXME`, `XXX` or `HACK` in any Go, shell, JS or workflow file.
- Duplicates: `deleteBlock` (`blockops.go:198-214`) and `deleteRange` (`blocksel.go:95-110`) are the same function.
  The "first paragraph of a slice and its text" snippet is written out five times. The glass look is implemented
  three times. The "pages matching q" list is built four times in `slash.go` with caps of 60, 40, 40 and 6. Every
  caret preset re-implements `shift`, `reset` and the idle test by hand (PERF-03's helper would fold them).
- Names: `fold` is both the search-folding function and the method that folds blocks, beside two things named
  `unfold`. `blockOf` is `blockRange` with a comment that describes different behaviour.
- Large functions: `kbKeys` (`kb.go:182-349`), `slashView` (`slash.go:703-836`), `track` (`main.go:1293-1376`),
  `cellEditView` (`tables.go:704-905`). `track` interleaves five features over shared bookkeeping; each new
  shortcut has to re-derive it by hand, which is where EDIT-05 comes from.
- Stale comments: `caret.go:21` and `main.go:1367` say the layout mirrors richtext v0.7.4 (`go.mod` is at v0.8.4).

## Sound

Offsets are consistently runes, converted to UTF-16 in one place with a test. The caret is checked against the
editor's own on every rune of a sample, so a MyGo upgrade that moves it fails a test. Conversions are separate undo
steps, so ⌘Z gives the marks back. Keyboard-only reach is good: the block menu, fold, table cells, `gf` on a chip,
`f` hints. Links open through an allow list. No malformed input tried produced a panic.
