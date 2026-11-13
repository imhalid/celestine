package main

import (
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Backups of the library's file. Once a day, a week or a month, as the settings say, at the first start in it, a
// snapshot of it - a consistent copy, made while the app goes on - is written, compressed, to the config folder; the
// last seven days, four weeks and six months are kept. One is taken before the big operations too (an import, an
// update, a restore), and by hand. A restore is put away and made at the next start, with the file it replaces kept
// beside. A backup can be looked at in a window of its own (backupview.go), and deleted.

const (
	backupDays   = 7 // days of backups kept, the newest of each
	backupWeeks  = 4 // and weeks before, the newest of each
	backupMonths = 6 // and months before, the newest of each: a monthly backup's
	backupPre    = 5 // the ones taken before an operation, newest kept
	backupExt    = ".db.gz"
)

// backupDir is where the backups are written. A test that sets XDG_CONFIG_HOME has them there, on macOS too, where
// the config folder does not follow it: a test's backups went into the user's own folder, and showed in their list.
func backupDir() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" && testing.Testing() {
		return filepath.Join(d, "celestine", "backups")
	}
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "celestine", "backups")
	}
	return ""
}

// A Backup is a file of the backups folder.
type Backup struct {
	Path  string
	At    time.Time
	Label string // what it was taken for: "" for the daily, "elle", "içe aktarma"…
	Size  int64
}

var backupName = regexp.MustCompile(`^kitaplik-(\d{8})-(\d{6})(?:-(.+))?\.db\.gz$`)

// hardenPrivateDir ensures dir (and a "celestine" parent config folder if present) has 0700 permissions (#179).
func hardenPrivateDir(dir string) {
	if dir == "" {
		return
	}
	_ = os.Chmod(dir, 0o700)
	if parent := filepath.Dir(dir); filepath.Base(parent) == "celestine" {
		_ = os.Chmod(parent, 0o700)
	}
}

// hardenBackupDir tightens an existing backup folder to 0700 and its .db.gz files to 0600 (#179).
func hardenBackupDir(dir string) {
	hardenPrivateDir(dir)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), backupExt) {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 == 0 {
			continue
		}
		_ = os.Chmod(filepath.Join(dir, e.Name()), 0o600)
	}
}

// backups lists the folder's backups, newest first, without changing permissions (#179).
func backups(dir string) []Backup {
	ents, _ := os.ReadDir(dir)
	var out []Backup
	for _, e := range ents {
		m := backupName.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		at, err := time.ParseInLocation("20060102150405", m[1]+m[2], time.Local)
		if err != nil {
			continue
		}
		info, _ := e.Info()
		b := Backup{Path: filepath.Join(dir, e.Name()), At: at, Label: m[3]}
		if info != nil {
			b.Size = info.Size()
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out
}

// backupTo takes a backup of the store into dir: a snapshot, compressed. label says what it is for.
func backupTo(s *Store, dir, label string, now time.Time) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	hardenBackupDir(dir)
	raw := filepath.Join(dir, ".snapshot-"+now.Format("150405.000")+".db")
	os.Remove(raw)
	if err := s.Snapshot(raw); err != nil {
		os.Remove(raw)
		return "", err
	}
	defer os.Remove(raw)
	name := "kitaplik-" + now.Format("20060102-150405")
	if label != "" {
		name += "-" + strings.Map(func(r rune) rune {
			if r == ' ' || r == '/' || r == '\\' {
				return '-'
			}
			return r
		}, label)
	}
	out := filepath.Join(dir, name+backupExt)
	if err := gzipFile(raw, out); err != nil {
		os.Remove(out)
		return "", err
	}
	return out, nil
}

func gzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	f, err := os.OpenFile(dst+".tmp", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	zw, _ := gzip.NewWriterLevel(f, gzip.BestCompression)
	if _, err := io.Copy(zw, in); err != nil {
		f.Close()
		os.Remove(dst + ".tmp")
		return err
	}
	if err := zw.Close(); err != nil {
		f.Close()
		os.Remove(dst + ".tmp")
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(dst+".tmp", dst)
}

func gunzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	zr, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, zr); err != nil {
		f.Close()
		os.Remove(dst)
		return err
	}
	return f.Close()
}

// dailyDue reports whether no daily backup was taken today.
func dailyDue(dir string, now time.Time) bool { return backupDue(dir, now, "") }

