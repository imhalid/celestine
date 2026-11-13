package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/locales"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	lib, err := OpenLibraryIn(st, StoreRoot)
	if err != nil {
		t.Fatal(err)
	}
	c, err := lib.NewCollection("Not")
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.NewPage(nil, "Birinci")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.SaveText("yedeklenecek metin\n"); err != nil {
		t.Fatal(err)
	}
	return st
}

// A backup is a consistent copy, compressed, that opens as a library.
func TestBackupSnapshotOpens(t *testing.T) {
	st := testStore(t)
	dir := t.TempDir()
	now := time.Date(2026, 11, 7, 9, 0, 0, 0, time.Local)
	path, err := backupTo(st, dir, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "kitaplik-20261107-090000.db.gz") {
		t.Fatalf("name %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".snapshot*")); len(left) != 0 {
		t.Fatalf("snapshot left: %v", left)
	}
	raw := filepath.Join(t.TempDir(), "r.db")
	if err := gunzipFile(path, raw); err != nil {
		t.Fatal(err)
	}
	st2, err := OpenStore(raw)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	lib, err := OpenLibraryIn(st2, StoreRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Collections) != 1 || len(lib.Collections[0].All()) != 1 {
		t.Fatalf("the backup is not the library: %d collections", len(lib.Collections))
	}
	body, _ := lib.Collections[0].All()[0].Body()
	if !strings.Contains(body, "yedeklenecek") {
		t.Fatalf("body %q", body)
	}
}

func touchBackup(t *testing.T, dir string, at time.Time, label string) string {
	t.Helper()
	name := "kitaplik-" + at.Format("20060102-150405")
	if label != "" {
		name += "-" + label
	}
	p := filepath.Join(dir, name+backupExt)
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Seven days and four weeks are kept, the newest of each; those taken before an operation are the last five.
func TestBackupPrune(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.Local)
	for d := 0; d < 60; d++ { // a backup every day for sixty days, and a second one on some of them
		touchBackup(t, dir, now.AddDate(0, 0, -d).Add(-time.Hour), "")
		if d%3 == 0 {
			touchBackup(t, dir, now.AddDate(0, 0, -d).Add(-30*time.Minute), "")
		}
	}
	for i := 0; i < 9; i++ {
		touchBackup(t, dir, now.Add(-time.Duration(i)*time.Hour*5), "içe-aktarma")
	}
	pruneBackups(dir, now)
	all := backups(dir)
	daily, pre := 0, 0
	days := map[string]int{}
	var oldest time.Time
	for _, b := range all {
		if b.Label != "" {
			pre++
			continue
		}
		daily++
		days[b.At.Format("20060102")]++
		oldest = b.At
	}
	if pre != backupPre {
		t.Fatalf("%d operation backups kept", pre)
	}
	for d, n := range days {
		if n != 1 {
			t.Errorf("%d backups of %s", n, d)
		}
	}
	if daily < backupDays || daily > backupDays+backupWeeks+backupMonths+1 {
		t.Fatalf("%d daily backups kept", daily)
	}
	if now.Sub(oldest) > time.Duration(backupDays+7*backupWeeks+31*backupMonths)*24*time.Hour {
		t.Fatalf("too old: %v", oldest)
	}
	months := map[string]int{} // before the weeks, one a month
	for _, b := range all {
		if b.Label == "" && now.Sub(b.At) > time.Duration(backupDays+7*backupWeeks)*24*time.Hour {
			months[b.At.Format("200601")]++
		}
	}
	for m, n := range months {
		if n != 1 {
			t.Errorf("%d backups of the month %s", n, m)
		}
	}
	if len(months) == 0 {
		t.Error("no monthly backup kept before the weeks")
	}
	// the last seven days are all there
	for d := 0; d < backupDays; d++ {
		if days[now.AddDate(0, 0, -d).Add(-time.Hour).Format("20060102")] != 1 {
			t.Errorf("day %d is gone", d)
		}
	}
	// of two on a day, the newer stays
	for _, b := range all {
		if b.At.Format("20060102") == now.Format("20060102") && b.Label == "" && b.At.Minute() != 30 {
			t.Errorf("not the newer of the day: %v", b.At)
		}
	}
}

func TestBackupDailyDue(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.Local)
	if !dailyDue(dir, now) {
		t.Fatal("none, and not due")
	}
	touchBackup(t, dir, now.Add(-2*time.Hour), "elle") // by hand: not the day's
	if !dailyDue(dir, now) {
		t.Fatal("a backup by hand counted as the day's")
	}
	touchBackup(t, dir, now.Add(-time.Hour), "")
	if dailyDue(dir, now) {
		t.Fatal("due, with the day's taken")
	}
	if !dailyDue(dir, now.AddDate(0, 0, 1)) {
		t.Fatal("not due tomorrow")
	}
}

