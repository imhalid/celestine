# 08 Tests, docs and hygiene

The health of the test suite, what the docs say against what the code does, the licence, the dependencies, and
where the name is hard-coded.

Back to the [index](README.md). The full root suite was not run, so there are no per-test timings and no coverage
profile; the two failing tests were run alone.

## Findings

### QA-01 No CI on push or pull request
- **High.** Re-checked.
- `.github/workflows/` holds only `release.yml`, which runs on tags `v*`. `CLAUDE.md` says commits go to `main` and
  the gate passes before a tag. At `HEAD` on this machine two tests fail (QA-02).
- So `main` is red without anyone being told, and a failure is found only after a tag exists; since versions go up
  one at a time, the version is burned or the tag must be moved.
- `-race` is never run anywhere, though `docs/23` asks for it for `Store`, `SyncOnce` and `prewarm`. The wasm build
  is in the written gate and not in the workflow (PLAT-07). `govulncheck` is not installed and nothing runs it.
- Fix: a `ci.yml` on push and pull request, on `ubuntu-latest` and `macos-latest` (macOS is where QA-02 shows):
  `gofmt -l .`, `go vet ./...`, `go test -race ./internal/library/`, `go test ./...`, the three no-cgo builds and
  the wasm build, `govulncheck ./...`. Have `release.yml` reuse it.

### QA-02 Two tests fail at `HEAD` on macOS
- **Medium.** Both run alone and seen to fail.
- `TestSettingsInEnglish`: the list of "words only Turkish has" (`settings_english_test.go:12`) contains `var`, and
  macOS temp paths start with `/var/folders`. The settings show the library's path and the MCP command line, which
  contain it. On Linux (`/tmp`) it passes, which is why a release could still be cut. Any user path containing
  `bir`, `ve` or `her` would do the same. Fix: strip path-like tokens before matching, or skip texts that contain
  the library's folder.
- `TestWhiteSpaceMarks`: `spaces: nothing drawn (18 bytes differ)` against a threshold of 20
  (`editing_test.go:173-174`). Eighteen changed bytes means the dots are drawn; the count depends on the machine's
  font and on anti-aliasing. The cause was not traced into the painter. Fix: count the marks drawn through the
  painter instead of bytes, or require more than zero with the existing "none left behind" check as the control.
- A correction to an older note: `TestStatsOut` is not date-dependent at `HEAD`. Its clock is fixed at 2026-11-07
  and it passes.

### QA-03 The root suite is serial by construction
- **Medium.** Counted; timings reasoned, not measured.
- 461 tests in 99 files. `t.Parallel` and `testing.Short` appear zero times. 294 testers are made and about 1 125
  frames are asked for, each a software rasterisation; the default tester is 900×1400 px (`caret_test.go:62`).
  `settings_english_test.go` uses 1180×2400 and took 2.46 s alone. `caret_test.go:81-83` renders a frame per rune.
  `perf_test.go` builds a 1 500-page library. `wobble_test.go` sleeps about a second.
- Why it cannot simply be made parallel: tests share package variables: `lang`, `phone`, `rowH`, `keychain`,
  `version`, `toastShaders`, `welcomeShaders`, `library.AllowHTTP`, and the keyring's process-wide mock.
- Fix, in order of payoff:
  1. Measure first: `go test -json .` and rank tests by elapsed time.
  2. `-short` skips for the heaviest (the perf budget, the per-rune caret sweep, the welcome grounds, the wobble
     sleeps); `-short` on every push, the full run on tags.
  3. A smaller default tester for tests that do not look at pixels.
  4. Move pure-logic tests that need no window into `internal/library` or a second internal package.
  5. Move `lang`, `phone` and `rowH` onto `app`, so UI tests can become parallel later. This is also what UX-04
     needs.

