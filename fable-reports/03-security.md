# 03 Security

MCP, the updater's trust chain, files that come from outside, secrets on disk, and the release pipeline. The
server's authentication and the sync encryption are in [02 Sync](02-sync.md) (SYNC-04, SYNC-08, SYNC-10,
SYNC-13 to SYNC-18). How the updater behaves when nothing is attacking it is in
[07 Platforms and release](07-platforms-release.md).

Back to the [index](README.md). Almost everything here was confirmed by reading the whole code path, not by running
it; each top finding names the run that would settle it.

## The attack surface, briefly

| Surface | Who can reach it | Guard today |
| --- | --- | --- |
| MCP server, `127.0.0.1:7457` | Any local process; a browser page through DNS rebinding | `Host` allow list, then a 192-bit bearer token compared in constant time |
| MCP tools | Whoever has the token, and whatever text steers that agent | Per-collection access checked in every data tool; `run_command` is an allow list by group |
| `celestine mcp` (stdio bridge) | The agent that starts it | Reads the app's settings for port and token |
| Markdown in: import, a dropped file, paste, an agent's write | A file from anywhere | `importMarkdown`; picture paths confined to the page's folder |
| `page.json` and `collection.json` from sync or a folder | Another device with the key; anyone who can write the folder | `SafeName` on remote names |
| Update manifest and asset | The bucket, the Worker, the network | HTTPS, sha256, ed25519 over version and hash; fails closed with no key |
| The release workflow | Third-party actions, dependency code under test | Secrets as GitHub secrets |

## Findings

### SEC-01 The stdio bridge cannot read the token where a keychain exists, and writes a plaintext one into the library
- **High.** Read; the ordering re-checked. Not run.
- Where: `main.go:397-399` runs `runMCP` and exits; `keychain = systemKeychain()` is set only at `main.go:478`. In
  the bridge process the keychain is nil, so `loadSecrets` fetches nothing (`keychain.go:48`). The running app has
  already stripped the token from the stored settings (`keychain.go:85-87`). So `mcp.go:185-191` finds an empty
  token, makes a new one and saves it, and with no keychain that write puts everything into the library file's
  settings row. `mcpbridge.go:44` does this every 300 ms for up to 30 s.
- Effect on macOS, Windows, and Linux with a Secret Service: the bridge that "Add to Claude Desktop / Cursor /
  Claude Code" installs sends a token the app never had, gets 401, and exits with code 0. Meanwhile a second process
  has written the live library's settings (a stale copy can overwrite a concurrent save), a plaintext token sits in
  the file the keychain was meant to keep clean, and at the app's next start that token is moved into the keychain,
  replacing the real one and breaking every HTTP client set up with it.
- The test passes only because it has no keychain (`mcpbridge_test.go:59-76`).
- To confirm: on macOS with MCP on, run `celestine mcp`, send `initialize`, look for 401 and for a `token` in the
  settings row.
- Fix: set the keychain before `runMCP`; make the bridge's read of the settings read-only (never a new token, never
  a save, the store opened read-only); exit non-zero with the HTTP status when the relay fails.

### SEC-02 A file chip can point anywhere on disk and is opened by the system on click
- **High.** Read; the open re-checked. Not run.
- The open: `chips.go:404-408` is `filepath.Join(a.page.Dir, h.ch.target)` handed to `mygo.Shell.OpenPath`, with no
  check. The same join serves file properties (`propkinds.go:105`, `:136`, `:140`). Pictures are guarded
  (`images.go:50-62`) and links are allow-listed (`links.go:55-64`); this one path is not.
- How a target gets there:
  - from `page.json` directly, which arrives by sync, a restored backup, or a folder library someone else can write;
  - through Markdown: a line `<!--celestine:block <base64>-->` becomes a chip of unknown kind carrying raw bytes
    (`blockmark.go:50-59`); it is written back as those bytes (`body.go:70-75`) and read again as a normal object
    (`body.go:59-67`, `:175-177`). So an agent's write, an imported vault, a dropped `.md` or a paste can carry
    `{"k":"file","target":"../../Applications/X.app","alias":"Invoice.pdf"}`. The label shown is the alias.
- Effect: one click launches any existing local path with its default handler.
- A second bug in the same lines: in the default one-file library `page.Dir` is a virtual path such as
  `/notes/welcome`, so a legitimate attachment never opens. Nothing extracts it to disk.
- Fix: validate `target` with `filepath.IsLocal` when the body is decoded and again at the click; in `blockMarksIn`
  refuse a mark whose kind this version knows; for a one-file library extract the file to a private temp folder
  before opening it.

