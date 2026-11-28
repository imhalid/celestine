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

- **The sky (`mobile/sky.go`, `mobile/skydraw.go`, `mobile/theme.go`, `mobile/weather.go`) (#665).** A full-bleed
  sky with cotton-paper grain runs edge-to-edge under the status bar and the gesture bar. Its colors are one family
  of anchors across the day, set on the real local sunrise and sunset from Open-Meteo (no API key, cached 30 min in
  `weather.json`) and mixed minute by minute in OKLab: ink-blue night, violet before dawn, periwinkle over peach at
  dawn, washed blue morning, open sky at noon, lilac over honey at golden hour, lavender over coral at sunset, indigo
  over plum at dusk, deep evening blue. Each weather is a mood of its own in OKLab (a tint kept and a tint taken, a
  lightness, a flattening), stronger at the zenith than at the horizon so the hour still shows low down: pearl
  overcast, sea-glass mist, slate rain, a storm that darkens even the day, cool snow. Over the colors: the sun's glow
  on its arc, the moon in its real phase with a halo, stars, soft drifting clouds, bands of mist, fine slanted rain,
  soft snow, a far double flash in a storm. Rain and snow fall in the sky above the date and thin out over the text.
  The weather moves only in a live window, not while writing or under a sheet, and not when the system asks for
  reduced motion, at the lowest frame rate that looks smooth (6–24 fps). A light sky never goes below OKLab
  lightness 0.82 and a dark one never above 0.4, so the ink always reads; the ink, secondary ink, whisper labels and
  accent are moved only as far as they must to hold 7, 4.5, 3 and 3 to 1 against both ends of the sky
  (`TestSkyInkReads` sweeps every five minutes × weather × appearance). The Light and Dark appearances map the same
  family to moonlit paper by night and to deep blue by day. Android's system bar icons follow the sky's polarity.
  `SKY_SHOTS=<dir> go test -run TestSkyShots ./mobile/` renders the whole family into one contact sheet.
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
