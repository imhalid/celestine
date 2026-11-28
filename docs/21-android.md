# 21 — Android

The Android app (`mobile/`) is **One Day**: open the phone and you are already on Today's sheet of paper — nothing
else to browse, no tabs, no collections, no past-day archive, no floating chrome. Only today can be written on the
phone; past days and the full library live on desktop Celestine via end-to-end encrypted sync (#633, #634).

## Architecture

- **One library, two interfaces.** `mobile/` is `package main` in the same Go module and shares `internal/library`,
  `internal/account`, and `internal/textnorm` with desktop Celestine. Nothing from desktop UI (`package main` at root
  or Ryugo) is imported.
- **Desktop journal contract (`mobile/journal.go`).** Today's sheet writes directly to the desktop's `Journal`
  collection with `celestine:day` set to today's date (`YYYY-MM-DD`). No empty file is written until the user actually
  types or adds a task.
- **Automatic midnight page turn (`mobile/app.go`).** When midnight crosses, yesterday's page is flushed, sealed on
  disk, and the paper turns cleanly onto a fresh sheet for the new day.
- **Lossless desktop round-trip (`mobile/body.go`).** Desktop-only objects (page links, dates, tables, images,
  footnotes) ride through `richtext.Attachment` untouched so editing today's page on both phone and desktop loses
  nothing.
- **Folder storage.** Uses `library.OSFS{}` rather than SQLite because `modernc.org/sqlite` issues `lstat` syscalls
  rejected by the Android x86_64 emulator seccomp filter.

## Design: One Day

- **Continuous Solar & Weather Sky-Paper (`mobile/theme.go`, `mobile/weather.go`, `mobile/app.go`, `mobile/today.go`).**
  A full-bleed vertical gradient + hardware-tiled cotton-paper grain runs edge-to-edge under both the top status/battery
  bar and the bottom gesture bar. Anchored to the user's real local sunrise and sunset via Open-Meteo (zero API key,
  cached 30 min in `weather.json`), the gradient evolves smoothly minute by minute: crisp airy morning sky, bright
  midday cotton, glowing honey-amber golden hour in late afternoon, deep shadowed evening blue after dusk, and dark calm
  night — further modulated by live weather (clear, partly cloudy, overcast, misty, rainy slate, stormy, snowy). Android
  system bar icons (`mygo.SystemBars.SetStyle`) adapt automatically to paper luminance.
- **Swiss-Japanese Stationery Layout (`mobile/today.go`).** A whisper-thin vertical margin rule runs down the left edge
  like a physical notebook spine; the atmospheric time phase (`Night · 00:52`) sits at the top, an oversized sculptural
  day numeral balances the right-aligned weekday & month above a flat 1.5px daylight hairline, and a floating rounded
  pen-tray pill hovers above the keyboard while editing.
- **Writing ergonomics (`mobile/today.go`).** Full-height rich-text editor with an Android touch overlay so finger
  drags scroll smoothly, taps place the caret, and taps on a task checkbox tick directly without popping up the
  keyboard (#654, #655). While writing, a slim keyboard accessory bar offers Task, Heading, Bold, and Done; dismissing
  Android's keyboard releases editor focus cleanly (#656).
- **Discreet Sync & Appearance Sheet (`mobile/settings.go`, `mobile/signin.go`).** A single quiet cloud icon in the
  top-right corner opens a minimal sheet for System/Light/Dark appearance, desktop E2E sync sign-in/sync-now, and
  copying today's text.

## Building & Testing

```sh
go test -race ./mobile/...    # drives the One Day UI with MyGo's Tester
scripts/android.sh            # builds mobile/dist/android/ .apk and .aab
scripts/android.sh install    # installs and launches on connected device/emulator
scripts/android.sh dev        # live rebuild loop via mygo dev
```

## Sync

`mobile/sync.go` runs the desktop's end-to-end encrypted sync (`library.SyncCounted`, `internal/account`) using
Android's keystore via `mygo.Secrets`. Sync runs on launch, foreground resume (>20 s away), 2 s after typing stops,
every 5 minutes while open, and on demand from the corner Sync sheet. `scripts/worker-test.sh` verifies live sync via
`TestSyncLive`.