### SEC-03 The update is verified in one place and installed from another
- **Medium.** Read.
- Where: `update.go:248` accepts an existing `/tmp/celestine-update` owned by anyone; `:260-261` creates the `.part`
  file following symlinks; the hash is taken from the stream (`:270`); `installBinary` later renames the path, not
  the bytes that were checked (`:305-311`).
- On a shared Linux machine another user can pre-create the folder and swap the file between the check and the
  rename, or leave a symlink at the `.part` name to overwrite a file of the victim's.
- Fix: download beside the executable with `os.CreateTemp` (which also fixes PLAT-04), in a 0700 folder, and hash
  the file by its open descriptor just before the swap.

### SEC-04 The signature covers the version and the hash only
- **Medium.** Read.
- Where: `update.go:58` (`sigText(version, sha)`); the asset is picked by map key (`:238`), and its kind and the
  channel are trusted as given (`:452`, `:161`).
- Whoever controls the manifest can serve the signed Linux binary as `darwin-arm64`, or a signed zip as a plain
  binary. It verifies, is installed, cannot start, and by PLAT-05 is never rolled back. A signed beta can be
  replayed to the stable channel. Downgrades are correctly refused.
- Fix: sign `version, os-arch, kind, channel, sha256`, or the whole manifest.

### SEC-05 The beta channel is not wired: a `-beta` tag goes to every stable user
- **Medium.** Read.
- Where: `release.yml:66-67` never passes a channel and always uploads `update.json`; the Worker serves `beta.json`
  only for `?channel=beta` (`cloudflare/update/worker.js:17`), which the app never sends (`update.go:147`).
  `update.go:121` compares pre-release strings as text, so `beta.10` sorts before `beta.2`.
- Fix: derive the channel and the object name from the tag in the workflow; have the app ask for its channel;
  compare pre-release parts as numbers.

### SEC-06 The bridge sends the bearer token to whatever listens on the port
- **Medium.** Read.
- Where: `mcpbridge.go:107-118` is a bare TCP dial; `:57` then sends the token. When the app cannot bind the port it
  only records an error and turns MCP off (`mcp_server.go:39-42`).
- A local process that binds 7457 first collects the token from the next `celestine mcp` and uses it later against the
  real server.
- Fix: have the server prove it holds the token before the token is sent (an HMAC of a nonce), or move the bridge
  to a unix socket or named pipe in a 0700 folder.

### SEC-07 `run_command` reaches collections the agent may not write or see
- **Medium.** Read.
- Where: `movecmd.go:63-78` makes a command "Sayfayı taşı: <collection>" for every other collection; the prefix is
  in an allowed group (`palgroups.go:19`); `mcpMayChange` (`mcp_server.go:563-579`) checks only the open page's
  collection. The `move_page` tool checks the destination; this path does not.
- `list_commands` (`mcp_server.go:487-491`) returns every allowed title, which includes the names of hidden
  collections and the title of a hidden page the user has open.
- Fix: resolve the destination of move commands and require write on it; drop from the list any command whose title
  names something the agent may not see.

### SEC-08 `run_command` can put the user's clipboard into a page the agent then reads
- **Medium.** Read.
- Where: "Panodan tablo ekle (CSV)" (`palette.go:126`) is in an allowed group; `tables.go:948-959` reads the
  clipboard; any text whose first line has a comma, semicolon or tab is accepted.
- Fix: an explicit deny list beside the allowed groups; take this command off the agent's list.

### SEC-09 A panic in a tool handler ends the app with no crash log
- **Medium.** Verified by structure; no concrete trigger found.
- Where: the MCP SDK runs handlers on its own goroutines with no `recover`; `a.do` only unlocks; `a.guard()` is not
  on that path. `run_command` runs arbitrary UI commands off the UI thread, including ones that open windows.
- Fix: a receiving middleware with a `recover` that writes the crash note and returns a tool error; post
  `run_command` to the UI thread.

### SEC-10 The signing key shares a job with third-party actions and with `go test`
- **Medium.** Read.
- Where: `release.yml:25`, `:27`, `:79` pin actions by tag, not by commit; the job has `contents: write`; line 33
  runs `go test ./...`, which is all dependency code; line 64 writes the secret key into the workspace as a file.
- A moved action tag or a compromised dependency reads the ed25519 key. Every installed copy then accepts that
  party's "update".
- Fix: split into a build job with no secrets and read-only permissions, and a sign-and-publish job that downloads
  the artifacts and runs only `scripts/release`, behind a protected environment, with the key passed by environment
  or stdin. Pin all actions by SHA. Add a step that verifies the manifest with the public key linked into the
  binaries.