// backupDue reports whether no backup of the schedule was taken in now's day, week or month: every is "" (each
// day), "week" or "month" (#409).
func backupDue(dir string, now time.Time, every string) bool {
	period := func(t time.Time) string {
		switch every {
		case "week":
			y, w := t.ISOWeek()
			return fmt.Sprint(y, "w", w)
		case "month":
			return t.Format("200601")
		}
		return t.Format("20060102")
	}
	p := period(now)
	for _, b := range backups(dir) {
		if b.Label == "" && period(b.At) == p {
			return false
		}
	}
	return true
}

// pruneBackups keeps the newest daily backup of each of the last backupDays days, the newest of each of
// the backupWeeks weeks before them and of the backupMonths months before those, and the newest backupPre of those
// taken before an operation or by hand.
func pruneBackups(dir string, now time.Time) {
	all := backups(dir)
	keep := map[string]bool{}
	days, weeks, months := map[string]bool{}, map[string]bool{}, map[string]bool{}
	pre := map[string]int{} // by label: the sync's backups, taken often, do not push out one before an import or by hand
	y0, m0, d0 := now.Date()
	today := time.Date(y0, m0, d0, 0, 0, 0, 0, now.Location())
	for _, b := range all { // newest first
		if b.Label != "" {
			if pre[b.Label] < backupPre {
				keep[b.Path] = true
				pre[b.Label]++
			}
			continue
		}
		by, bm, bd := b.At.Date()
		day := time.Date(by, bm, bd, 0, 0, 0, 0, now.Location())
		age := int(today.Sub(day).Hours() / 24)
		dk := day.Format("20060102")
		switch {
		case age < backupDays:
			if !days[dk] {
				days[dk], keep[b.Path] = true, true
			}
		case age < backupDays+7*backupWeeks:
			yr, wk := b.At.ISOWeek()
			k := fmt.Sprint(yr, "-", wk)
			if !weeks[k] {
				weeks[k], keep[b.Path] = true, true
			}
		case age < backupDays+7*backupWeeks+31*backupMonths:
			if k := b.At.Format("200601"); !months[k] {
				months[k], keep[b.Path] = true, true
			}
		}
	}
	for _, b := range all {
		if !keep[b.Path] {
			os.Remove(b.Path)
		}
	}
}

// restoreMark is the file that says a backup is to be put in place at the next start: it holds its path.
func restoreMark(dir string) string { return filepath.Join(filepath.Dir(dir), "restore-from") }

// safeRestorePath verifies that path is an existing regular .db.gz file strictly inside the backup dir (#185).
func safeRestorePath(dir, path string) (string, error) {
	if dir == "" || strings.TrimSpace(path) == "" {
		return "", errors.New(locales.BackupErrOpen())
	}
	src := filepath.Clean(strings.TrimSpace(path))
	info, err := os.Lstat(src)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New(locales.BackupErrOpen())
	}
	dirAbs, err := filepath.EvalSymlinks(dir)
	if err != nil {
		dirAbs, err = filepath.Abs(dir)
		if err != nil {
			return "", err
		}
	}
	srcAbs, err := filepath.EvalSymlinks(src)
	if err != nil {
		return "", err
	}
	if filepath.Clean(srcAbs) == filepath.Clean(dirAbs) || !Within(dirAbs, srcAbs) || !strings.HasSuffix(srcAbs, backupExt) {
		return "", errors.New(locales.BackupErrOpen())
	}
	return srcAbs, nil
}

// planRestore puts the restore of backup path away for the next start.
func planRestore(dir, path string) error {
	safe, err := safeRestorePath(dir, path)
	if err != nil {
		return err
	}
	hardenPrivateDir(dir)
	return os.WriteFile(restoreMark(dir), []byte(safe), 0o600)
}

// applyRestore, before the library is opened, puts the planned backup in the place of the library's
// file. The file it replaces is kept beside it, so that nothing is lost by a wrong choice. It says what it
// did, or "" when nothing was planned.
func applyRestore(dir, file string, now time.Time) (string, error) {
	mark := restoreMark(dir)
	b, err := os.ReadFile(mark)
	if err != nil {
		return "", nil
	}
	os.Remove(mark)
	src, err := safeRestorePath(dir, string(b))
	if err != nil {
		return "", fmt.Errorf("%s: %w", locales.BackupErrOpen(), err)
	}
	tmp := file + ".restoring"
	if err := gunzipFile(src, tmp); err != nil {
		return "", fmt.Errorf("%s: %w", locales.BackupErrOpen(), err)
	}
	test, err := OpenStore(tmp) // a file that does not open is not put in place
	if err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("%s: %w", locales.BackupErrNotLibrary(), err)
	}
	test.Close()
	kept := ""
	if _, err := os.Stat(file); err == nil {
		kept = file + ".onceki-" + now.Format("20060102-150405")
		if err := os.Rename(file, kept); err != nil {
			os.Remove(tmp)
			return "", err
		}
	}
	// the file's journals go with it, not onto the restored one: in WAL what was last written is in -wal (#136)
	for _, ext := range []string{"-wal", "-shm", "-journal"} {
		if kept != "" {
			os.Rename(file+ext, kept+ext)
		}
		os.Remove(file + ext)
	}
	if err := os.Rename(tmp, file); err != nil {
		return "", err
	}
	return locales.BackupRestored(filepath.Base(src), filepath.Base(kept)), nil
}

