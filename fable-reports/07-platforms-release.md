# 07 Platforms and release

Where the library lives in a packaged app, how the updater behaves, the release workflow, the web build and
Android. The updater's trust chain is in [03 Security](03-security.md) (SEC-03, SEC-04, SEC-05, SEC-10, SEC-11).

Back to the [index](README.md). No device, emulator, packaged app or live service was used: everything here is from
the sources, the scripts and the docs.

## How each target is made

- **Desktop**: five targets built without cgo from one Linux runner. `scripts/package.sh` makes `celestine.app` in a
  zip for macOS and a tarball for Linux. `scripts/release` signs a manifest (`version + "\n" + sha256`, ed25519).
- **Release**: a tag `v*` runs `.github/workflows/release.yml`: tests, builds, packages, signs when the key secret
  is set, uploads to R2 when those secrets are set, publishes a GitHub release. `cloudflare/update` serves the
  bucket read-only.
- **Update**: the app fetches the manifest, downloads the asset to the temp folder while hashing, checks the
  signature, swaps the file or the bundle, writes a mark, and offers a restart. At the next start an unclean
  previous session with the mark present rolls back.
- **Web**: `scripts/web.sh` builds the wasm, copies `web/*`, and patches `index.html` with an inline Python script.
  The library is one in-memory map written whole to one IndexedDB key 300 ms after a change. No versions, no
  backups, no keychain, no MCP there.
- **Android**: the web build inside one `WebView` (`MainActivity.kt`, 108 lines). Debug build only.

## Findings

### PLAT-01 The default library lives inside the program's folder
- **High.** Read; re-checked. Not exercised with a packaged app.
- Where: `internal/library/store.go:162-178`. `LibraryFile()` is `kitaplik.db` in the executable's folder whenever
  that folder is writable, and the data folder only when it is not.
- `scripts/package.sh:29` puts the binary at `celestine.app/Contents/MacOS/celestine`. A `.app` the user owns is
  writable, so the library is `celestine.app/Contents/MacOS/kitaplik.db`. No Go file treats a bundle specially.
- What follows:
  - An in-app update swaps the bundle (`update.go:365-375`). The new bundle has no library, the app opens a fresh
    seeded one, and the real one sits in `celestine.app.old`, which the updater removes a day later. The pre-update
    backup in the config folder is the only copy left.
  - Replacing the app by hand, the normal way to install a new version, deletes the library with it.
  - An unsigned, quarantined app runs translocated from a read-only path, so the first runs use the data folder;
    once quarantine is cleared the app switches to an empty file inside the bundle.
  - Writing into `Contents/MacOS` breaks any future code signature.
  - On Linux the tarball's README says `install -Dm755 celestine ~/.local/bin/celestine` (`scripts/package.sh:82`), so
    the library and its journal land in `~/.local/bin`.
- Fix: "beside the program" only for a portable layout the user chose, which is a `kitaplik.db` already there;
  otherwise always the data folder, and never inside an app bundle. Add a one-time move of a library found inside a
  bundle, and a test of `libraryFileIn` for a path under `*.app/Contents/MacOS`.
- Do this before the first release that people install.

### PLAT-02 The docs say the update pipeline is not connected
- **Medium.** Suspected: from the docs only, not checked against GitHub or Cloudflare.
- `docs/19`: the signing and R2 secrets are "not set: releases are unsigned and nothing goes to R2". `docs/16`:
  "automatic update does not in fact work" and "the repository stays private". The README points to the Releases
  page for downloads.
- If still true: released builds have an empty update address and key and never update; and with a private
  repository the README's download link is a 404 for everyone else.
- Decide one distribution path and make the README, `docs/16` and `docs/19` agree with it. PLAT-01, PLAT-03,
  PLAT-04 and PLAT-05 should be fixed before the pipeline is switched on, since no installed copy has yet exercised
  them.

### PLAT-03 "Restart" after an update quits the app without starting it again
- **High.** Read; re-checked. Depends on timing; not run.
- Where: `updateui.go:25-34` starts the child, then quits 200 ms later. The child's first step is
  `RequestSingleInstanceLock()` (`main.go:403`), which hands its arguments to the running copy and returns false
  while the parent is alive. The parent releases the lock only when it quits, after flushing and giving sync up to
  six seconds.
- So the child exits, the parent comes to the front and then quits. The user is left with no window. The rollback
  restart (`main.go:507-511`) has the opposite order and works only by racing.
- Fix: a `--restarted` flag that makes the child wait a few seconds for the lock; or release the lock before
  starting the child.