### QA-04 Test files that are leftovers by name
- **Low.** Read.
- `updbg_test.go` (24 lines): one real regression test, `TestUpFromHeading`, under a debugging name.
- `extra_test.go` (356 lines): ten unrelated real tests; `TestProfiles` sits here beside `profile_test.go`.
- `shot_test.go` (554 lines, 16 tests): screenshot generators, each skipped unless an environment variable names an
  output; two more hide in `views_test.go:329`, `:468`. They are tools the gate never runs, so they rot unnoticed.
- Fix: distribute the first two by area; put the screenshot tools behind `//go:build shots` and document the
  variables in one place.

### QA-05 What the tests do not reach
- **Medium.** A rough pass: for each root file, how many of its functions are called by name from a test. This
  finds "never called directly", not "never executed"; a real number needs `-coverprofile` on the full suite.
- No test file and no direct call: `mizan.go`, `toast.go`, `funbar.go`, `caretextras.go`, `mcp_extra.go`, `vim.go`,
  `updateui.go`, `mcp_help.go`, `menupanel.go`, `help.go`, `tagfield.go`, `dockdrop.go`, `movecmd.go`,
  `blockmark.go`.
- A test file but few functions called: `tables.go` (4 of 38), `collsettings.go` (4 of 27), `settings.go`
  (2 of 21), `welcome.go` (2 of 24), `drag.go` (1 of 14), `calendarview.go` (0 of 10).
- Nothing at all for `store_js.go` (PLAT-08), and no Worker test that runs by default (SYNC-20).
- The tests that would have caught the top findings are listed at the end of
  [01 Data integrity](01-data-integrity.md) and in SYNC-20. The single most valuable one is the body round-trip
  property test (DATA-10).

### QA-06 Flake candidates
- **Low.** Read.
- Fixed 2 ms sleeps "so the mtime differs" (`syncapp_test.go:399`, `:463`, `:628`, `linkindex_test.go:34`, `:45`,
  `:61`) are candidates on filesystems with coarse mtimes.
- `lang` is restored to a hard-coded `"tr"` in two tests (`settings_english_test.go:28-29`,
  `appcmds_test.go:83-84`), though English is the default.
- `TestPerfBudget`: PERF-18.

### QA-07 Docs that contradict the code or each other
- **Medium as a set.** Each line checked.

| Doc | What it says | What is true |
| --- | --- | --- |
| `CLAUDE.md` | MyGo is "replaced by the fork `github.com/imhalid/mygo` in `go.mod`" | `go.mod` has no `replace`; it requires a pseudo-version of MyGo itself |
| `CLAUDE.md` | The default library is `./kitaplik` | `kitaplik.db` beside the program (`store.go:174`); `architecture.md` has it right |
| `docs/12` | A fork of MyGo is used through `replace`; "no colouring today" | Neither; `codecolors.go` colours code |
| `docs/19` | macOS and Linux builds are bare binaries; `app-zip` is not used | `scripts/package.sh` makes the `.app` zip |
| `docs/20`, `docs/23` C5 | The token and key are in the settings file | In the system keychain since `keychain.go` |
| `docs/23` | The deploy lacks the cron trigger | `deploy.sh:39-43` builds it and refuses without it |
| `docs/04` | Code colours, attachments, live query, zoom are missing | All four exist |
| `docs/02` | Only five property kinds; no natural dates | `docs/06` and the code say otherwise |
| `docs/21` | Back goes to the app's history; a share makes a note | PLAT-15, PLAT-14 |
| `docs/15` | A key hook would simplify the `/` menu; block selection is next | The hook exists; so does `blocksel.go` |
| `docs/17`, `docs/18` | wasm is 29.6 MB; wasm is 28.5 MB | One of them |
| `architecture.md` | The gate, without the wasm build; "docs are in Turkish" | `CLAUDE.md` includes wasm; new docs are English |
| `sw.js`, `web.sh`, `MainActivity.kt` | The library is in `localStorage` | IndexedDB |
| `docs/00` | Cites `~/thymer-research/…` | A path outside the repository |

- Numbering: two files are numbered 22; 01, 08 and 11 are gone. Rename one, and say in the index that numbers are
  not reused.

