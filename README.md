<p align="center"><img src="assets/icon-256.png" width="128" alt="Ryudit"></p>

# Ryudit

**The notes app you've been looking for.**

Ryudit is a fast, native notes app for Mac, Windows and Linux. You write, and it stays out of your way: type `#` and a
space and you have a heading, `-` and you have a list, `[]` and you have a task. The marks disappear as you type, so
your page always looks finished.

## What it does

- **Writing that formats itself.** Headings, lists, tasks, quotes, code, tables and pictures, all from the keyboard.
- **Collections.** Group pages into collections and see them as a list, a table, a board or a calendar. Add your own
  properties (a status, a date, a number) to every page in a collection.
- **A journal.** A page for every day, one click away.
- **Links between pages.** Type `@` to link a page or a date; a click takes you there.
- **Everything at your fingertips.** ⌘K finds any page, ⌘P runs any command; most of the app works without a mouse.
- **Sync across your devices.** Sign in and your notes follow you to your other computers. What you write goes up a
  couple of seconds after you stop typing.
- **Private by design.** Your library is encrypted on your computer before it leaves it; the server only ever sees
  scrambled data. A recovery key, shown once when you make your account, is the only way back if you forget your
  password, so keep it somewhere safe.
- **Safe from accidents.** Daily backups, a trash you can restore from, and earlier versions of every page.
- **Make it yours.** Light and dark themes, color themes, typefaces, playful animated carets, and English or Turkish.
- **Works with AI assistants.** Turn it on in the settings and assistants such as Claude can read and write your
  notes, with your permission.

## Get it

Download the latest version for your computer:
[macOS (Apple silicon)](https://dl.ryudit.halid.dev/download/darwin-arm64),
[macOS (Intel)](https://dl.ryudit.halid.dev/download/darwin-amd64),
[Windows](https://dl.ryudit.halid.dev/download/windows-amd64),
[Linux](https://dl.ryudit.halid.dev/download/linux-amd64) ([ARM](https://dl.ryudit.halid.dev/download/linux-arm64)).
Versions starting with 0 are early previews. The app tells you when a new version is out and updates itself.

The macOS app is not signed by Apple yet: the first time, right-click it and choose Open.

Ryudit is invite-only for now. Join the waitlist at [ryudit.halid.dev](https://ryudit.halid.dev); when you are
approved, an access code comes to your email, and you open your account with it in the app.

## A few keys to start with

| Keys | Does |
| --- | --- |
| ⌘K | Find a page |
| ⌘P | Run a command |
| ⌘N | New page |
| ⌘F | Find in the page |
| ⌘\ | Show or hide the sidebar |
| ⌘, | Settings |
| `/` at the start of a line | Turn it into another kind of block |

On Windows and Linux, use Ctrl in place of ⌘.

## For developers

Ryudit is written in Go. To run it from the source:

```bash
go run .
```

How it is built, the file format and the code's layout are in [docs/architecture.md](docs/architecture.md).