// zipDir writes the folder as a zip.
func zipDir(src, dst string) error {
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	base := filepath.Base(src)
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		w, err := zw.Create(filepath.ToSlash(filepath.Join(base, rel)))
		if err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(w, in)
		return err
	})
	if cerr := zw.Close(); err == nil {
		err = cerr
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// ---- the app's side ----

// store is the library's file, or nil for a library of plain files.
func (a *app) store() *Store {
	s, _ := a.lib.FS.(*Store)
	return s
}

// backupDirOf is where this app's backups are: the config folder's, unless a test has its own.
func (a *app) backupDirOf() string {
	if a.backupDir != "" {
		return a.backupDir
	}
	return backupDir()
}

// backupNow takes a backup in the background and says it did; label says what it is for.
func (a *app) backupNow(label string, quiet bool) {
	s := a.store()
	dir := a.backupDirOf()
	if s == nil || dir == "" {
		if !quiet {
			a.say(toastInfo, locales.ThisLibraryIsMadeOfPlain())
		}
		return
	}
	a.flush()
	if a.backupBusy.Swap(true) {
		return
	}
	go func() {
		defer a.guard()
		defer a.backupBusy.Store(false)
		path, err := backupTo(s, dir, label, time.Now())
		pruneBackups(dir, time.Now())
		a.do(func() error {
			switch {
			case err != nil:
				a.say(toastErr, locales.BackupBackupFailed(err.Error()))
			case !quiet:
				a.say(toastOK, locales.BackupBackedUp(filepath.Base(path)))
			}
			return nil
		})
	}()
}

// backupDaily takes the backup of the schedule - the day's, the week's or the month's - if none was, a moment after
// the start.
func (a *app) backupDaily() {
	if a.settings.NoBackup || a.store() == nil {
		return
	}
	if backupDue(a.backupDirOf(), time.Now(), a.settings.BackupEvery) {
		a.backupNow("", true)
	}
}

// backupBefore takes a backup before an operation that changes much, and waits for it: the operation
// comes after.
func (a *app) backupBefore(what string) {
	s := a.store()
	dir := a.backupDirOf()
	if s == nil || dir == "" || a.settings.NoBackup {
		return
	}
	a.flush()
	if _, err := backupTo(s, dir, what, time.Now()); err == nil {
		pruneBackups(dir, time.Now())
	}
}

// restoreFrom plans the restore of a backup, and says what will happen.
func (a *app) restoreFrom(b Backup) {
	if err := planRestore(a.backupDirOf(), b.Path); err != nil {
		a.say(toastErr, locales.BackupRestoreFailed(err.Error()))
		return
	}
	a.say(toastInfo, locales.BackupRestoreOnRestart(dateTime(b.At, false)))
}

// exportZip writes the library as Markdown files and zips them, next to its file.
func (a *app) exportZip(store *Store) (string, bool) {
	dir := filepath.Join(filepath.Dir(store.File), "kitaplik-md-"+time.Now().Format("20060102-150405"))
	a.flush()
	if err := a.lib.ExportMarkdown(dir); err != nil {
		os.RemoveAll(dir)
		return locales.CommonDidntWork(err.Error()), false
	}
	zipPath := dir + ".zip"
	err := zipDir(dir, zipPath)
	os.RemoveAll(dir)
	if err != nil {
		os.Remove(zipPath)
		return locales.CommonDidntWork(err.Error()), false
	}
	return locales.CommonSavedTo(zipPath), true
}

// backupUI is the backups' settings as they are used: when the last one by hand was taken, and the one whose delete was
// asked, for a moment.
type backupUI struct {
	handAt time.Time
	del    string
	delAt  time.Time
}

// backupRest is how long Back up now rests after a backup: one a second is no use (#409).
const backupRest = 60 * time.Second

// backupConfirm is how long a delete asked waits for its second click.
const backupConfirm = 4 * time.Second

// settingsBackup is the settings' group of the backups: on or off and how often, the folder and a backup by hand,
// and the list, each to look at, restore or delete.
func (a *app) settingsBackup(c *ui.Context) {
	cl := ryugo.From(c).Colors
	dir := a.backupDirOf()
	on := !a.settings.NoBackup
	every := a.settings.BackupEvery
	list := backups(dir)
	now := c.Now()
	settingGroup(c, "set-backup",
		func() {
			settingRow(c, locales.CommonDailyBackup(), locales.BackupACompressedCopyLibrary(), func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					if on {
						opts := []ryugo.Option[string]{{Value: "", Label: locales.BackupEveryDay()}, {Value: "week", Label: locales.BackupEveryWeek()},
							{Value: "month", Label: locales.BackupEveryMonth()}}
						ryugo.Select(c.Key("set-backup-every"), &every, opts, ryugo.SelectOptions{Label: locales.BackupEvery(), Size: ryugo.Sm}).Width(140)
					}
					ryugo.Switch(c.Key("set-backup-on"), &on, ryugo.SwitchOptions{}).Label(locales.CommonDailyBackup())
				})
			})
		},
		func() {
			settingRow(c, locales.BackupBackupFolder(), dir, func() {
				rest := a.bk.handAt.Add(backupRest).Sub(now)
				b := ryugo.Button(c.Key("backup-now"), locales.CommonBackUpNow(), ryugo.ButtonOptions{Size: ryugo.Sm})
				if rest > 0 || a.backupBusy.Load() { // resting after one: it comes back by itself
					b.Disabled(true)
					c.After(max(rest, 100*time.Millisecond))
				} else if b.Clicked() {
					a.bk.handAt = now
					a.backupNow("elle", false)
				}
			})
		})
	if on != !a.settings.NoBackup {
		a.settings.NoBackup = !on
		a.saveSettings()
	}
	if every != a.settings.BackupEvery {
		a.settings.BackupEvery = every
		a.saveSettings()
	}
	if len(list) == 0 {
		return
	}
	if a.bk.del != "" && now.Sub(a.bk.delAt) >= backupConfirm { // not clicked again: not deleted
		a.bk.del = ""
	}
	rows := []func(){}
	for i, b := range list {
		if i >= 12 {
			break
		}
		rows = append(rows, func() {
			what := locales.CommonDaily()
			if b.Label != "" {
				what = backupKind(b.Label) // what it was taken for, as the file names it: elle, içe-aktarma
			}
			settingRow(c, dateTime(b.At, true), what+" · "+fmt.Sprintf("%.1f MB", float64(b.Size)/1e6), func() {
				ui.Row(c).Gap(6).Children(func() {
					if ryugo.Button(c.Key("preview:"+b.Path), locales.BackupPreview(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						a.previewBackup(b)
					}
					if ryugo.Button(c.Key("restore:"+b.Path), locales.CommonRestore(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						a.restoreFrom(b)
					}
					asked := a.bk.del == b.Path
					label, opts := locales.CommonDelete(), ryugo.ButtonOptions{Size: ryugo.Sm}
					if asked { // the second click deletes: a file gone is gone
						label, opts.Color, opts.Variant = locales.BackupDeleteSure(), cl.Up, ryugo.Solid
						c.After(a.bk.delAt.Add(backupConfirm).Sub(now))
					}
					if ryugo.Button(c.Key("delete:"+b.Path), label, opts).Clicked() {
						if asked {
							a.deleteBackup(b)
						} else {
							a.bk.del, a.bk.delAt = b.Path, now
						}
					}
				})
			})
		})
	}
	ui.Text(c, locales.BackupBackups()).FontSize(13).FontWeight(620).TextColor(cl.Muted)
	settingGroup(c, "set-backups", rows...)
}

// deleteBackup removes a backup's file, and says so.
func (a *app) deleteBackup(b Backup) {
	a.bk.del = ""
	if err := os.Remove(b.Path); err != nil {
		a.say(toastErr, locales.CommonDidntWork(err.Error()))
		return
	}
	a.say(toastOK, locales.BackupDeleted(dateTime(b.At, false)))
}

// backupKind is what a backup was taken for, as its file's name says it (backupNow, backupBefore: elle,
// içe-aktarma, güncelleme, senkron), in the window's language; a backup taken each day has none.
func backupKind(label string) string {
	switch strings.ReplaceAll(label, "-", " ") {
	case "", "günlük":
		return locales.BackupKindDaily()
	case "elle":
		return msg("byHand")
	case "içe aktarma":
		return msg("beforeAnImport")
	case "güncelleme":
		return msg("beforeAnUpdate")
	case "geri yükleme":
		return msg("beforeARestore")
	case "senkron":
		return locales.BackupKindSync()
	}
	return label
}
