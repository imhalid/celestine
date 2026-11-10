# 09 Action plan

Everything in the other reports, in the order it should be done, grouped the way the project works: one issue per
piece of work. Back to the [index](README.md).

Sizes are rough judgement, not estimates from doing the work: **S** is an afternoon with its test, **M** a day or
two, **L** a week or a change that needs a design first.

## Step 0: measure and guard, before fixing

These make the rest safe to do and are small.

| Issue to open | Covers | Size |
| --- | --- | --- |
| CI on push and pull request: the gate, `-race` on the library, the wasm build, macOS and Linux | QA-01, PLAT-07 | S |
| Fix the two tests that fail at `HEAD` on macOS | QA-02 | S |
| A frame timer behind an environment variable; record the idle frame rate | "What to measure first" in [04](04-performance.md) | S |
| `FuzzBodyRoundTrip` and `FuzzParseBody`, with the declared losses in one table | DATA-10 | M |
| `.gitignore`: `release.key` and the release leftovers | SEC-11 | S |

The round-trip test will fail at first. That is its job: each of DATA-01, DATA-02, DATA-08 and DATA-09 then turns
one row of the table green.

## Step 1: nothing a user wrote may be lost

In this order. The first three are the ones a user can hit today with one click.

| Issue to open | Covers | Size |
| --- | --- | --- |
| Sync: a collection switched off must not be deleted on the other side | SYNC-01 | S |
| Sync: start from an empty state when the remote or account changes | SYNC-02 | M |
| An unreadable page opens read-only; never sweep its pictures | DATA-03, DATA-04 | M |
| Tasks view, quick note and tag rename edit the body, not its Markdown | DATA-01 | M |
| Underline and literal tags no longer become raw HTML blocks | DATA-02 | S |
| Block move, drag and duplicate keep paragraph IDs | DATA-05 | S |
| The default library is never inside the program's folder; move one that is | PLAT-01 | M |
| A damaged settings file is kept and reported, not replaced | DATA-11 | S |
| A failed save survives leaving the page; one field for save errors only | DATA-07, UX-01 | M |
| A damaged `collection.json` does not stop the app; no `log.Fatal` on library content | DATA-12 | M |
| Deleting a fold or zoom chip does not delete what it hides | DATA-06 | M |
| Confirm delete-for-good and reset-all; undo for profiles and removed properties | UX-02 | M |
| Web store: tell "absent" from "failed"; never write over a library that failed to read | PLAT-08, PLAT-09 | M |
| The mass-removal guard: small libraries, empty hashes, and a "send mine up" action | SYNC-03 | M |

## Step 2: before more people install it

| Issue to open | Covers | Size |
| --- | --- | --- |
| A licence, and third-party notices in every release | QA-09, QA-10 | S |
| Updater: restart that restarts; download beside the executable; a rollback keyed on the new version's own stamp | PLAT-03, PLAT-04, PLAT-05, SEC-03 | M |
| Updater: sign platform, kind and channel; wire the beta channel | SEC-04, SEC-05 | M |
| Release workflow: a signing job apart from build and test; actions pinned by SHA; checksums always | SEC-10, PLAT-06 | M |
| MCP bridge: the keychain before `runMCP`; a read-only read of the settings; a real exit code | SEC-01 | S |
| File chips and file properties open only local names; attachments open from the one-file library | SEC-02 | M |
| MCP: the destination of move commands; names in `list_commands`; the clipboard command | SEC-07, SEC-08 | S |
| Backups, crash logs and the config folder are private | SEC-15 | S |
| Cover and banner names are checked when read | SEC-12 | S |
| The rate limiter's cleanup deletes only its own window; refuse failed admin logins | SYNC-08, SYNC-13 | S |
| WAL for the library file; escape the path in the URI | DATA-14, DATA-15 | S |
| Decide the distribution path and make README, docs/16 and docs/19 agree | PLAT-02 | S |

## Step 3: the app at rest should be at rest

Measure with step 0's timer before and after each.

