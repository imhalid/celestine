# 02 Sync

The sync engine (`internal/library/sync.go`, `merge.go`, `syncbase.go`, `synctrash.go`), the app side
(`syncapp.go`, `account.go`, `profile.go`, `keychain*.go`) and the server (`cloudflare/sync/`).

Back to the [index](README.md). "Reproduced" here means a throwaway test was run against the real package with a
folder remote; no live server was contacted, so nothing is known about what is deployed.

## How it works

- **The engine.** For each file the state keeps the local and remote hashes of the last pass (`SyncState.Files`). A
  pass walks the local tree, lists the remote, and for each name: only local changed, send (conditional on the
  remote hash seen); only remote changed, fetch or queue for removal; both changed, merge a page block by block
  (`MergePage`) or let the remote win and keep ours beside it as a conflict copy. Removals go to the trash for 30
  days. The state is saved once, at the end of the pass.
- **Remotes.** A folder (`DirRemote`) or the Worker (`httpRemote`, 60 s timeout, HTTPS only). Both are always
  wrapped in `EncRemote`: no key, no sync.
- **Encryption.** One 32-byte data key. A file is AES-256-GCM with a random 96-bit nonce, no associated data
  (`sync.go:434-438`). A remote name is HMAC-SHA256(key, path). `_index` is the sealed map from opaque name to path.
- **The key.** With an account: a random key made at registration, wrapped under a key derived from the password
  with PBKDF2-HMAC-SHA256, 200 000 rounds, salt `"ryudit/v1/"+email`. With a passphrase: the key itself is PBKDF2 of
  the passphrase with a 16-byte salt kept in clear on the remote.
- **What the server sees.** The e-mail, device names (the hostname), IPs, and per file a stable opaque name, its
  size, its write times. Not names, tree shape or content.
- **The server.** D1 for accounts, devices, invites, rate-limit counters; R2 for `u<id>/<name>` (files) and
  `v<id>/<name>@<ms>` (up to 10 versions). Writes are conditional on the object's etag; quota is reserved in one SQL
  statement before the write.

## Findings

### SYNC-01 Turning a collection's sync off deletes it on the server and on every other device
- **High.** Reproduced; re-checked.
- Where: `sync.go:824-835`. The work list is built from local names (the walk skips collections chosen out,
  `:714`), remote names (skip applied, `:829`), and every name in the state with no skip (`:833-835`). For a file of
  a collection that was synced before, the local hash is empty, the remote still has it, so it reads as "deleted
  here" and `:877-878` runs `r.Delete`.
- Run: two devices hold `Ozel/B/page.json`; one syncs with `Ozel` switched off. Result: one deletion, the remote
  entry becomes a tombstone, the other device's next pass moves its copy to the trash.
- The existing test's comment (`sync_test.go:156`, "choosing it out later does not delete it from the other side")
  does not test this: the collection was never synced there.
- Fix: drop from the work list any state name whose top folder is skipped, leaving the state entry alone so that
  switching it back on is a no-op. Add the two-device test.

### SYNC-02 The sync state is never reset when the remote changes
- **High.** Reproduced for the engine; the app side read.
- Where: the state is keyed only by the library (`syncapp.go:150-175`). Nothing clears it on sign-in
  (`account.go:332-348`), sign-out (`:351-357`), a mode change (`syncapp.go:650-654`) or an edit of the folder, URL
  or token (`:687-690`).
- What happens: every file is recorded as "synced, remote hash X". Against a different, empty remote each one reads
  as "unchanged here, gone there".
  - Run with 6 files: six deletions. The pages go to the trash, the root-level file is deleted outright, and the
    next pass uploads only the trash folders to the new account.
  - Run with 30 files: the guard holds all 30, nothing is ever uploaded, and editing one makes a conflict copy.
- Who meets it: folder sync to account; a self-hosted token to account; signing out and into another account; an
  account deleted and opened again; a self-hoster who recreates the bucket.
