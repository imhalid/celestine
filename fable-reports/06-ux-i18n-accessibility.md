# 06 UX, i18n and accessibility

Errors the user never sees, actions with no way back, the language system, contrast and motion, and what a notes
app usually has that this one does not yet.

Back to the [index](README.md). The page-history files (`timemachine.go`, `tmside.go`, `tmdiff.go`,
`historyui.go`) hold uncommitted work for issue #71 and were reviewed as they are on disk.

## How these parts work

- **Settings** are one struct, kept as a row of the library file (or `settings.json`), secrets moved to the
  keychain. `settingDefs()` (`cmdreg.go:92`) lists every setting as a command; a test holds the struct and the list
  together. The settings screen draws its own rows by hand.
- **Commands**: `appCommands()` is one list for the menu bar, the window's shortcuts, the palette and MCP.
- **Menus**: one context menu (`ctxmenu.go`) drawn by one panel (`menupanel.go`).
- **`T()`** (`i18n.go:27`): two maps. `enText` maps Turkish keys to English (the older strings); `trText` maps
  English keys to Turkish (the new ones). The dictionary is assembled in eight `init` blocks, first writer wins.
- **Themes**: 13 themes of literal colours; `skin()` re-maps Ryugo's grays between the theme's page and text
  colours by luminance position.

## Findings

### UX-01 One field takes about 85 unrelated errors, shows them as "could not save", and stops autosave
- **High.** Read.
- `a.saveErr` is assigned in 29 files: delete for good, pin, move, rename, duplicate, restore a version, set a
  view, new collection, cover, journal.
- It is shown in one place: `editing.go:74` returns `"kaydedilemedi: " + a.saveErr`, drawn in the status bar
  (`main.go:1577`), only when "saved" is not hidden and the status bar is not hidden, in Turkish with no `T()`.
- It is cleared by the next keystroke (`main.go:1375`) and by opening a page (`main.go:733`).
- And `autosave` returns while it is set (`main.go:1616`): any of those errors suspends autosave of the open page
  until the user types again.
- Scenario: "delete for good" fails on a locked file. The status bar says "Kaydedilemedi: not in the trash", or
  nothing at all with the status bar hidden; the open page's pending edit is not saved meanwhile.
- Fix: keep `saveErr` for a page's save only. Add `a.fail(err)` that raises an error toast (the toast system
  already has the kind) and use it everywhere else. See also DATA-03 and DATA-07, which need the same separation.

### UX-02 Destructive actions with no confirmation and no undo
- **High.** Read; the delete re-checked.
- **Delete for good**: `trash.go:155-156`, an icon-only button beside "Restore", and `ctxmenu.go:172-173`, act at
  once. One mis-click beside Restore loses a page and its sub-pages.
- **Reset all settings**: `settingsx.go:364-367`, `palette.go:154`. The reset includes the language (a Turkish
  user is switched to English), MCP (the server stops) and the sync mode (sync is turned off). There is no undo,
  though single settings have one.
- **Applying a profile**: `settingsx.go:193-195` deletes every profile key before applying, so the built-in
  profiles, which hold only sizes, reset the theme, the colour theme, the caret, the three typefaces and the
  corners. No undo.
- **Deleting a property**: `propsui.go:404-407` removes it from the whole collection at once.
- **Collection settings**: Esc or a click outside discards the draft (`collsettings.go:77-79`), a long template or
  JSON edit included, without asking.
- **Page export** overwrites same-named files beside the chosen `.md` (`pagemenu.go:179`).
- Fix: one `a.confirm(title, body, danger, run)` for delete-for-good and reset-all; snapshot the settings before a
  reset or a profile and attach it as the notice's undo; an undo toast for a removed property (the values stay in
  the pages, so putting the schema entry back restores them); "Discard changes?" when the draft differs; export
  attachments into a folder of their own.