| Issue to open | Covers | Size |
| --- | --- | --- |
| The scrollbar and the carets ask for a frame only while something moves | PERF-03, PERF-04 | M |
| The palette keeps its list; no disk read in a frame (#16) | PERF-01 | M |
| Search results and dynamic collections are cached; snippets work on a window | PERF-02 | M |
| Two scrollbars in a split (#14): boxes per pane and per editor | PERF-05 | M, with a MyGo patch |
| The gear menu (#15): measure, then theme change without re-keying the tree | PERF-06 | S to M |
| Settings are always debounced; the title is written with the page | PERF-07 | S |
| The watcher scans outside the lock | PERF-08 | S |
| Collection views keep their filtered and sorted pages; windowed lists | PERF-09 | M |
| `TestPerfBudget` as a ratio, with a counter that cannot flake (#70) | PERF-18 | S |
| Start-up reads only JSON blobs; the prewarm streams; `links.plain` is pruned | PERF-11, PERF-12 | M |
| The pre-sync backup only when a pass will pull or remove, in its own bucket | SYNC-06 | S |
| Reorder in one transaction; no version when only the meta changed | DATA-16, PERF-15 | M |

## Step 4: the edges of the format and of sync

| Issue to open | Covers | Size |
| --- | --- | --- |
| Markdown in: CRLF, BOM, dividers after fences, task boxes in code | DATA-09 | S |
| Markdown out: escapes, the sentinel, link titles with marks | DATA-08 | M |
| The body keeps unknown fields; the app owns its style struct | DATA-10 | M |
| Seal files with their name as associated data; a generation in the index | SYNC-04 | M, with a migration |
| Write the index before the objects | SYNC-05 | M |
| A pass that does not list when nothing moved | SYNC-07 | M |
| Transfers: named 401 and 413, a local size limit, idle timeouts | SYNC-11 | S |
| Passphrase change that re-seals, or none | SYNC-09 | S |
| Stronger stretching under a version label | SYNC-10 | M |
| Worker tests that run by default | SYNC-20 | M |
| Paste as Markdown and in-app paste dress their objects | EDIT-06 | S |
| Undo does not re-convert | EDIT-05 | S to M |
| Vim on the last line; motions by cluster | EDIT-01 | S |

## Step 5: larger pieces that need a design first

Each of these is a decision as much as a change. They are listed with what the review found that bears on the
decision.

- **A language catalog with stable ids** (UX-04, UX-03). Needed before a third language (#23) and before commands
  can be rebound safely (#61, EDIT-03), because commands, recents, key bindings, palette groups and toast kinds are
  all keyed on translated Turkish titles today. MyGo's own `i18n` already has catalogs, plurals, regions and
  direction. This is the one change that unblocks three open issues.
- **CJK and right-to-left** (#23, EDIT-04). After the catalog. The first visible step is small: draw the plain
  caret from the editor's own rectangle while composing.
- **Keyboard on non-US layouts, and "Esc enters normal mode" as a setting** (EDIT-02). Needs one check on a Turkish
  Q keyboard first.
- **Themes as data files** (UX-13). The only extension point the project's decisions allow, and not built yet.
- **Reduce motion** as one setting that follows the system (UX-06), with a contrast pass on muted text (UX-05).
- **Web and Android as real targets** (PLAT-08 to PLAT-17). The store's error handling first; then the permission,
  the backup flag, back and share.
- **The suite's speed** (QA-03). `-short` first; parallel only after `lang`, `phone` and `rowH` leave package
  scope, which the catalog change also wants.
- **The rebrand** (QA-16). Put the name in one constant first; do not change the encryption labels.

## Docs to bring up to date in passing

`CLAUDE.md` (the fork line, the default library), `docs/12`, `docs/19`, `docs/20`, `docs/23`, `docs/04`, `docs/02`,
`docs/21`, `docs/15`, the two files numbered 22, and `architecture.md`'s table (#60), each as listed in QA-07 and
QA-12. A `SECURITY.md` and a link to the privacy page from the README (QA-08).

## What this review could not tell you

- How fast anything actually is. Step 0's timer comes first for that reason.
- Whether the live Worker has the cron trigger, Turnstile and HSTS set, and which version is deployed.
- Whether the signing and R2 secrets are set, and whether the repository is public.
- How the app behaves with an IME, right-to-left text, a screen reader, a Turkish keyboard, on Windows, on Linux,
  or on a phone.
- Known vulnerabilities and available upgrades of the modules.
- The parts read only in passing: the drawing code of the auto-covers and the welcome shaders, the bodies of the
  caret presets, the table editor's UI, the admin panel and landing page sources, `drag.go`, `dockdrop.go`.
