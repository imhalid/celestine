package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The app's side of synchronization: the settings, the remote they make, the state kept in the library's
// file, a sync now and then in the background, the badge of its state, the conflicts to choose between.

const syncStateKey = "sync-state"

type syncUI struct {
	busy     bool
	status   string    // "eşitleniyor…", "Eşit 14:32", "çevrimdışı: …", "çakışma: 2"
	last     time.Time // when the last pass began
	at       time.Time // when the last one went well
	result   string
	err      string
	conflict int
	// the conflict files as last looked for, and when; zero: look again (conflictFiles)
	conflicts      []string
	conflictsAt    time.Time
	conflictsBusy  bool
	conflictsAgain bool
	tried          time.Time
	pass           string // the passphrase being typed
	recovery       string // the recovery key just made, to be written down
	hasKey         bool
}

// syncRemote makes the remote the settings describe.
func (a *app) syncRemote() (Remote, error) {
	var r Remote
	switch a.settings.SyncMode {
	case "dir":
		if strings.TrimSpace(a.settings.SyncDir) == "" {
			return nil, errors.New("klasör seçilmemiş")
		}
		r = dirRemote{a.settings.SyncDir}
	case "server":
		if a.settings.SyncURL == "" || a.settings.SyncToken == "" {
			return nil, errors.New("sunucu adresi ya da jeton yok")
		}
		r = newHTTPRemote(a.settings.SyncURL, a.settings.SyncToken)
	default:
		return nil, errors.New("senkron kapalı")
	}
	// end-to-end encryption is not a choice: nothing leaves this computer before a passphrase has made the key
	if a.settings.SyncKey == "" {
		return nil, errors.New("önce bir parola yaz: eşitleme şifresiz çalışmaz")
	}
	key, err := base64.StdEncoding.DecodeString(a.settings.SyncKey)
	if err != nil {
		return nil, errors.New("şifreleme anahtarı bozuk")
	}
	return newEncRemote(r, key)
}

func (a *app) syncEvery() time.Duration {
	if m := a.settings.SyncMinutes; m >= 1 && m <= 1440 {
		return time.Duration(m) * time.Minute
	}
	return 5 * time.Minute
}

func (a *app) loadSyncState() *syncState {
	st := &syncState{}
	if a.settingsStore != nil {
		if b := a.settingsStore.Meta(syncStateKey); b != nil {
			json.Unmarshal(b, st)
		}
	}
	return st
}

func (a *app) saveSyncState(st *syncState) {
	if a.settingsStore != nil {
		b, _ := json.Marshal(st)
		a.settingsStore.SetMeta(syncStateKey, b)
	}
}

// syncNow makes a pass in the background: what is typed is saved first, the library is read again if
// something came, and the state is told in the badge.
func (a *app) syncNow() {
	if a.sy.busy || a.settings.SyncMode == "" {
		return
	}
	st := a.store()
	if st == nil {
		a.sy.err = "senkron, tek dosyalı kitaplıkta çalışır"
		return
	}
	remote, err := a.syncRemote()
	if err != nil {
		a.sy.err, a.sy.status = err.Error(), "senkron ayarlı değil"
		return
	}
	a.flush()
	a.backupBefore("senkron")
	a.sy.busy, a.sy.status, a.sy.last = true, "eşitleniyor…", a.clock()
	skip := map[string]bool{}
	for _, d := range a.settings.SyncOff {
		skip[d] = true
	}
	state := a.loadSyncState()
	go func() {
		defer a.guard()
		res, err := SyncOnce(st, storeRoot, remote, state, skip, time.Now())
		a.do(func() error {
			a.sy.busy = false
			a.saveSyncState(state)
			switch {
			case err != nil:
				a.sy.err, a.sy.status = err.Error(), "çevrimdışı"
			default:
				a.sy.err, a.sy.at, a.sy.result = "", time.Now(), res.String()
				a.sy.status = "Eşit " + a.sy.at.Format("15:04")
				if len(res.Errors) > 0 {
					a.sy.err = res.Errors[0]
				}
				a.lookForConflicts()            // the badge says "çakışma: n" when it is done
				if res.Pulled+res.Removed > 0 { // something came: the window takes it, as it takes what an agent writes
					a.checkDisk()
					a.reloadLibrary()
				}
				if res.Pulled+res.Removed+res.Pushed+len(res.Conflicts) > 0 {
					a.notice = "Senkron: " + res.String()
				}
			}
			return nil
		})
	}()
}