- Fix: keep an identity of the remote in the state (server and account, or the folder, plus a random id in a sealed
  object on the remote) and start from an empty state when it differs. Clear it on sign-out.

### SYNC-03 The mass-removal guard: what it lets through, and its only button is the destructive one
- **Medium.** Reproduced and read.
- (a) Libraries under 20 synced files are not covered (`sync.go:857`). Run: the index removed from the remote, six
  files, all six removed locally. A missing index is read as "nothing was put" (`:470-474`).
- (b) An entry listed with an empty hash counts as present, so the guard is skipped (`:862-865`, `:902`). The Worker
  lists `hash: ""` for any object without its metadata (`worker.js:197`), such as objects restored into the bucket
  with another tool. Run with 40 files: 40 removals, none held.
- (c) Tombstones are exempt by design, so a server that answers "deleted" for everything empties every device into
  its trash.
- (d) Root-level files and anything already under the trash are deleted, not trashed (`synctrash.go:36-42`).
- (e) When files are held, the settings offer only "Silmeleri uygula" (`syncapp.go:797-806`). In the case the guard
  exists for, the right action is "send mine up again"; there is none, and held files are never re-uploaded.
- Fix: treat "not deleted, no hash" as an error for that name; hold whenever all previously synced files vanish,
  whatever the count; give the held state a second action that forgets the held names so they go up as new; trash
  root-level files too.

### SYNC-04 Sealed files are not bound to their name or version
- **Medium.** Reproduced.
- Where: `sync.go:437` seals with no associated data; `:564-573` accepts whatever opens under the key. The listed
  hash is the server's word and is never checked.
- Run: the sealed objects of two pages exchanged on the remote. A new device and the original device both took the
  swap with no error, overwriting the local pages. An old blob served under the same name is accepted likewise, and
  an old index hides files, which then read as deleted (SYNC-03a).
- What the "zero knowledge" claim holds against: a passive server and a leak of the database or the bucket. It does
  not hold for integrity against a server that acts.
- Fix: seal with `AAD = "ryudit/file/v1\x00" + path`, and another label for the index; put a generation that only
  grows inside the sealed index and refuse a lower one; optionally keep each file's plaintext hash in the index.

### SYNC-05 A new file at the same path on two devices: one is overwritten without a conflict copy
- **Medium.** Read; a two-goroutine test with a pause before the index flush would show it.
- Where: `sync.go:581-589`. On a conflict for a name not yet in the index, the object is assumed to be this device's
  own orphan and written over. Between one device's `Put` of a new name and its index flush (minutes in a first
  pass), another device creating the same path does exactly that. Today's journal page is the natural case.
- Fix: write the index first. At the start of a pass compute the new names, put the index once, then the objects; an
  index name without an object is already harmless. That also removes the orphan branch and the flush-failure
  bookkeeping.

### SYNC-06 A full compressed backup before every pass, which evicts the backups that matter
- **Medium.** Read.
- Where: `syncapp.go:261-264` runs `backupTo(…, "senkron", …)` and `pruneBackups` in every pass; passes start two
  seconds after each pause in typing, on focus, and on every move of the server's counter. The backup snapshots the
  whole file and gzips it at best compression, also while offline.
- `pruneBackups` keeps the newest five labelled backups of any label (`backup.go:176-181`). After five passes, about
  ten seconds of typing, every "before an import" and by-hand backup is gone, and the five kept are seconds apart.
- With DATA-14 this backup also blocks saves while it reads.
- Fix: take it only when the pass is about to pull, merge or remove (decide after the listing), at most once in
  some minutes, and give `senkron` a retention bucket of its own.

### SYNC-07 Every pass costs the whole library
- **Medium.** Read.
- With one changed page, server mode: the whole index downloaded and opened again, the whole listing (about 150
  bytes a file, 180 days of tombstones included), the upload, the counter. The counter is used to decide when to
  run, never to skip the listing.
