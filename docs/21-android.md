# 21 — Android

The Android app is `mobile/`: an app of its own, written for a phone, on the same library as the desktop (#633). It
replaced the web build in a WebView that was here before (`android/`, removed), which this page described until then.

## The way chosen

Of the three ways #573 weighed (the WebView, a Kotlin client with the library bound through gomobile, MyGo's own
Android host), the app is the third, with a twist the maintainer asked for: not the desktop app drawn on a phone, but a
second program with an interface made for a hand, sharing only the library.

- **One library, two interfaces.** `mobile/` is `package main` in the same module, so it imports `internal/library`
  as the desktop does: the page file, the document (`library.Doc`), collections, properties, the trash and, next, sync.
  A page written on the phone is the desktop's page. Nothing of the desktop's interface (`package main` at the root,
  Ryugo) is in it, and nothing of it is in the desktop.
- **MyGo's Android host.** The Go code is a shared library drawn with OpenGL ES by MyGo's Kotlin host
  (`mygo build -platform android`): native text input with Gboard's suggestions and Turkish letters, the system's back,
  the safe area and the keyboard's room, Material's bars and ripples (`ui/mobile`).
- **The desktop's rules, kept the same.** The journal is the desktop's collection marked Journal; a day is the page
  whose `celestine:day` property is that day, titled in the journal's format; past days are locked (the library refuses
  their edits) and no day goes to the trash. `mobile/journal.go` follows `journal.go` of the desktop for this.

## The app

Its look is its own, made for a hand, not the desktop's nor Material's: blocks of rounded paper on a warm ground (soft
ink on a deep one by night), Geist for the words and heavy for what is big, a color for each collection. Blocks press
in a little under a finger, their corners kept.

- **No bar of tabs.** A round button at the top left opens the menu, which covers the screen over the screen blurred,
  its places (Home, Library, Tasks, Search, Settings) large and coming in one after another. A round **+** at the bottom
  is the only other control: tapped, the screen blurs and what it makes rises over it (today's page, a note, a note in a
  collection, a search), the + turning to a ×. Android's back closes what is open first.
- **Home** is blocks: the day, large, on a field of soft colors that drift slowly (a shader, still with reduced
  motion); today's page with its first lines and words; the tasks open in the library and the days in a row the
  journal has words; the collections, colored, in a row that scrolls sideways; the pages written lately.
- **Library** is the collections two to a row; a collection is its pages as blocks under its name, in its color.
- **Tasks** are every task not done, by page; a task taps through to its page.
- **Search** finds titles and words as they are typed, Turkish letters by their plain ones too (`internal/textnorm`).
- **Settings** choose light, dark or the system's look as three blocks (the system's bars follow).
- **A page** is the whole screen to write in and nothing else: its collection, its title (asked for first on a new
  note), its text; a line's Markdown makes it a heading, a list or a checklist (`# `, `- `, `[] `); Done at the top puts
  the keyboard away, ··· opens a sheet (copy the text, move to the trash). Saved 0.8 s after the typing stops, as the
  screen is left, and as the app goes to the background.

What the rich text has no run for (a link to a page, a day, a table, a picture, a footnote) rides through the editor as
an attachment carrying the library's object, written back as it came (`mobile/body.go`): a page from the desktop loses
nothing for having been opened on the phone.

The library is a folder of page files in the app's own storage, not the desktop's SQLite file: modernc's SQLite calls
`lstat` directly, which the seccomp filter of Android's x86_64 emulator refuses, and sync works on a folder as well.

## Building

```sh
scripts/android.sh            # mobile/dist/android/: an .apk and an .aab for every ABI
scripts/android.sh install    # for the ABI adb sees, installed and started
scripts/android.sh dev        # mygo dev: rebuilt as the Go code changes
```

It needs Android Studio's SDK, its NDK and a JDK; `go run github.com/egoist/mygo/cmd/mygo doctor` (from `mobile/`) says
what is missing. On this machine the emulator boots only with `-gpu host`. The icon is the app's
(`mobile/resources/`, made by `scripts/icons.sh`), adaptive on Android 8 and later. `go test ./mobile` drives the
screens with MyGo's Tester.

## Found on the way

- **A message asked for at the program's start froze MyGo's backend.** The desktop's table labels were a package
  variable filled with messages, and asking for a message asks MyGo for the locale, which makes its backend. On Android
  that happened as the library loaded, before the host was there, and the app never got past its splash. They are made
  as a table is drawn now (`views.go`), which also has them follow a change of language. Nothing that reaches MyGo may
  run in a package's initialization.
- **The time zone** is the phone's: MyGo asks the host for it as Go starts, so `time.Local` is right for the journal's
  days.

## Next

- Sync: signing in to the account and syncing the library, as the desktop does (`syncapp.go`, `account.go`), its
  protocol and keys moved to a package both use.
- Sharing text into the app (Android's share menu) and a widget for today's page.
- Trying it on a phone (Samsung Galaxy S21), and a release build signed for Google Play.