### SEC-11 `release.key` is not ignored by git
- **Medium.** Read.
- Where: `scripts/release/main.go:44` writes the secret key as `release.key` in the working directory, which is the
  repository root when run as documented. `.gitignore` has no line for it.
- One `git add -A` puts the update-signing key into the history.
- Fix: add `/release.key`, `/update.json`, `/release-notes.md`, `/notes.md` to `.gitignore`; better, have `keygen`
  print the secret and never write it inside the repository.

### SEC-12 Cover and banner names from files are joined, read and removed unchecked
- **Medium in a folder library, low in the one-file library.** Read.
- Where: `properties.go:246` (`p.Cover = meta["cover"]`), `library.go:49` (`Banner`); joined at `cover.go:160`,
  `:207`, `banner.go:86`; removed at `cover.go:48-49`, `:74-75`, `banner.go:37`, `:54`.
- A page from a synced folder or an imported vault with `cover: ../../../somewhere/file`: when the user changes or
  removes the cover, that file is deleted. In the one-file library `Store.Clean` confines the path to the library,
  so the worst case is another page's file.
- Fix: `SafeName` on both when read, and again before `Remove`.

### SEC-13 No size or pixel limits on what is read in
- **Medium.** Read.
- Where: `os.ReadFile` of the whole file at `attachfile.go:22`, `images.go:222`, `cover.go:199`, `importmd.go:110`,
  `:216`, all inside `a.do` or a frame; `images.go:116-117` decodes with no dimension check; the decoded-image cache
  (`images.go:127-130`) never evicts.
- A multi-gigabyte file dropped on the text is read into memory and into the library while the window is frozen. A
  small PNG that declares a huge canvas exhausts memory each time its page opens. A long session keeps every picture
  it ever showed decoded.
- Fix: stat first and refuse or warn above a limit; read the header with `image.DecodeConfig` and cap the pixels;
  make the cache an LRU; read off the lock (see PERF-16).

### SEC-14 Dropping a `.md` file can walk the whole folder it came from
- **Medium.** Read.
- Where: `dropfiles.go:39` passes the file's folder as the import root; `importmd.go:199-211` walks it recursively
  the first time a picture is not found beside the file, and takes the first file of that name.
- A note dropped from the home folder that names one missing picture walks the home folder inside a frame, and may
  attach an unrelated picture with the same name.
- Fix: for a single dropped file, look only beside it.

### SEC-15 Backups are world-readable copies of a file the app keeps private
- **Medium.** Read.
- Where: the library file is `0600` (`store_sql.go:63`), but the backup folder is `0755` (`backup.go:76`), the `.gz`
  and the `VACUUM INTO` snapshot are `0644` (`backup.go:109`, `backup_sql.go:18`), and the config folder is first
  made `0755` (`crash.go:86`). A backup holds every page and the settings row, which carries the tokens and the
  encryption key when no keychain answers. Crash logs and the update mark are `0644` too.
- Fix: `0700` folders and `0600` files for everything under the config folder.

### SEC-16 "Every write is kept as a version and can be taken back in one click" is not true for several writes
- **Low.** Read.
- Where: the claim is in `mcp_help.go:41`. `undoAgent` (`agentedits.go:94-128`) rewrites file contents and trashes
  made pages. Not undone or not recorded: `trash_page`, `move_page`, `create_collection`, `add_property`,
  `update_collection`, `restore_page`, and everything done through `run_command`. In a folder library a reload
  replaces every page object and silently empties the record.
- Fix: record operations as well as contents, keyed by page path; correct the help text until then.

### SEC-17 Smaller permission gaps in MCP
- **Low.** Read.
- A hidden collection that is in the trash is listed and can be restored whenever `*` allows it
  (`mcp_more.go:176`, `:219-220`).
- `get_daily_note` answers "the journal has no page for…" before checking access (`mcp_more.go:127-131`).
- `restore_version` returns ok when the restore failed (`historyui.go:120-131`).
- `move_page` does not refuse journal or dynamic collections as the window does.
- "Yeni sürümü denetle" is allowed and skips the "updates off" setting (`update.go:183`): an agent can make an
  offline app go to the network and, with automatic updates on, start an install.
- A settings import does not exclude `mcpAccess`, `updateUrl`, `updateAuto` (`settingsx.go:218-219`): a settings
  file from someone else silently re-opens hidden collections to agents and can point updates elsewhere.