- Locally: a full tree walk and the whole state marshalled into one row.
- A folder remote reads and hashes every remote file each pass (`sync.go:78-96`); on iCloud or Dropbox that is the
  whole library's bytes every few seconds of typing.
- The index only grows; trash folders and conflict copies add to it.
- An interrupted pass saves no state, so the next finds every transferred file "changed on both sides" and downloads
  each one to compare.
- Fix: skip the listing when the counter has not moved since this device's last pass; conditional GET for the
  index; a `since=` listing on the Worker; cache size and mtime to hash for a folder remote; save the state every few
  hundred files.

### SYNC-08 The rate limiter's cleanup wipes the hourly counters
- **Medium.** Read, arithmetic; re-checked.
- Where: `auth.js:58-62`. `win` is in units of the caller's window, and the cleanup deletes `WHERE win < now/window
  - 2` across all keys. A cleanup fired from a 900-second limiter compares against a number about four times larger
  than any 3600-second row's, so it deletes every hourly row, current ones included.
- Effect: each call to `/api/login`, and each call by a signed-in device to `/api/password` or `/api/recovery`, has
  a 2 % chance of zeroing the per-IP login limit, the invite-code limits, registration, waitlist and recovery. About
  fifty cheap requests reset them at will.
- Fix: delete only rows of the same window (`key LIKE '%:' || ?`), or store an expiry and delete by time.

### SYNC-09 "Parolayı değiştir" in passphrase mode replaces the key without re-sealing anything
- **Medium.** Reproduced.
- Where: `syncapp.go:836-860`. `OpenEnc` derives from the same salt and never checks the result against the remote,
  then the old key is overwritten. Sync then fails with "wrong passphrase or key" on every pass, and the recovery
  key shown cannot be entered anywhere in this mode. A typo when joining a second device has the same result.
- Also: this runs the network and PBKDF2 on the frame, under the app's lock.
- Fix: in `OpenEnc`, when an index exists, open it with the derived key and refuse if that fails; implement the
  change as wrapping a random key, as the account does, or remove the action; run it through a goroutine.

### SYNC-10 Password stretching is the only thing between a server leak and the data
- **Medium.** Read.
- PBKDF2-HMAC-SHA256 at 200 000 rounds is below the current recommendation for that function (600 000) and is not
  memory-hard. The account salt is the e-mail: public and fixed. The server holds the wrapped key, which gives a
  yes or no per guess. The data key is never rotated: a password change, a recovery and removing a device all keep
  it, so an old password plus an old copy of the wrapped key opens data written afterwards.
- Fix: Argon2id, or more rounds, under a new version label with parameters returned by a pre-login call; a random
  salt stored on the server mixed in; say in the settings that changing the password does not re-key.

### SYNC-11 Transfers: one 60-second deadline, no size check, no stop on repeated failure
- **Medium.** Read.
- The client timeout covers the whole body (`sync.go:186`): a 10 MB attachment on a slow uplink can never be sent
  and is retried in full on every pass.
- The Worker refuses bodies over 50 MiB and over quota with 413; the client checks nothing locally, sends the file
  anyway, and shows only "sunucu 413 döndü" on every pass. Quota full and file too large look the same.
- A revoked token (401) shows as "offline".
- A server that stops answering mid-pass costs up to 60 s for each remaining file.
- Fix: a deadline on headers and an idle timeout on the body; refuse over 50 MiB locally with a clear line; name
  401 and 413; stop the pass after a few transport errors in a row.

### SYNC-12 Quota drifts upward when a version cannot be kept
- **Low.** Read.
- Where: `worker.js:231` reserves; `:234` calls `keepVersion` outside the `try` that refunds. Nothing recounts.
- Fix: one `try/finally` from the reservation to the end; a recount in the cron or the admin panel.

### SYNC-13 Failed admin logins are counted but never refused
- **Low.** Read.
- Where: `auth.js:371-374` calls `allow(…)` and discards the answer.
- Fix: check `full(…)` before the comparison and answer 429.

### SYNC-14 Limiters keyed by the victim's name allow lockout
- **Low.** Read.
- Ten requests in 15 minutes naming an address block that address's sign-in from anywhere (`auth.js:280`); five an
  hour block its recovery (`:296`); 200 wrong invite codes an hour from any 20 addresses stop every registration.
- Fix: count per name and IP first, with a much higher cap per name alone.

### SYNC-15 Remote deletions of ordinary files become trash "pages" that every device uploads again
- **Low.** Read; the restore behaviour suspected.
- Where: `synctrash.go:67-87`. Any removed file whose folder is not the root makes a trash item of kind page named
  after its folder. A picture removed from a page arrives on the other devices as a trashed "page" holding one file,
  and that folder is then synced to everyone. See DATA-20 for what restoring such an item does.
- Fix: trash only when the removed set contains the folder's `page.json` or `collection.json`.

### SYNC-16 The legacy registration path ignores the invite's e-mail binding
- **Low.** Read. `auth.js:242-254`: with no `email` in the body, any unused code is accepted for any username.
  Remove the path, or allow it only for invites made before a date.

### SYNC-17 Device tokens never expire
- **Low.** Read. `auth.js:89-98` checks only existence and status; a row is added on every sign-in with no cap; the
  device name sent is the hostname (`account.go:170-175`). Expire devices unseen for some days in the cron, cap them
  per account, and let the user name the device.

### SYNC-18 One key for three jobs
- **Low.** Read. The data key is the AES key, the HMAC key of names and the HMAC key of the recovery proof, whose
  message is `"ryudit/recovery-auth"`. A library file at that path is stored on the server under exactly the value
  `/api/recover` accepts. Derive three keys with HKDF and distinct labels.

### SYNC-19 Smaller points
- "Deleted there, changed here" keeps ours and also makes a conflict copy of it with the same bytes
  (`sync.go:959-974`). Reproduced. Test for the empty remote hash first.
- "Versions on the server" cannot work for a folder library: `profile.go:254` builds a name from an absolute path.
  Suspected.
- Reserved names are compared exactly (`sync.go:829`): `.GIT/config` from a device holding the key passes. Compare
  with `EqualFold`.
- The keychain entry is scoped by the settings file's absolute path (`keychain.go:33-43`): moving the library leaves
  the key behind and signs the library out. A failed keychain read at start looks like a sign-out
  (`keychain.go:60-63`).
- The self-hosted token field is drawn in clear (`syncapp.go:667-675` takes `secret` and discards it).
- Server: bodies are buffered before the size check (`auth.js:72-76`); Turnstile passes when its secret is unset
  (`auth.js:126`); the Worker also answers on `workers.dev`, outside any rule on the zone; no HSTS header on the
  pages (may be set on the zone).
- `docs/23` says the deploy lacks the cron trigger; `deploy.sh:39-43` now builds it from `wrangler.toml` and refuses
  without it. Whether the live Worker has it was not checked.

### SYNC-20 Tests
- Nothing covers a collection switched off after it was synced (SYNC-01), a changed remote (SYNC-02), a missing or
  older index (SYNC-03, SYNC-04), two devices creating one path (SYNC-05), or a passphrase change (SYNC-09).
- The Worker has no test that runs by default: the three live tests skip without an environment variable. The
  limiter (SYNC-08), quota refunds (SYNC-12) and `keepVersion` have none. A `wrangler dev --local` job, or Miniflare
  unit tests, would cover them; see QA-01.

## Sound

Per-account isolation (`u<id>/…` built from the token's account, flat keys, ids never reused); bound statements
throughout; hashed tokens and codes with constant-time compares; the double conditional write; remote names filtered
before use; a file saved during a pass is not overwritten; the CSV export guards formula cells; a strict CSP on the
pages; HTTPS enforced in both clients; and `merge3`, which could not be made to drop text.