### PLAT-04 The downloaded file is renamed from the temp folder, which fails across filesystems
- **High on Linux, medium on Windows.** Read.
- Where: `update.go:449` downloads into `os.TempDir()`; `update.go:311` is `os.Rename(newFile, exe)`, which does
  not copy across devices.
- With `/tmp` on tmpfs (Fedora, Arch, openSUSE, Debian 13), or `%TEMP%` on another drive than the program, every
  update ends in "invalid cross-device link". The tests use one temp tree, so they pass.
- Fix: download to a temp file beside the executable. This also closes SEC-03.

### PLAT-05 The rollback fires when it should not and misses when it should
- **Medium.** Read.
- The mark is written at install time by the old process; "unclean" is simply "the `running` file was still
  there"; the rollback happens on both (`main.go:506`).
- (1) Install, choose "Later", then the old process is killed or loses power: the next start is the new version's
  first, sees an unclean end, and rolls itself back saying "version X crashed at start", though it never ran.
- (2) A new version that dies before `main.go:497` (one of the `log.Fatal` calls, or a file that cannot execute)
  never writes `running`, so the next start looks clean and nothing is ever rolled back. With SEC-04 a wrong-platform
  binary lands here.
- (3) The old file's removal is scheduled with a 24-hour timer (`update.go:423`): a process that lives less than a
  day never removes `celestine.app.old`, a second bundle left in `/Applications`.
- Fix: the new version writes its own "first start of X began" stamp before anything can fail; roll back only when
  that stamp exists without a "proved" stamp; delete the old file at the first start after proving.

### PLAT-06 Release workflow details
- **Low each.** Read.
- The in-app "what's new" is the last commit's message (`release.yml:66`), while the GitHub release gets the
  commits since the previous tag (`:53-60`). Use the same notes for both.
- With the key set and `DOWNLOAD_HOST` unset, the manifest is signed with `https:///v…` addresses and uploaded
  (`:67`). Fail the step when it is empty.
- R2 is updated, the manifest last, before the GitHub release step (`:77-84`). If that step fails, installed apps
  are already offered the version. Publish the release first, or as a draft published last.
- Without the key the release carries no checksums at all. Always publish a `SHA256SUMS`.
- The workflow repeats `scripts/build.sh`'s flags instead of calling it, so the 25 MB budget (`build.sh:36`) is
  never enforced in CI.
- macOS zips are neither signed with a Developer ID nor notarised, and the README does not say how to open an
  unidentified app. Until signing exists, say so.

### PLAT-07 The wasm build is in the written gate and not in the workflow
- **Low.** Read. `CLAUDE.md` and `scripts/sync-mygo.sh:50` include `GOOS=js GOARCH=wasm go build .`;
  `release.yml:28-43` builds only the five desktop targets. See QA-01.

