package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

// settingsAt opens the settings at the section named name.
func settingsAt(t *testing.T, a *app, name string) {
	t.Helper()
	a.openSettings()
	for i, s := range settingsSections {
		if s.name == name {
			a.settingsTab = i
			return
		}
	}
	t.Fatalf("no section %q", name)
}

// TestSyncFolderPicker: the folder mode of the sync has a button that opens the system's folder picker (#406).
func TestSyncFolderPicker(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.settings.SyncMode = "dir"
	settingsAt(t, a, "sync4")
	tt.Frame()
	tt.Frame()
	if !tt.HasText(locales.BackupChooseFolder()) {
		t.Fatalf("no picker: %v", tt.Texts())
	}
}

// TestBackupsInSettings: a backup in the list can be looked at and deleted, the delete on a second click; Back up now
// rests after a backup; and a backup opens as a library to look at (#409).
func TestBackupsInSettings(t *testing.T) {
	st := testStore(t)
	lib, _ := OpenLibraryIn(st, StoreRoot)
	a := newApp(lib)
	a.backupDir = filepath.Join(t.TempDir(), "b")
	tt := ui.NewTester(a.view, 900, 1400)
	a.backupBefore("x")
	list := backups(a.backupDir)
	if len(list) != 1 {
		t.Fatalf("%d backups", len(list))
	}
	v, err := openBackupView(list[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(v.lib.Collections) == 0 {
		t.Fatal("the backup opened with no collections")
	}
	dir := v.dir
	v.close()
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("its folder stays after it closed")
	}

	settingsAt(t, a, "common_library")
	tt.Frame()
	tt.Frame()
	if !tt.HasText(locales.BackupPreview()) {
		t.Fatalf("no preview: %v", tt.Texts())
	}
	if err := tt.Click(locales.CommonDelete()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, err := os.Stat(list[0].Path); err != nil || !tt.HasText(locales.BackupDeleteSure()) {
		t.Fatalf("deleted at the first click, or not asked: %v", tt.Texts())
	}
	if err := tt.Click(locales.BackupDeleteSure()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, err := os.Stat(list[0].Path); err == nil {
		t.Fatal("not deleted at the second click")
	}

	if err := tt.Click(locales.CommonBackUpNow()); err != nil {
		t.Fatal(err)
	}
	at := a.bk.handAt
	for i := 0; i < 200 && a.backupBusy.Load(); i++ {
		time.Sleep(10 * time.Millisecond)
	}
	tt.Frame()
	tt.Click(locales.CommonBackUpNow()) // resting: nothing
	tt.Frame()
	if at.IsZero() || a.bk.handAt != at {
		t.Fatalf("the button did not rest: %v %v", at, a.bk.handAt)
	}
}

// TestBackupViewSearchAndRestore: the backup's window finds pages by the words of their text, shows the ones picked
// alone, and restores them into the library, into the collection of their name (#496).
func TestBackupViewSearchAndRestore(t *testing.T) {
	st := testStore(t)
	lib, _ := OpenLibraryIn(st, StoreRoot)
	coll := lib.Collections[0]
	other, _ := coll.NewPage(nil, "İkinci")
	other.SaveText("baharat her şeydir\n")
	a := newApp(lib)
	a.backupDir = filepath.Join(t.TempDir(), "b")
	a.backupBefore("x")
	v, err := openBackupView(backups(a.backupDir)[0])
	if err != nil {
		t.Fatal(err)
	}
	defer v.close()
	v.app = a
	tt := ui.NewTester(v.view, 1000, 700)
	tt.Frame()
	v.query = "baharat"
	tt.Frame()
	row := func(title string) bool { return tt.HasText(locales.BackupPickPage(title)) } // the list's, not the page shown
	if !row("İkinci") || row("Birinci") {
		t.Fatalf("the search by the text: %v", tt.Texts())
	}
	if err := tt.Click(locales.BackupPickPage("İkinci")); err != nil {
		t.Fatal(err)
	}
	v.query, v.only = "", true
	tt.Frame()
	if !row("İkinci") || row("Birinci") {
		t.Fatalf("the picked alone: %v", tt.Texts())
	}
	if err := tt.Click(locales.BackupRestorePages(1)); err != nil {
		t.Fatal(err, tt.Texts())
	}
	n := 0
	for _, p := range coll.All() {
		if p.Title == "İkinci" {
			if body, _ := p.Text(); body == "baharat her şeydir\n" {
				n++
			}
		}
	}
	if n != 2 || len(lib.Collections) != 1 {
		t.Fatalf("restored: %d copies with the text, %d collections", n, len(lib.Collections))
	}
}
