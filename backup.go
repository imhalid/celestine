package main

import (
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Backups of the library's file. Once a day, at the first start, a snapshot of it - a consistent copy,
// made while the app goes on - is written, compressed, to the config folder; the last seven days and
// four weeks are kept. One is taken before the big operations too (an import, an update, a restore),
// and by hand. A restore is put away and made at the next start, with the file it replaces kept beside.

const (
	backupDays  = 7 // days of backups kept, the newest of each
	backupWeeks = 4 // and weeks before, the newest of each
	backupPre   = 5 // the ones taken before an operation, newest kept
	backupExt   = ".db.gz"
)

// backupDir is where the backups are written.
func backupDir() string {
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

// backups lists the folder's backups, newest first.
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
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
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
	f, err := os.Create(dst + ".tmp")
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
	f, err := os.Create(dst)
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
func dailyDue(dir string, now time.Time) bool {
	y, m, d := now.Date()
	for _, b := range backups(dir) {
		if b.Label == "" {
			by, bm, bd := b.At.Date()
			if by == y && bm == m && bd == d {
				return false
			}
		}
	}
	return true
}

// pruneBackups keeps the newest daily backup of each of the last backupDays days, the newest of each of
// the backupWeeks weeks before them, and the newest backupPre of those taken before an operation or by hand.
func pruneBackups(dir string, now time.Time) {
	all := backups(dir)
	keep := map[string]bool{}
	days, weeks := map[string]bool{}, map[string]bool{}
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

// planRestore puts the restore of backup path away for the next start.
func planRestore(dir, path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return os.WriteFile(restoreMark(dir), []byte(path), 0o644)
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
	src := strings.TrimSpace(string(b))
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
	f, err := os.Create(dst)
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
			a.notice = "Bu kitaplık düz dosyalardan oluşuyor: yedek için klasörü kopyalaman yeter."
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
				a.notice = "Yedek alınamadı: " + err.Error()
			case !quiet:
				a.notice = "Yedeklendi: " + filepath.Base(path)
			}
			return nil
		})
	}()
}

// backupDaily takes the day's backup, if none was, a moment after the start.
func (a *app) backupDaily() {
	if a.settings.NoBackup || a.store() == nil {
		return
	}
	if dailyDue(a.backupDirOf(), time.Now()) {
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
		a.notice = "Geri yüklenemedi: " + err.Error()
		return
	}
	a.say(toastInfo, locales.BackupRestoreOnRestart(dateTime(b.At, false)))
}

// exportZip writes the library as Markdown files and zips them, next to its file.
func (a *app) exportZip(store *Store) string {
	dir := filepath.Join(filepath.Dir(store.File), "kitaplik-md-"+time.Now().Format("20060102-150405"))
	a.flush()
	if err := a.lib.ExportMarkdown(dir); err != nil {
		os.RemoveAll(dir)
		return "Olmadı: " + err.Error()
	}
	zipPath := dir + ".zip"
	err := zipDir(dir, zipPath)
	os.RemoveAll(dir)
	if err != nil {
		os.Remove(zipPath)
		return "Olmadı: " + err.Error()
	}
	return "Yazıldı: " + zipPath
}

// settingsBackup is the settings' group of the backups.
func (a *app) settingsBackup(c *ui.Context) {
	cl := ryugo.From(c).Colors
	dir := a.backupDirOf()
	on := !a.settings.NoBackup
	list := backups(dir)
	settingGroup(c, "set-backup",
		func() {
			settingRow(c, locales.CommonDailyBackup(), locales.BackupACompressedCopyLibrary(), func() {
				ryugo.Switch(c.Key("set-backup-on"), &on, ryugo.SwitchOptions{}).Label(locales.CommonDailyBackup())
			})
		},
		func() {
			settingRow(c, locales.BackupBackupFolder(), dir, func() {
				if ryugo.Button(c.Key("backup-now"), locales.CommonBackUpNow(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.backupNow("elle", false)
				}
			})
		})
	if on != !a.settings.NoBackup {
		a.settings.NoBackup = !on
		a.saveSettings()
	}
	if len(list) == 0 {
		return
	}
	rows := []func(){}
	for i, b := range list {
		if i >= 12 {
			break
		}
		b := b
		rows = append(rows, func() {
			what := locales.CommonDaily()
			if b.Label != "" {
				what = T(strings.ReplaceAll(b.Label, "-", " ")) // what it was taken for, as the file names it: elle, içe-aktarma
			}
			settingRow(c, dateTime(b.At, true), what+" · "+fmt.Sprintf("%.1f MB", float64(b.Size)/1e6), func() {
				if ryugo.Button(c.Key("restore:"+b.Path), locales.CommonRestore(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.restoreFrom(b)
				}
			})
		})
	}
	ui.Text(c, locales.BackupBackups()).FontSize(13).FontWeight(620).TextColor(cl.Muted)
	settingGroup(c, "set-backups", rows...)
}