### QA-08 Project documents that do not exist
- **Low.** Read.
- No `CHANGELOG` (release notes are generated commit titles). No `SECURITY.md`, for an app with accounts,
  end-to-end encryption, a local server with a token and a self-updater. No `CONTRIBUTING` (`CLAUDE.md` is the only
  statement of the rules). No user documentation beyond the README's feature list; the in-app help and the tour are
  the manual.
- Privacy: the server has a page at `/privacy`, but the README does not link it, and its "the server only ever sees
  scrambled data" does not mention that the account's e-mail address is stored.

### QA-09 There is no LICENSE
- **High.** Re-checked.
- No licence file is tracked; the README says nothing about terms; `cloudflare/sync/package.json` has npm's default
  `"license": "ISC"`, which nobody chose.
- With no licence nobody may legally use, build or contribute; if the app is meant to be proprietary, that is not
  stated either, and the ISC line contradicts it.
- Fix: add `LICENSE`, or a short "all rights reserved" notice with terms for the binaries; set
  `"license": "UNLICENSED"` and `"private": true` in `package.json` if the Worker is not open.

### QA-10 `NOTICE.md` omits every library compiled into the binaries
- **Medium.** The list read; not every licence text.
- `NOTICE.md` has two sections (Cursor-Smith, synth-ui), both accurate. The binaries link MyGo, Ryugo,
  `modernc.org/sqlite` and its runtime (and SQLite itself), the MCP SDK, `yaml/v3`, `go-text/typesetting`,
  `purego`, `go-keyring`, `dbus`, `wincred`, `uuid`, `golang.org/x/*` and others. MIT, BSD and Apache-2.0 all ask
  that their notice accompany binary distributions; the release zips and tarballs carry none.
- Fix: generate a third-party notices file at release time (`go-licenses report`), put it in the `.app`'s
  resources, the tarball and an About screen.

### QA-11 Dependencies: two same-day pseudo-versions carry the whole UI
- **Medium.** Read.
- `go.mod:6-7` requires `mygo v0.8.4-0.20261109095711-…` and `ryu-go v0.1.1-0.20261109084958-…`: both commits are
  from the day of this review, on untagged `main` branches. The module cache holds eight Ryugo pseudo-versions from
  five days: the pin moves daily.
- The risk: a `main` commit between releases has had no release testing, and a commit that is later force-pushed
  away can be fetched only while the proxy has it.
- `go.mod:27` marks `go-keyring` as indirect though `keychain_os.go` imports it; `go mod tidy` will move it.
- `go mod verify` passes. Available upgrades and known vulnerabilities were not checked (no network).
- Fix: tag Ryugo and require the tag (`docs/14` already proposes it); require MyGo's latest release and carry what
  is missing as the documented patch branch, or accept `main` and say so in `CLAUDE.md`.

### QA-12 `architecture.md`'s table is far behind (issue #60)
- 70 of 139 root files are not in it (`caretfx*.go` counted as covered). Missing:
  `account.go`, `appcmds.go`, `appthemes.go`, `attachfile.go`, `autocover.go`, `autocover_scenes.go`,
  `backlinks.go`, `banner.go`, `blockmark.go`, `blocksel.go`, `bulk.go`, `calendarview.go`, `caretextras.go`,
  `caretplay.go`, `caretstyle.go`, `codecolors.go`, `cover.go`, `ctxmenu.go`, `dockdrop.go`, `dropfiles.go`,
  `dynamic.go`, `edges.go`, `embed.go`, `emptystate.go`, `fieldmenu.go`, `focus.go`, `funbar.go`, `hidetitle.go`,
  `historyui.go`, `i18n_find.go`, `images.go`, `importmd.go`, `keychain_js.go`, `keychain_os.go`, `keys.go`,
  `links.go`, `mcp_help.go`, `mcp_js.go`, `mcpsetup.go`, `menupanel.go`, `mobile.go`, `movecmd.go`, `pagemenu.go`,
  `palgroups.go`, `panes.go`, `panewin.go`, `prewarm.go`, `profile.go`, `propkinds.go`, `quick.go`, `radius.go`,
  `selbar.go`, `settingsx.go`, `shown.go`, `sidepanel.go`, `sortlist.go`, `statusbar.go`, `tab.go`, `tagfield.go`,
  `tags.go`, `tasks.go`, `templates.go`, `themebutton.go`, `titlemenu.go`, `toast.go`, `version.go`, `welcome.go`,
  `welcomelooks.go`, `windowcontrols.go`, `writing.go`.
