# ryudit

A notes app in Go: one binary, native UI on MyGo (`github.com/egoist/mygo`, replaced by the fork
`github.com/imhalid/mygo` in `go.mod`) and the Ryugo kit (`github.com/imhalid/ryu-go`). docs/architecture.md has the
file-by-file table; `docs/` has the design notes (the older ones in Turkish; new ones in English).

## Layout

- The root is the app: `package main`, the window and everything drawn in it. One `app` struct (main.go) holds
  its state; a feature keeps its own in a struct of its own (`paneState`, `syncUI`, `welcomeState`).
- `internal/library/` is the library: collections, pages, the page file, properties, the one-file store
  (`store_sql.go`, `store_js.go` for the browser), versions, the trash, sync, the Markdown export. It never
  imports `mygo`, `mygo/ui` or the app. New library code goes there.
- `core_alias.go` gives the library's names to the app unqualified (`type Page = library.Page`). A library name
  the app needs is exported and gets its line there (or the app writes `library.X`). A variable is not aliased
  but pointed at (`PagesGen`); `library.AllowHTTP` and `library.Format` are set as they are.
- The body of a page is the editor's format (`body.go`); the library reads it through `library.Format`
  (`BodyFormat`), which `body.go` sets at init. A library test alone has a plain Markdown format instead.
- The app cannot add methods to the library's types: a function in the app (`dayTitle(c, d)`) or a method in
  the library.
- `web/`, `android/` and `cloudflare/` (the sync and update Workers) are the other targets; `scripts/` builds them.

## Commands

```sh
go run . [library]                  # ./kitaplik by default, made and seeded when missing
RYUDIT_WELCOME=1 go run .           # the first-run welcome, again
go test ./internal/library/         # the library alone: a second or two; run it after any library change
go test -run 'Sync|Store' .         # the app's tests of an area
go test ./...                       # everything: about four minutes, run it in the background
```

The gate before a commit (the release workflow runs it too): `gofmt -l .` empty, `go vet ./...`,
`go test ./...`, and builds without cgo: `CGO_ENABLED=0 GOOS=darwin|linux|windows go build .` and
`GOOS=js GOARCH=wasm go build .`. The Go of `go.mod` formats; another gofmt aligns differently.

## The rules of the code

- The app's state is behind `a.mu`: a frame holds it (`viewFrame`). A goroutine does its slow part outside it
  and writes what it found through `a.do(func() error { ... })`, and starts with `defer a.guard()`, which logs
  a panic instead of losing it.
- Time comes from `a.clock()`, not `time.Now()`, so tests can set it; a test that counts days sets the clock
  before anything is counted.
- English is the project's language: new words shown in the app, comments, docs, commit messages, release
  notes, tags, issues and PRs are written in English. Only the conversation with the maintainer is in Turkish.
- The app is English unless Turkish is chosen. Words shown go through `T("…")` (i18n.go): the older strings are
  written in Turkish with their English in `enText`; a new one is written in English, and T gives it as it is.
- Comments are sentences that say what a thing is and why, in the voice of the ones around them; a
  declaration's comment starts with its name.
- UI tests drive the real view with `ui.NewTester` (`newTestApp` in helpers_test.go). Shaders are off in tests
  where the CPU renderer is too slow (`welcomeShaders`).
- A change to the library comes with a test in `internal/library/` when it can be made without the app.
- Commits go to `main`: a one-line title that says what changed, then prose.
- Releases: a tag `vX.Y.Z` pushed to GitHub runs `.github/workflows/release.yml` (tests, the five builds, the
  GitHub release, notes from the commits since the tag before). A `v0.x` tag, or one with a "-", is a prerelease.
  Versions go up one at a time from v0.0.1; the gate passes before a tag.
- A local checkout of Ryugo beside this one is used through a `go.work` (ignored by git).