### SEC-18 The server can be turned on from outside the window, and stays on
- **Low.** Read.
- Where: `main.go:573-579`, `:604`: `--mcp`, sent by any `celestine mcp` or any process that runs the binary, calls
  `setMCP(true)`, which saves the setting. "Off by default" holds only until some agent config starts the bridge.
- Page text is returned to agents as it is, and nothing in the server's instructions says it is data, not
  instructions. `trash_page` asks nothing.
- Fix: do not persist a server started by `--mcp`; show a notice when it happens; add the sentence.

### SEC-19 `mcpsetup` rewrites other apps' config through a generic map
- **Low.** Read.
- Where: `mcpsetup.go:48-84`: the file is round-tripped through `map[string]any` (keys re-sorted, integers above
  2^53 changed), renamed over the path (a symlinked dotfile becomes a regular file, mode `0600`), with a fixed temp
  name. `mcpsetup.go:98` runs `claude mcp add` with no timeout from a click handler, inside the frame.
- Fix: resolve symlinks, keep the mode, run the command in a guarded goroutine with a timeout.

### SEC-20 Hardening the unpack and the fetch
- **Low**, because the zip is signature-checked first. Read.
- `update.go:330-338`: the traversal check is `HasPrefix(name, "..")` and the top folder is trimmed by string
  prefix, so an entry `celestine.app../x` lands one level outside the staging folder. Symlink entries are written as
  regular files; there is no cap on unpacked size; the zip is never removed.
- `HTTPSOnly` applies to the first URL only (`update.go:144`, `:242`); the default client follows a redirect to
  `http://`. Set `CheckRedirect`.
- `release.yml:53-66`, `:76` paste `${{ github.ref_name }}` into shell: a tag may contain `$(…)`. Pass it through
  `env:`.
- `scripts/release/main.go:58` does not check the key's length; `ed25519.Sign` panics on a bad file.

### SEC-21 Smaller points
- Import follows symlinked `.md` files (`importmd.go:110`): a vault from a git clone can pull any readable file into
  the library, and from there into sync. Use `Lstat` and skip.
- Page export writes each attachment beside the chosen path with `os.WriteFile`, replacing same-named files without
  asking (`pagemenu.go:166-181`).
- CSV export writes cells that begin with `=`, `+`, `-`, `@` as they are (`tablefn.go:140-154`): a formula planted
  by an import or an agent runs when the file is opened in a spreadsheet. Prefix such cells with `'`. The server's
  CSV export already does.
- `backup.go:228` trusts any path in `restore-from`.
- Android backs the WebView storage, with the notes and the sync secrets in plain text, up to the cloud: see
  PLAT-16.
- On macOS the keychain library goes through `/usr/bin/security`, so items may be readable by any process of the
  user without a prompt. Suspected; `security find-generic-password -s celestine -w` from another terminal shows it.

## Grep inventory, root package, non-test

| Pattern | Hits | Note |
| --- | --- | --- |
| `exec.Command` | `mcpbridge.go:37`, `mcpsetup.go:98`, `updateui.go:25`, `main.go:508` | Own executable or `claude` from `LookPath`; no shell, no argument from page text or the network |
| `net.Listen` | `mcp_server.go:38` | Loopback only |
| `io.ReadAll` | `account.go:154` | Behind a 1 MiB limit; the manifest is capped too |
| HTTP clients | `account.go:110` (30 s), `update.go:149` (15 s), `update.go:252` (10 min), `mcpbridge.go:68` (none, local stream) | The MCP server sets `ReadHeaderTimeout` only; no body cap, reached after the token check |
| `InsecureSkipVerify` | none | |
| `math/rand` | `caretfx.go`, `caretfx_util.go`, `journal.go` | Effects and "a random day" only |
| `time.Now()` in app code | about 35 uses in 16 files | Against the `a.clock()` rule; the update schedule, backup rotation and conflict names cannot be driven by a test clock |
| `log.Fatal` | `main.go:444`, `:461`, `:470`, `:476`, `:609`, `:628` | See DATA-12 |
| goroutines without `guard` | `chips.go:407`, `propkinds.go:136`, `:140`, `links.go:63`, `mcp_server.go:30`, `:48` | One-line shell opens and the server |

## Sound

The token (192 bits, `crypto/rand`, constant-time compare, `Host` checked first, loopback bind, no CORS, never
written into other apps' config). Access checked in every data tool, both ends of `move_page` included, with hidden
things answering "not there". The dangerous command groups (sync target, restore, reset, file dialogs, export,
macros) left out of `run_command`. The updater failing closed with no key, refusing downgrades, capping the manifest
and the asset. No remote content ever fetched for a page. No secrets in tracked files.