### PLAT-08 Web store: a slow or failed read opens an empty library, which is then written over the real one
- **High for web and Android.** Read; the timeout re-checked. Not run.
- Where: `store_js.go:88` (a rejected read reports "not found"), `:95-96` (a 4-second timeout reports "not
  found"), `:119` (a JSON parse error is ignored and the empty tree kept). Then `:145-171`: any change writes the
  in-memory tree over the same key.
- Three different failures are indistinguishable from a first run: the read takes over four seconds, which a
  library of tens of megabytes as one base64 string on a phone can; the read rejects; the JSON is cut short. The
  app seeds an empty library and 300 ms later the real one is replaced. This target has no versions and no backups
  to recover from.
- This is separate from the accepted one-blob design: it is the error handling around the blob.
- Fix: three results from the read (a value, absent, failed); on failure or a parse error return an error from
  `OpenStore` and refuse to write; no timeout on the read; before overwriting keep the previous value under a
  second key.

### PLAT-09 Web store: write failures are invisible
- **Medium.** Read.
- Where: `store_js.go:158` clears `dirty` before the write; `:164-166` drops the promise of `put`. A quota error is
  an unhandled rejection: nothing retries, nothing is shown, and the user goes on typing into memory only.
- On `pagehide` the transaction is not created synchronously (it waits for an `open`), so a tab that is really
  closing can drop the last edits.
- Fix: one open connection; `then` and `catch` on the write, setting `dirty` back and surfacing a save error;
  create the transaction synchronously in the `pagehide` handler.

### PLAT-10 Web store: over the localStorage quota the program panics
- **Low**, only where IndexedDB is missing. Read.
- `store_js.go:169`: the comment expects `setItem` to fail silently; `js.Value.Call` panics on a JS exception, in a
  timer goroutine holding the store's lock, and the wasm program exits. Wrap it in a `recover`.

### PLAT-11 `scripts/web.sh`: stale wasm files, a silent patch, and Python
- **Medium.** Read; the stale files are confirmed by `docs/21`.
- `web.sh:16` takes `ls dist/web/*.wasm | head -1`; nothing removes `dist/web` first. After a second build there
  are two wasm files and the service worker may be registered with the old one; `android.sh:11` copies all of them
  into the APK (`docs/21`: a debug build can reach 69 MB).
- `web.sh:23-30`: four `replace(…, 1)` calls whose misses are not detected. If MyGo changes its `index.html`,
  `touch.js`, the manifest or the worker registration silently stop being added and the build still succeeds.
- The build needs `python3`.
- Fix: remove `dist/web` first; take the wasm name from `index.html`; fail when a replacement changed nothing; do
  the patch in a few lines of Go so the build needs only Go.

### PLAT-12 `web/sw.js`: a failed precache is swallowed
- **Low.** Read.
- `sw.js:13` catches the failure of `addAll` and activates anyway; `sw.js:16` then deletes every other cache. If
  the 28 MB wasm fails to download, the new worker deletes the previous build's cache and nothing opens offline.
- The manifest's `start_url` is `./index.html`, while the worker precaches `./`.
- Comments in `sw.js:4`, `web.sh:6` and `MainActivity.kt:25` still say the library is in `localStorage`.
- Fix: let install fail so the old worker stays; add `index.html` to the precache.

### PLAT-13 Android: no INTERNET permission, so sync cannot reach a server
- **High for Android.** The manifest re-checked; the effect at run time not exercised.
- `android/app/src/main/AndroidManifest.xml` has no `<uses-permission>` at all. A WebView without
  `android.permission.INTERNET` fails every network request; the app's own pages load only because
  `shouldInterceptRequest` answers them. Sign-in and sync, the only way data leaves the phone, cannot work.
  `docs/21` lists typing, saving and reopening as tried, not sync.
- Fix: add the permission and try a sync on a device.

### PLAT-14 Android: text shared to the app is dropped
- **Medium.** Read.
- `MainActivity.kt:95-96` builds `…/index.html#share=<text>`; the manifest declares the `SEND` filter; no Go code
  reads `location.hash`. Celestine appears in every app's share sheet, opens, and does nothing with the text.
- With the default launch mode a share starts a second activity, a second WebView holding its own copy of the
  library over the same key; whichever writes last replaces the other's edits (suspected, follows from the
  whole-blob writes).
- Fix: read the hash at start in the js build and make a quick note from it; `singleTask` and `onNewIntent`; or
  remove the filter until it works.

### PLAT-15 Android: the back button always leaves the app
- **Medium.** Read.
- `MainActivity.kt:100-105` sends a synthetic key to the page and then calls `super.onBackPressed()`
  unconditionally. `docs/21` says back goes to the app's own history. With `targetSdk = 36` the deprecated
  `onBackPressed` may not be called at all (suspected, from the platform's documented behaviour).
- Fix: an `OnBackInvokedCallback`; ask the page whether it can go back and finish only when it cannot.

### PLAT-16 Android and web: the notes and the sync secrets are in app storage in plain text, and Android backs it up
- **Medium.** Read.
- `AndroidManifest.xml:7` is `allowBackup="true"` with no extraction rules. There is no keychain in the browser
  (`keychain_js.go:5-6`), so the settings keep the sync key and the device token, in the same blob as the library.
- The library and the key go to the cloud backup and to device-to-device transfer, outside the end-to-end
  encryption the README promises for what leaves the computer.
- Fix: `allowBackup="false"` (sync is the backup), or extraction rules that exclude the WebView's folder; later the
  Android Keystore through a JS interface.

### PLAT-17 Android is a debug build only, outside the gate and the release
- **Low.** Read.
- `scripts/android.sh:17` runs `assembleDebug`. `versionCode = 1`; no signing config, no release build type; the
  Gradle wrapper has no checksum pinned. A link in a page navigates the app's own WebView to the site
  (`MainActivity.kt:59`, no `shouldOverrideUrlLoading`).
- When Android ships: a release build type signed from the environment, the version from the tag, external links
  opened with `ACTION_VIEW`, the wrapper's checksum pinned.

## Order for a first public release

1. PLAT-01: the library out of the bundle.
2. PLAT-03, PLAT-04, PLAT-05: the updater's own behaviour, then SEC-03, SEC-04, SEC-10, SEC-11 for its trust.
3. QA-01: CI on push, with the wasm build.
4. QA-09, QA-10: a licence and the third-party notices.
5. PLAT-06: checksums, the same notes in both places, a line in the README about unsigned macOS builds.
6. Web and Android only after PLAT-08 and PLAT-09; Android also needs PLAT-13 and PLAT-16.