- Also without a row: `web/`, `android/`, `cloudflare/`, `scripts/`.
- A table kept by hand will fall behind again. A test that fails when a root `.go` file is in neither the table nor
  an allow list would hold it.

### QA-13 `prototypes/slime-md` is outside the gate
- **Low.** It has its own `go.mod` (MyGo v0.7.4), so build, vet and test at the root skip it. It carries 1.6 MB of
  PNG sheets. `docs/00` still points to it as where the carets are ready. Archive it under a tag and delete it, or
  say in `architecture.md` that it is frozen.

### QA-14 `rsrc_windows_amd64.syso`: a committed binary with nothing checking it
- **Low.** Produced by `go run ./scripts/winicon` from `assets/icon.ico`. Nothing regenerates and compares it, so
  an icon change that forgets the step ships the old icon. Only amd64 exists. It carries icons only: no version
  information (Explorer shows no version or product name) and no application manifest.
- Fix: a test in `scripts/winicon` that builds the bytes in memory and compares with the committed file.

### QA-15 Smaller hygiene points
- `.gitignore` does not cover `release.key` (SEC-11), nor `update.json`, `release-notes.md`, `notes.md`.
- `web/icon-256.png` and `icon-512.png` duplicate the ones in `assets/`.
- `scripts/sync-mygo.sh:27`, `:37` does `git reset --hard origin/ryudit` and a force push in the fork checkout:
  local unpushed patch commits there are discarded without a check. Stop when the branch is ahead of its remote.
- `store_js.go:219` returns a Turkish error from the library; `:40` has a `Now` field that is never used.
- `time.Now()` appears in about 35 places in app code against the `a.clock()` rule (the table in
  [03 Security](03-security.md)).
- Tracked files: 395; the pack is 18 MiB; nothing generated is tracked besides the `.syso`, the Gradle wrapper and
  the lock file. No secrets in tracked files.

### QA-16 The rebrand surface (issues #1 to #10)
- Counted over tracked files.

| What | Occurrences | Files |
| --- | --- | --- |
| `ryudit` in any case | 301 | 82 |
| Module path `github.com/imhalid/ryudit` | 9 | 9 |
| `RYUDIT_*` environment variables (7 names) | 35 | 7 |
| Config and data folder joins in Go | 14 | 8 |
| Bundle and application id `dev.halid.ryudit` | 5 | 4 |
| `*.halid.dev` hosts | 14 | 8 |
| Storage and cache keys (`"ryudit:`, `"ryudit-`) | 26 | 15 |
| Dotfiles `.ryudit-*` | 12 | 10 |
| R2 buckets and Workers | 11 | 4 |
| `ryudit://` resource URIs | 2 | 2 |
| Keychain service | 1 definition, 6 uses | 2 |

- The ones that hold **data** and need a migration, not a rename: the config and data folder, the keychain service,
  the IndexedDB database and key, `.ryudit-format`, the bundle id (macOS preferences, Android data), the update
  file names installed copies ask for, the bucket names, and the encryption labels (`"ryudit/v1/"` in the KDF salt
  and `"ryudit/recovery-auth"`, which must **not** change with the name or every account stops opening).
- Before the rename: put the name in one Go constant and one shell variable.

## Sound

Live-network tests are opt-in by environment variables and say how to run them; every other HTTP test is on
`httptest`. The clock is injectable and the stats tests use it properly. The keychain tests use the library's mock,
never the real keychain. `internal/library` is isolated and its 38 tests run in under a second of test time.
`build.sh` records sizes against the previous build and has a budget. `gofmt -l .` is clean, and the wasm target
compiles at the working tree.