// TestBackupDue: a weekly backup is due once a week, a monthly once a month (#409).
func TestBackupDue(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.Local)                 // a Wednesday
	touchBackup(t, dir, time.Date(2026, 11, 9, 9, 0, 0, 0, time.Local), "") // Monday of the same week
	if backupDue(dir, now, "week") || !backupDue(dir, now, "") {
		t.Fatal("the week's is taken, the day's is not")
	}
	if !backupDue(dir, now.AddDate(0, 0, 7), "week") {
		t.Fatal("not due the next week")
	}
	if backupDue(dir, now, "month") || !backupDue(dir, time.Date(2026, 12, 1, 9, 0, 0, 0, time.Local), "month") {
		t.Fatal("the month's")
	}
}

// A restore planned is made at the next start, the file it replaces is kept, and what is not a library is refused.
func TestBackupRestore(t *testing.T) {
	st := testStore(t)
	cfg := filepath.Join(t.TempDir(), "backups")
	path, err := backupTo(st, cfg, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// the library changes after the backup
	lib, _ := OpenLibraryIn(st, StoreRoot)
	lib.Collections[0].All()[0].SaveText("sonradan yazılan\n")
	file := st.File
	st.Close()

	if msg, err := applyRestore(cfg, file, time.Now()); msg != "" || err != nil {
		t.Fatalf("nothing planned: %q %v", msg, err)
	}
	if err := planRestore(cfg, path); err != nil {
		t.Fatal(err)
	}
	msg, err := applyRestore(cfg, file, time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC))
	if err != nil || !strings.Contains(msg, "geri yüklendi") {
		t.Fatalf("%q %v", msg, err)
	}
	if _, err := os.Stat(restoreMark(cfg)); err == nil {
		t.Fatal("the plan is still there: it would be made again")
	}
	st2, err := OpenStore(file)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	lib2, _ := OpenLibraryIn(st2, StoreRoot)
	body, _ := lib2.Collections[0].All()[0].Body()
	if !strings.Contains(body, "yedeklenecek") || strings.Contains(body, "sonradan") {
		t.Fatalf("not the backup's: %q", body)
	}
	kept, _ := filepath.Glob(file + ".onceki-*")
	if len(kept) != 1 {
		t.Fatalf("the replaced file not kept: %v", kept)
	}
	st3, err := OpenStore(kept[0])
	if err != nil {
		t.Fatal(err)
	}
	defer st3.Close()
	lib3, _ := OpenLibraryIn(st3, StoreRoot)
	if b, _ := lib3.Collections[0].All()[0].Body(); !strings.Contains(b, "sonradan") {
		t.Fatalf("the kept file is not the old library: %q", b)
	}
	// a file that is not a library is refused, and the library stays
	bad := filepath.Join(cfg, "kitaplik-20260101-000000.db.gz")
	os.WriteFile(bad, []byte("hiç gzip değil"), 0o644)
	planRestore(cfg, bad)
	before, _ := os.ReadFile(file)
	if _, err := applyRestore(cfg, file, time.Now()); err == nil {
		t.Fatal("a broken backup was put in place")
	}
	after, _ := os.ReadFile(file)
	if string(before) != string(after) {
		t.Fatal("the library changed")
	}
}