### UX-03 English is the default, yet many screens show Turkish
- **High.** Read.
- A sample of what never passes through `T()`:
  - 33 buttons with a Turkish literal: "Kaydet", "Uygula", "Özellik ekle", "Dışa aktar", and the whole update
    dialog ("Tamam", "Yeniden başlat", "Sonra", "Bu sürümü atla", "Şimdi güncelle") (`updateui.go:79-102`, `:189`);
  - the first-run tour's bodies and its "İleri" and "Bitti" (`help.go:111-121`);
  - dialog and tab titles: "Kısayollar", "… / Ayarlar", "Genel", "Sayfalar", "Kod";
  - field labels and help lines in the collection settings;
  - tooltips: "Kalın (⌘B)" (`selbar.go:114-118`), the window controls;
  - the status bar's "harf", the history's footer hints, the states of setting commands;
  - in the editor: the fold chip "⋯ N satır" (`chips.go:67`), "Tablo", "Sorgu", "Sayfa", "Aç", "Katla", and the
    callout templates, which put Turkish text **into the document** (`blocks.go:150-155`).
- Why the tests miss it: `TestEnglishCoversScreen` checks that a literal has an entry in the dictionary, not that
  it is passed to `T`. A raw button labelled "Kaydet" passes because "Kaydet" is in the dictionary. Only the
  settings screen is rendered and inspected.
- Fix: thin wrappers that translate (`a.button`, `field(label, help)`, the bar button calling `T`); a test that
  forbids a string literal with letters as the label of a button, a dialog title or a field unless wrapped; and a
  rendered English test per dialog.

### UX-04 The language system does not scale past two languages, and already produces wrong text
- **Medium; high for issue #23.** Read.
- **Keys without context.** One Turkish string, two meanings: `T("Yeniden başlat")` on the "show the welcome
  again" row shows "Restart", the updater's, not the "Start again" that `welcome.go:585` tries to register. The
  workaround elsewhere is a trailing space in the key: ten such entries, none of which any code looks up.
- **Behaviour keyed on Turkish text.** Toast kinds come from Turkish suffixes ("ndı", "ldi", " yok", "hata")
  (`toast.go:45-63`): notices written in English, which is the rule for new code, are all "info", and a page title
  inside a notice can flip the kind. Palette groups come from Turkish title prefixes (`palgroups.go:15-50`):
  commands with English titles fall into "Uygulama". Menus filter by a title being equal to "Yan bölmede aç"
  (`ctxmenu.go:33`), "Yeniden adlandır" (`titlemenu.go:35`), "Hızlı not" (`appcmds.go:244`).
- **Typed commands know only Turkish**: "editor typeface Georgia" does nothing in English (`cmdreg.go:457-460`).
- **State keyed by the translated title**: recent commands and custom keys (`palette.go:835-836`, `:243`). After a
  language switch the recents are empty and the key hints vanish.
- **Word order by concatenation**: `T("Satır ")+n+T(", sütun ")+m` (`statusbar.go:34`) and others.
- **No plural rule.**
- **Dates and numbers ignore the language**: 15 `time.Format` layouts with month or weekday names print "2 Jan"
  and "Mon" in a Turkish window; Monday is hard-coded as the first day (`propsui.go:236`).
- **Key hints are Mac glyphs on every platform**: `accelLabel` (`appcmds.go:120-148`) always writes ⌘ ⌥ ⇧; the
  help page, tooltips and the tour are literal "⌘P".
- **User data gets Turkish defaults in English**: "Yeni koleksiyon", "Adsız", "-kopya", "(kopya)", the view names
  "Liste", "Tablo", "Pano".
- The cost of a third language: every string is a map key in one of two source languages, so a third needs two more
  maps and a three-way `T`. MyGo already ships what is missing (`mygo/i18n` catalogs with plural formats,
  `App.SetLocale`, regional date formats and first weekday, `Context.Direction`, logical start and end paddings),
  and the app uses none of it.
- Fix: stable message ids in one catalog per language, loaded through `mygo/i18n`, the two existing languages
  generated from today's maps; an explicit kind on notices (`a.note(kind, msg)`); a stable id on every command for
  grouping, recents and keys; dates through the region; one `keyLabel(accel)` that writes Ctrl and Alt on Windows
  and Linux.

