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
ink on a deep one by night), Geist for the words and heavy for what is big, each collection in its color. Blocks press
in a little under a finger, their corners kept.

- **No bar of tabs, no button for the menu.** A screen pulled down past its top opens the menu (#638), which covers
  the screen over the screen dimmed, its places (Home, Library, Tasks, Search, Settings) large and coming in one after
  another, and closes at a tap anywhere else; its places fade into each other, where a page or a collection slides in.
  A round **+** at the bottom right is the only control: tapped, the screen dims and what it makes rises over it at the
  right (today's page, a note, a note in another collection, a search), the + turning to a ×. Android's back closes
  what is open first. The screen is dimmed, not blurred: a blur of the whole screen is drawn again each frame of the
  motion, which made it stutter on a phone (#637).
- **Home** is a summary on a dark ground, as a phone's morning note is: the weekday, large (at least four letters, its
  vowels dropped when longer: Pzrts, Cuma), with the date, which stay where they are; under them a sentence of the day
  (the tasks open, the notes written this week) with its numbers marked in coral, each opening what it counts (the
  Tasks screen, a screen of the week's notes; #643), and the pages and the days in a row. The ground is one flat color,
  the hour's (dawn, day, evening, night) deepened almost to black, on under the system's bar (#644, #645); the summary
  does not move as the screen scrolls (#641). A sheet comes over it, with no labels over its parts: the collections two
  to a row and the pages written lately (today's page is the +'s). Scrolled up, the sheet comes over the summary to the date
  row and stays there, its top round and with its handle, the ground in its corners with no line; what is on it
  scrolls up under its top and fades out into it over a tall stretch, with no band (#639, #645).
- **Collections** each have a color, the desktop's eight (c1 to c8, Ryugo's chart colors) or one by their name, and an
  icon, the desktop's. A collection is a card of one flat color: on the light look a pastel of its color, its icon bare
  and its words dark in a deep tone of it (#641); on the dark a deep, quiet tone of it, its words a light tint (#642), so that collections tell apart by their colors. A collection held (its card, or its name on
  its screen) rises out of its place into a preview at the top, the card a little larger, its place left empty, while
  its sheet comes up under it: its colors and its icons, kept at a tap and shown on the preview at once, and its
  settings, its name (kept as the sheet closes) and, for the journal, how its days are titled (the days titled again
  when that changes). A day of the journal shows as its date alone.
- **Blocks side by side group** (#647): the collections' cards and the lists of pages stand a few DIPs apart, the
  corners where two meet small and the group's outer corners large; a collection's head card is one group with its
  pages.
- **A card grows into its screen.** A collection's card, tapped, flies into its screen's head, the card larger, as the
  screens cross-fade (MyGo's Match transition), and back into its place (#646).
  The other ways in move alike (#649): a page's block grows into its page, the page scaled out of it, and back; a place
  of the menu flies into its screen's title, the same word; a collection's card on home flies to its card in the
  Library. Not while the keyboard is up, so that a page resizes at once. A page's text is shown once the page has landed, and not
  as it goes: MyGo's rich text editor is costly to draw while it moves (on a Galaxy S21 the app built each frame in
  under 2 ms and Go sat all but idle, the time being the GPU's), and a page scaled as a picture on its way was the
  rest of it, so it is laid out at each size instead (#650).
- **Haptics:** a collection held lands with a medium tap, a task ticked and the + with a light one, a color or an icon
  chosen with a selection tick; the pull that opens the menu has MyGo's own.
- **A page in a list** is its title with its collection's chip at the right, in the collection's color, and its first
  words on a line with when it was written last at the right.
- **Tasks** are every task not done, by page, ticked off where they are: written to the page at once, shown done
  until the screen is left so that a tick by mistake is taken back. A past day's tasks stay as they were.
- **Icons** are duotone: a soft fill under the stroke, in the color of where they are (`mobile/icons.go`).
- **Search** finds titles and words as they are typed, Turkish letters by their plain ones too (`internal/textnorm`).
- **Settings** choose light, dark or the system's look as three blocks (the system's bars follow), and sign in to sync.
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

## Sync

The phone signs in with the account the computer uses and syncs as the desktop does (#634):

- **One protocol.** The account's keys and the server's calls are `internal/account`, which the desktop's `account.go`
  wraps too: the keys are made from the password on the phone, only their proof leaves it, and the data key comes back
  sealed and is opened here.
- **One sync.** `mobile/sync.go` makes the desktop's pass (`library.SyncCounted`) over the phone's folder, through the
  same end-to-end encrypted remote, its state and the pages as last synced in the app's folder. A new library's own
  files (its journal) are marked pristine, so the first pass takes the account's copies with no conflict.
- **When.** As the app opens and comes back after 20 s away, 2 s after the typing stops or a page goes to the trash,
  every 5 minutes while it shows, and from Settings. Pages that came are read again into the screens; a page being
  read shows what came.
- **Secrets.** The token and the data key are in MyGo's secret store, encrypted with a key of Android's keystore; the
  settings keep only the account's address. Signing out forgets both; the pages stay.
- **Tested** against the sync Worker on this machine (`scripts/worker-test.sh` runs `TestSyncLive`): a page written
  on a computer comes to the phone, and the phone's journal and notes go to the computer, with no conflict.

## Next

- Sharing text into the app (Android's share menu) and a widget for today's page.
- Trying it on a phone (Samsung Galaxy S21), and a release build signed for Google Play.