// The app: a backup by hand, the daily one once, one before an import, and the notice that says so.
func TestAppBackup(t *testing.T) {
	st := testStore(t)
	lib, _ := OpenLibraryIn(st, StoreRoot)
	a := newApp(lib)
	a.backupDir = filepath.Join(t.TempDir(), "b")
	a.backupBefore("içe aktarma")
	l := backups(a.backupDir)
	if len(l) != 1 || l[0].Label != "içe-aktarma" {
		t.Fatalf("before an import: %+v", l)
	}
	a.settings.NoBackup = true
	a.backupBefore("x")
	if len(backups(a.backupDir)) != 1 {
		t.Fatal("a backup taken with them off")
	}
	a.settings.NoBackup = false
	a.backupDaily()
	for i := 0; i < 200 && a.backupBusy.Load(); i++ {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	daily := 0
	for _, b := range backups(a.backupDir) {
		if b.Label == "" {
			daily++
		}
	}
	if daily != 1 {
		t.Fatalf("%d daily backups", daily)
	}
	a.backupDaily() // the day's is taken
	time.Sleep(100 * time.Millisecond)
	if n := len(backups(a.backupDir)); n != 2 {
		t.Fatalf("a second daily: %d backups", n)
	}
	// the palette: restore lines, naming the backups
	n := 0
	for _, it := range a.commands() {
		if strings.HasPrefix(it.title, "Yedekten geri yükle: ") {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("%d restore commands", n)
	}
}

func TestExportZip(t *testing.T) {
	st := testStore(t)
	lib, _ := OpenLibraryIn(st, StoreRoot)
	a := newApp(lib)
	msg, ok := a.exportZip(st)
	if !ok {
		t.Fatal(msg)
	}
	zp := strings.TrimPrefix(msg, locales.CommonSavedTo(""))
	defer os.Remove(zp)
	fi, err := os.Stat(zp)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("zip: %v", err)
	}
	if left, _ := filepath.Glob(strings.TrimSuffix(zp, ".zip")); len(left) != 0 {
		t.Fatal("the folder is left")
	}
}

// Each label keeps its own last backups: the sync's, taken often, do not push out the one taken before an import
// (#205).
func TestBackupPruneByLabel(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.Local)
	touchBackup(t, dir, now.Add(-48*time.Hour), "içe-aktarma")
	for i := 0; i < 12; i++ {
		touchBackup(t, dir, now.Add(-time.Duration(i)*time.Minute), "senkron")
	}
	pruneBackups(dir, now)
	by := map[string]int{}
	for _, b := range backups(dir) {
		by[b.Label]++
	}
	if by["içe-aktarma"] != 1 || by["senkron"] != backupPre {
		t.Fatalf("kept: %v", by)
	}
}

func TestBackupAndMarkPrivatePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	st := testStore(t)
	cfg := filepath.Join(t.TempDir(), "celestine")
	bdir := filepath.Join(cfg, "backups")
	if err := os.MkdirAll(bdir, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(cfg, 0o755)
	_ = os.Chmod(bdir, 0o755)
	legacy := filepath.Join(bdir, "kitaplik-20260101-010101.db.gz")
	if err := os.WriteFile(legacy, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(legacy, 0o644)

	// Listing backups must not chmod folders or files (#179).
	if got := backups(bdir); len(got) != 1 {
		t.Fatalf("backups = %d, want 1", len(got))
	}
	for _, dir := range []string{cfg, bdir} {
		if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o755 {
			t.Fatalf("backups() changed %s mode = %v, err = %v", dir, info.Mode().Perm(), err)
		}
	}
	if info, err := os.Stat(legacy); err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("backups() changed legacy mode = %v, err = %v", info.Mode().Perm(), err)
	}

	out, err := backupTo(st, bdir, "", time.Date(2026, 11, 7, 9, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{cfg, bdir} {
		if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("%s mode = %v, err = %v", dir, info.Mode().Perm(), err)
		}
	}
	for _, f := range []string{legacy, out} {
		if info, err := os.Stat(f); err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %v, err = %v", f, info.Mode().Perm(), err)
		}
	}
	mark := filepath.Join(t.TempDir(), "update-pending")
	_ = os.WriteFile(mark, []byte("1.0.0\n/new\n/old\n"), 0o644)
	rollbackIfBroken(mark, "1.0.0")
	if info, err := os.Stat(mark); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("update mark mode = %v, err = %v", info.Mode().Perm(), err)
	}
}

func TestRestorePathValidation(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "backups")
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.db.gz")
	if err := os.WriteFile(outside, []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := planRestore(cfg, outside); err == nil {
		t.Fatal("planRestore accepted path outside backup dir")
	}
	target := filepath.Join(t.TempDir(), "kitaplik.db")
	for _, forged := range []string{outside, filepath.Join("..", filepath.Base(outside))} {
		_ = os.WriteFile(restoreMark(cfg), []byte(forged), 0o600)
		if _, err := applyRestore(cfg, target, time.Now()); err == nil {
			t.Fatalf("applyRestore accepted forged path %q", forged)
		}
	}
	link := filepath.Join(cfg, "kitaplik-20260101-010101.db.gz")
	if err := os.Symlink(outside, link); err == nil {
		_ = os.WriteFile(restoreMark(cfg), []byte(link), 0o600)
		if _, err := applyRestore(cfg, target, time.Now()); err == nil {
			t.Fatal("applyRestore followed symlink inside backup dir")
		}
	}
}