### UX-05 Contrast of muted text
- **Medium.** Computed from the literal theme colours, with the muted token placed where `skin()` puts it; ±0.2.
- Muted text is setting help lines, palette sub-lines, hints and timestamps, at 11 to 13 px. WCAG asks 4.5:1.

  | Theme, light | On the page | On the sidebar |
  | --- | --- | --- |
  | Gruvbox | **3.9** | **3.3** |
  | Sepya | **4.3** | **3.7** |
  | Kiraz | 4.8 | **4.1** |
  | Kehribar | 4.9 | **4.2** |
  | Orman | 4.7 | **4.2** |
  | Derin Deniz | 4.9 | **4.3** |
  | Aurora | 4.9 | **4.4** |

  The other light themes are at or just above 4.5 on the sidebar; every dark theme passes.
- On top of that, muted is used at reduced alpha for real text: 0.5 for the "Boş" placeholders (`propsui.go:96`,
  `:283`) is about 2.0:1 in every theme, the default included.
- Fix: compute `Muted` by a target contrast in `skin()` instead of by luminance position; use it at full alpha for
  placeholders.

### UX-06 Motion cannot be turned off
- **Medium.** Read.
- There is no reduce-motion setting, and nothing reads the system's preference, though MyGo exposes it
  (`ui.Preferences.ReduceMotion`). What cannot be turned off: the toast shader, the scrollbar's jelly, the
  auto-cover's "life" (on by default), sidebar and menu springs, the MCP pulse. "Hafif mod" covers carets only.
- Fix: one setting, on by default when the system asks for it, that gives a plain caret, no wobble, no confetti, no
  toast shader, no cover life, and calls `c.DisableAnimations()`.

### UX-07 Keyboard reach and assistive technology
- **Medium.** Read; no screen reader was run.
- Toast buttons ("Undo", "Open") have no key (`toast.go:226-236`): an undo offered for a setting changed from the
  palette cannot be taken without the mouse. Give the newest toast's action a key.
- The editor is made with the focus ring off (`main.go:1087`) on the grounds that the caret shows focus. The
  normal-mode anchor is a focusable 0×0 box with no role (`kb.go:80`): in normal mode the focus is on nothing
  visible, and a screen reader lands on a nameless "Klavye kipi".
- What is painted over the editor is not in the accessibility tree: dividers, task-kind boxes, callout tints, table
  cells (a table is one attachment named "Tablo"), query rows, embedded text, the code-language label. A table's
  content cannot be reached except through the cell editor.
- The sidebar and panel grips carry their element key as their label ("sidebar-grip") (`edges.go:49`); the star in
  the font list is a hit region of a row; ←→ on sliders is still open in `docs/16`.