// syncTick starts a pass when it is time: once at the start, then every few minutes.
func (a *app) syncTick(c *ui.Context) {
	if a.settings.SyncMode == "" || a.sy.busy {
		return
	}
	now := a.clock()
	if a.sy.tried.IsZero() || now.Sub(a.sy.tried) > a.syncEvery() {
		a.sy.tried = now
		a.syncNow()
	}
	c.After(30 * time.Second)
}

// conflictFiles are the files that a conflict left: "… (çakışma …)…", as last looked for. The sync settings ask
// at every frame, and going through a big library takes a good part of a second (it took 650 ms a frame, reading
// every file): so they are looked for away from the window, by name only, the first time and after a pass or a resolve.
func (a *app) conflictFiles() []string {
	if a.sy.conflictsAt.IsZero() {
		a.lookForConflicts()
	}
	return a.sy.conflicts
}

// lookForConflicts goes through the library's file names in the background; the window gets the list when it is done.
func (a *app) lookForConflicts() {
	st := a.store()
	if st == nil {
		return
	}
	if a.sy.conflictsBusy { // what it finds may be from before: it looks again when done
		a.sy.conflictsAgain = true
		return
	}
	a.sy.conflictsBusy = true
	go func() {
		defer a.guard()
		var found []string
		var walk func(dir, rel string)
		walk = func(dir, rel string) {
			ents, err := st.ReadDir(dir)
			if err != nil {
				return
			}
			for _, e := range ents {
				name := e.Name()
				if e.IsDir() {
					walk(filepath.Join(dir, name), path.Join(rel, name))
				} else if strings.Contains(name, " (çakışma ") && !strings.HasSuffix(name, ".tmp~") {
					found = append(found, path.Join(rel, name))
				}
			}
		}
		walk(storeRoot, "")
		sortStrings(found)
		a.do(func() error {
			a.sy.conflicts, a.sy.conflictsAt, a.sy.conflictsBusy = found, time.Now(), false
			a.sy.conflict = len(found)
			if a.sy.conflict > 0 && !a.sy.busy && a.sy.err == "" {
				a.sy.status = "çakışma: " + strconv.Itoa(a.sy.conflict)
			}
			if a.sy.conflictsAgain {
				a.sy.conflictsAgain = false
				a.lookForConflicts()
			}
			return nil
		})
	}()
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// mainOfConflict is the file a conflict copy is of.
func mainOfConflict(name string) string {
	i := strings.Index(name, " (çakışma ")
	j := strings.Index(name[i:], ")")
	if i < 0 || j < 0 {
		return ""
	}
	return name[:i] + name[i+j+1:]
}

// resolveConflict: use the copy (it takes the place of the main file) or throw it away; either way the copy goes.
func (a *app) resolveConflict(name string, use bool) error {
	st := a.store()
	if st == nil {
		return errors.New("tek dosyalı kitaplık değil")
	}
	abs := storeRoot + "/" + name
	if use {
		b, err := st.ReadFile(abs)
		if err != nil {
			return err
		}
		main := mainOfConflict(name)
		if main == "" {
			return errors.New("asıl dosya bulunamadı")
		}
		if err := st.WriteFile(storeRoot+"/"+main, b); err != nil {
			return err
		}
	}
	if err := st.Remove(abs); err != nil {
		return err
	}
	a.checkDisk()
	a.reloadLibrary()
	// the copy is gone from the list at once; the library is looked through again in the background
	var left []string
	for _, f := range a.sy.conflicts {
		if f != name {
			left = append(left, f)
		}
	}
	a.sy.conflicts, a.sy.conflict = left, len(left)
	a.lookForConflicts()
	return nil
}

// syncBadge is the state, in the status bar.
func (a *app) syncBadge(c *ui.Context) {
	if a.settings.SyncMode == "" || a.sy.status == "" {
		return
	}
	cl := ryugo.From(c).Colors
	col := c.Theme().TextMuted
	switch {
	case a.sy.conflict > 0:
		col = cl.Warn
	case a.sy.err != "":
		col = cl.Up
	}
	b := ui.Row(c.Key("sync-badge")).Padding(2, 6).Radius(rad(6)).Label("Senkron: " + a.sy.status).Role(ui.RoleButton).Cursor(ui.CursorPointer)
	if b.Hovered() {
		b.Background(c.Theme().SurfaceHover)
	}
	b.Children(func() { ui.Text(c, "⇅ "+a.sy.status).FontSize(11.5).TextColor(col) })
	if b.Clicked() {
		a.openSettings()
		a.settingsTab = tabSync
	}
}

// settingsSync is the settings' section of synchronization.
func (a *app) settingsSync(c *ui.Context) {
	cl := ryugo.From(c).Colors
	s := &a.settings
	mode := s.SyncMode
	modeRow := func() {
		settingRow(c, "Nereye", "Bir klasör (iCloud, Dropbox, Syncthing bırakınca onu taşır) ya da kendi sunucun (cloudflare/sync/worker.js).", func() {
			choiceBar(c, "set-sync-bar", T("Nereye"), []string{"", "dir", "server"}, []string{T("Kapalı"), T("Klasör"), T("Sunucu")}, &mode)
		})
	}
	settingGroup(c, "set-sync", modeRow)
	if mode != s.SyncMode {
		s.SyncMode = mode
		a.sy.tried = time.Time{}
		a.saveSettings()
	}
	if s.SyncMode == "" {
		ui.Text(c, T("Senkron kapalı: kitaplık yalnız bu bilgisayarda. Dosyalar düz ve taşınabilir; her gün yedeği alınır.")).FontSize(13).TextColor(cl.Muted).Wrap()
		return
	}
	if s.SyncMode == "server" {
		a.accountSection(c)
	}
	dir, url, token := s.SyncDir, s.SyncURL, s.SyncToken
	field := func(key, label string, v *string, secret bool) func() {
		return func() {
			ryugo.Field(c, ryugo.FieldOptions{Label: label}, func() {
				in := ryugo.Input(c.Key(key), v, ryugo.InputOptions{Label: label, Size: ryugo.Sm})
				_ = secret
				_ = in
			})
		}
	}
	rows := []func(){}
	if s.SyncMode == "dir" {
		rows = append(rows, field("sync-dir", "Klasör", &dir, false))
	} else {
		if s.AccountUser == "" {
			rows = append(rows, field("sync-url", "Sunucu adresi (https://…) — hesapsız, jetonla", &url, false), field("sync-token", "Bu cihazın jetonu", &token, true))
		}
	}
	if len(rows) > 0 {
		settingGroup(c, "set-sync-where", rows...)
	}
	if dir != s.SyncDir || url != s.SyncURL || token != s.SyncToken {
		s.SyncDir, s.SyncURL, s.SyncToken = dir, url, token
		a.saveSettingsSoon()
	}
	// encryption: always on; it needs a passphrase before anything is sent
	enc := s.SyncKey != ""
	if s.AccountUser == "" { // with an account the key is the account's: nothing to type
		settingGroup(c, "set-sync-enc",
			func() {
				settingRow(c, "Uçtan uca şifreleme", "Şart: dosyalar bu bilgisayarda şifrelenir, adları da gizlenir; sunucu ya da klasör yalnız anlamsız bloklar görür. Parola olmadan eşitleme başlamaz. Parolayı ve kurtarma anahtarını kaybedersen veri kurtarılamaz.", func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ryugo.Input(c.Key("sync-pass"), &a.sy.pass, ryugo.InputOptions{Label: "Parola", Size: ryugo.Sm, Placeholder: "Parola"})
						if ryugo.Button(c.Key("sync-setpass"), map[bool]string{true: "Parolayı değiştir", false: "Şifrelemeyi aç"}[enc], ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
							a.setSyncPassphrase(a.sy.pass)
						}
					})
				})
			},
			func() {
				if !enc {
					ui.Text(c, T("Parola yazılmadı: eşitleme çalışmaz.")).FontSize(12.5).TextColor(cl.Warn)
					return
				}
				ui.Text(c, T("Şifreleme açık.")).FontSize(12.5).TextColor(cl.Muted)
			})
	}
	if a.sy.recovery != "" {
		settingGroup(c, "set-sync-recovery", func() {
			settingRow(c, "Kurtarma anahtarı: bir yere yaz", a.sy.recovery, func() {
				ryugo.CopyButton(c.Key("copy-recovery"), a.sy.recovery, ryugo.CopyOptions{Label: T("Kopyala: kurtarma anahtarı")})
			})
		})
	}
	// which collections
	var rows2 []func()
	for _, co := range a.lib.Collections {
		co := co
		on := true
		for _, d := range s.SyncOff {
			if d == co.Dir {
				on = false
			}
		}
		rows2 = append(rows2, func() {
			now := on
			settingRow(c, co.Name, "", func() {
				ryugo.Switch(c.Key("sync-coll:"+co.Dir), &now, ryugo.SwitchOptions{}).Label(T("Eşitle: ") + co.Name)
			})
			if now != on {
				var out []string
				for _, d := range s.SyncOff {
					if d != co.Dir {
						out = append(out, d)
					}
				}
				if !now {
					out = append(out, co.Dir)
				}
				s.SyncOff = out
				a.saveSettings()
			}
		})
	}
	if len(rows2) > 0 {
		ui.Text(c, T("Hangi koleksiyonlar eşitlensin")).FontSize(13).FontWeight(620).TextColor(cl.Muted)
		settingGroup(c, "set-sync-colls", rows2...)
	}
	// state
	status := a.sy.status
	if status == "" {
		status = "henüz eşitlenmedi"
	}
	if a.sy.err != "" {
		status += " · " + a.sy.err
	}
	settingGroup(c, "set-sync-state", func() {
		settingRow(c, "Durum: "+status, a.sy.result, func() {
			if ryugo.Button(c.Key("sync-now"), "Şimdi eşitle", ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Soft}).Clicked() {
				a.syncNow()
			}
		})
	})
	if files := a.conflictFiles(); len(files) > 0 {
		ui.Text(c, T("Çakışmalar: iki cihazda da değişen dosyalar; ikisi de saklandı")).FontSize(13).FontWeight(620).TextColor(cl.Warn)
		var rows3 []func()
		for _, f := range files {
			f := f
			rows3 = append(rows3, func() {
				settingRow(c, mainOfConflict(f), f, func() {
					ui.Row(c).Gap(6).Children(func() {
						if ryugo.Button(c.Key("cf-use:"+f), "Bunu kullan", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
							if err := a.resolveConflict(f, true); err != nil {
								a.notice = "Olmadı: " + err.Error()
							}
						}
						if ryugo.Button(c.Key("cf-drop:"+f), "Sil", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
							if err := a.resolveConflict(f, false); err != nil {
								a.notice = "Olmadı: " + err.Error()
							}
						}
					})
				})
			})
		}
		settingGroup(c, "set-sync-conf", rows3...)
	}
}

// setSyncPassphrase turns encryption on with the passphrase: the key is made with the remote's salt, kept, and its
// recovery words shown once.
func (a *app) setSyncPassphrase(pass string) {
	if strings.TrimSpace(pass) == "" {
		a.notice = "Bir parola yaz."
		return
	}
	var inner Remote
	switch a.settings.SyncMode {
	case "dir":
		inner = dirRemote{a.settings.SyncDir}
	case "server":
		inner = newHTTPRemote(a.settings.SyncURL, a.settings.SyncToken)
	default:
		a.notice = "Önce nereye eşitleneceğini seç."
		return
	}
	e, err := openEnc(inner, pass)
	if err != nil {
		a.notice = "Olmadı: " + err.Error()
		return
	}
	a.settings.SyncKey = base64.StdEncoding.EncodeToString(e.key)
	a.sy.recovery, a.sy.pass, a.sy.hasKey = recoveryKey(e.key), "", true
	a.saveSettings()
	a.notice = "Şifreleme açık: kurtarma anahtarını bir yere yaz."
}
