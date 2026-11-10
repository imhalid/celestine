# Ryudit review, November 2026

A file-by-file review of the repository at `main` (`6b22bf3`, plus the uncommitted page-history work of issue #71),
made on 2026-11-09. It covers the library, sync and the Workers, MCP, the updater, the editor, the screens, the other
targets, the build, the tests and the docs: about 80 000 lines.

The reports are written to be read in any order. Each finding has an ID; other reports refer to it by that ID.

| Report | What is in it | Findings |
| --- | --- | --- |
| [01 Data integrity](01-data-integrity.md) | Every way a page, a picture, a setting or a library can be lost or silently changed, outside sync | DATA-01 … DATA-22 |
| [02 Sync](02-sync.md) | The sync engine, its encryption, the account server | SYNC-01 … SYNC-20 |
| [03 Security](03-security.md) | MCP, the updater, files from outside, secrets, the release pipeline | SEC-01 … SEC-21 |
| [04 Performance](04-performance.md) | Per-frame cost, idle cost, memory, start-up, storage; issues #14, #15, #16, #70 | PERF-01 … PERF-20 |
| [05 Editor and text](05-editor-and-text.md) | Keyboard layers, Vim, undo, paste, find, and what stands in the way of issue #23 | EDIT-01 … EDIT-10 |
| [06 UX, i18n, accessibility](06-ux-i18n-accessibility.md) | Errors the user never sees, actions without undo, the language system, contrast, product gaps | UX-01 … UX-14 |
| [07 Platforms and release](07-platforms-release.md) | Where the library is kept in a packaged app, the updater's behaviour, web, Android, CI | PLAT-01 … PLAT-17 |
| [08 Tests, docs, hygiene](08-tests-docs-hygiene.md) | The test suite's health, stale docs, licence, dependencies, the rebrand surface | QA-01 … QA-16 |
| [09 Action plan](09-action-plan.md) | Everything above in the order it should be done, grouped into issues | |

## How it was done, and how far to trust it

Seven reviewers each read one area in full and wrote findings with `file:line` evidence. Then the findings that carry
the most weight were read again against the code before they went into these reports. Each finding says how it was
confirmed:

- **reproduced**: a throwaway test was run against the real package (through `go test -overlay`, with the test file
  outside the repository; nothing in the repository was touched) and showed the behaviour.
- **read**: the code path was followed end to end, callers included, but nothing was run.
- **suspected**: one link in the chain was not confirmed; the finding says what would confirm it.
- **re-checked**: in addition to the above, the cited lines were read a second time while these reports were written.

What was not done: the app was never started, so nothing here is a profile, a screen recording or a device run. No
live server was contacted. `go test ./...` was not run in full. Nothing was run with an IME, a right-to-left script,
a screen reader, a non-US keyboard layout, on Windows, on Linux or on a phone. The frame-cost findings are therefore
readings of what the code does every frame, not measurements; the two places where a number is given say where it
came from.

Line numbers are those of the working tree on 2026-11-09 and will drift.

## The fifteen that matter most

In the order they should be looked at. The first eight can lose a user's writing.

| # | Finding | Report | Confirmed |
| --- | --- | --- | --- |
| 1 | Turning a collection's sync off deletes it on the server and on every other device | [SYNC-01](02-sync.md) | reproduced, re-checked |
| 2 | Changing the sync target or account empties a small library into the trash and blocks a larger one for good | [SYNC-02](02-sync.md) | reproduced |
| 3 | Ticking a task in the Tasks view, a quick note, a tag rename and an agent's edit all rewrite the whole page through Markdown, which drops task kinds, alignment, empty lines, table filters, file chips | [DATA-01](01-data-integrity.md) | reproduced (conversion), re-checked (paths) |
| 4 | A page file that cannot be read opens as an empty page; the first keystroke overwrites it, and opening it has already deleted its pictures | [DATA-03, DATA-04](01-data-integrity.md) | reproduced, re-checked |
| 5 | Moving, dragging or duplicating a block loses dividers, task kinds and code languages | [DATA-05](01-data-integrity.md) | reproduced |
| 6 | In a packaged app the default library is inside `ryudit.app`; an update or a reinstall leaves it behind or deletes it | [PLAT-01](07-platforms-release.md) | read, re-checked |
| 7 | Underline (⌘U) or a literal `<tag>` turns a paragraph, or a whole list, into a raw HTML block after one Markdown round trip | [DATA-02](01-data-integrity.md) | reproduced |
| 8 | Web and Android: a slow IndexedDB read is taken for a first run, and the empty library is written over the real one | [PLAT-08](07-platforms-release.md) | read, re-checked |
| 9 | `ryudit mcp` cannot read the token where a keychain exists, and writes a new plaintext token into the live library each time it tries | [SEC-01](03-security.md) | read, re-checked (ordering) |
| 10 | A Markdown comment can plant a file chip that opens any local path when clicked | [SEC-02](03-security.md) | read, re-checked (the open) |
| 11 | "Restart" after an update quits the app without starting it; the download cannot be installed across filesystems | [PLAT-03, PLAT-04](07-platforms-release.md) | read, re-checked |
| 12 | The palette rebuilds every command and reads the backup folder from disk on every frame (issue #16) | [PERF-01](04-performance.md) | read by two reviewers, re-checked |
| 13 | The default caret and the scrollbar keep the whole window repainting at the display's rate while nothing happens | [PERF-03, PERF-04](04-performance.md) | read, re-checked |
| 14 | No CI on push or pull request; two tests fail at `HEAD` on macOS; the suite cannot run in parallel | [QA-01, QA-02](08-tests-docs-hygiene.md) | reproduced (the failures), re-checked |
| 15 | There is no LICENSE, and the shipped binaries carry none of the notices their libraries require | [QA-09, QA-10](08-tests-docs-hygiene.md) | re-checked |

## What is done well

Worth saying, because it shapes the advice: most of the fixes are small because the structure under them is sound.

- The library is a package of its own that knows nothing of the window, behind one `FS` interface, with tests that
  run in a second. `Store.Clean` makes path traversal structurally impossible inside the one-file store.
- Sync is never unencrypted, hides names as well as bytes, and every write is conditional twice (the hash, then the
  object's etag). The block merge refuses rather than drops text. Per-account isolation on the server is sound; every
  D1 statement is bound; tokens and codes are stored hashed and compared in constant time.
- The MCP server binds to loopback, checks `Host` before the token, uses 192 bits from `crypto/rand`, and checks
  per-collection access in every data tool. `run_command` is an allow list.
- The updater fails closed without a key, refuses downgrades, and backs the library up first.
- Unknown object kinds survive the body, an edit session and a Markdown round trip.
- 30 000 random Markdown inputs, deep nesting and invalid UTF-8 produced no panic.
- Every goroutine in the app starts with `a.guard()` and writes through `a.do`; time goes through `a.clock()` almost
  everywhere.
- The docs say honestly what was not tried, which is where several findings here were cross-checked.
- There are no secrets in the repository and no `TODO`/`FIXME` debt.

## Two themes that run through all of it

**1. Markdown is still a write path.** The decision is that a page is typed blocks and Markdown lives only at the
edges. But four in-app operations still read the Markdown view, edit lines and save the page from Markdown, and the
conversion is lossy in a dozen ways. DATA-01, DATA-02, DATA-08 and DATA-09 are the same problem seen from different
sides; a body-level edit API and one property test (`body → Markdown → body`) close most of it.

**2. The frame does too much, and something always asks for the next one.** The view function rebuilds lists, reads
directories and re-scores searches on every frame, and the default caret and the scrollbar each request a full
repaint forever. Either alone would be tolerable; together they are issues #15 and #16 and the app's idle cost. The
fix is the same shape everywhere: keep the result, key it on a generation counter, and ask for a frame only while
something is still moving.