### UX-08 Keeping a copy by name is no longer reachable
- **Medium.** Read. Work in progress (#71).
- `openHistory` (`historyui.go:48`) has no caller outside tests. The page menu's "Sürüm geçmişi…" opens the time
  machine, whose foot has no name field; the comment at `timemachine.go:23` says naming "is the version dialog's",
  but that dialog now opens only for server versions, which hide the field.
- Fix: a "Keep this as…" button in the time machine's foot, then delete `openHistory`.

### UX-09 Settings without a row, and three sets of limits for one setting
- **Low.** Read.
- `letterSpacing`, `caretBlink`, `whiteInSel` and `syncMinutes` exist only as palette commands. The test checks
  "every field is a command", not "every command has a row".
- Font size: the slider is 13 to 24, the command and the getter 12 to 28. Line height and page width differ
  likewise. A value set by command pins the slider's thumb at an end.
- Settings search finds only the defs: MCP access, the sync fields, profiles, backups and the update address are
  not found by it.
- Fix: draw the leftovers from `settingDefs()` under their tab; sliders take their limits from the def.

### UX-10 The history says "the same" when it did not look
- **Medium.** Read. Work in progress (#71).
- `DiffLines` compares the first 1 500 lines only (`history.go:95-101`). `tmdiff.go:109-116` then shows "This copy
  is the same as the page now." when every difference lies after line 1 500; `historyui.go:226-228` likewise.
- Enter restores at once (`timemachine.go:163`). It can be undone, since the page is kept as a copy first, but
  nothing says so, and a failed restore closes the history all the same.
- A version whose data cannot be read: the Restore button does nothing, silently (`historyui.go:112-115`).
- On a phone the bars are 6 px high past about 80 copies, far under the touch size.
- Fix: say "compared up to line 1 500" when either side was cut, or diff by paragraphs; say that a restore can be
  undone; show the error.

### UX-11 Smaller points
- The language choice changes the window at once but the menu bar only at restart (known, `docs/16` N4).
- Import and export errors are a raw "Olmadı: " plus Go's error text (`settingsx.go:282`, `:388`).
- The collection settings dialog is fixed at 560 wide, the version dialog at 940, help at 680: no phone branch.
- The page "…" menu is placed from the window's bottom-right corner (`pagemenu.go:28`).
- Line numbers used as element keys, already stale: `"menupanel:91"`, `"palette:704"`.
- `T()` is called from goroutines while `lang` is an unsynchronised global (`banner.go:62`, `propkinds.go:157`,
  `settingsx.go:275`).

### UX-12 Against a notes app's baseline
Each was searched for in the code before being called missing.

| Feature | State |
| --- | --- |
| Export Markdown: a page, the library as a folder or a zip | present |
| Export HTML or PDF, print | **missing** |
| Attachments: file block, file property, pictures | present, but a file in the one-file library does not open (SEC-02) |
| Search inside attachments | missing |
| Templates per collection | present |
| Reminders and notifications | **missing** |
| Backlinks | present; a graph view was decided against |
| Import: a Markdown folder, Obsidian syntax | present |
| Import from Notion, Evernote, Apple Notes | missing |
| Page icon or emoji | missing (collections have icons) |
| Pinned or favourite pages | missing (collections can be pinned) |
| Empty the trash, as a command | missing (the library purges after 30 days by itself) |
| Encryption of the library at rest | missing (sync is end-to-end encrypted) |
| Spell check | missing, blocked on MyGo |
| Right-to-left text, CJK input | EDIT-04 |
| Mobile | the web build under 760 px and an Android WebView (PLAT-13 to PLAT-17); no iOS |
| Collaboration, AI, terminal | open epics #51, #27, #31 |

### UX-13 The docs' own lists: done, missing, stale

| Doc | Item | State |
| --- | --- | --- |
| 07 | Themes as data files | missing: themes are a literal list (`appthemes.go:28`). This is also the one extension point the project's decisions allow |
| 07 | A default collection for ⌘N | missing |
| 07 | Per-collection MCP access | done; the doc is stale |
| 07 | Date and time formats, first weekday | missing |
| 13 | A search box at the top of the page menu | missing |
| 13 | Person mention; task kind in the `/` menu | missing |
| 14 | A link card on paste | missing |
| 14 | Pin a page | missing |
| 14 | A list of what agents did | missing: a count and an undo only |
| 14 | Export HTML or PDF; a collection as one file | missing |
| 14 | Ryugo pinned to a tag | open (QA-11) |
| 16 | Caret blink rate; white space only in the selection | done, palette only (UX-09); the doc is stale |
| 16 | ←→ on settings controls; a VoiceOver audit | missing (UX-07) |
| 16 | Menus change only at restart; other screens not checked in English | open, and larger than the doc says (UX-03) |
| 23 | Secrets not in the keychain | done; the doc is stale |

### UX-14 Maintainability of the screens
- Long functions: `drawAutoCover` 394 lines, `settingDefs` 204, `commands` 166, `paintLife` 147.
- The same "title and help" block is written out in six places in the settings; the "copy row" closure twice, with
  different radii; pill buttons built by hand four times.
- Dead or stale: `cmdreg.go:442` has `… && false`; `save1` (`cmdreg.go:88`) has no caller; `hidetitle.go` holds one
  function and a comment about something it no longer does; `mobile.go:121` keeps an unused value alive;
  `collsettings.go:553` has a `default:` that cannot be reached; a custom `itoa` (`collsettings.go:815`) returns
  "0" for negatives.
- Comments out of date: `i18n.go:8-12` says the long help lines are still Turkish only and that a test checks
  nothing is made twice; it does not.

## Sound

The "every setting is a command" invariant is real and tested. Single setting changes from the palette carry an
undo, and theme and caret preview as the selection moves. One context menu and one menu panel for the whole app;
one command list feeding the menu bar, shortcuts, the palette and MCP. Restoring a version keeps the present page
as a copy first. The settings handle narrow windows, and there is a deliberate phone layout with touch sizes. The
pointer-versus-keyboard logic in the palette avoids the classic "resting pointer steals Enter".
