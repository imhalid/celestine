package main

import (
	"archive/zip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Updating. The app asks a manifest - a small JSON that a release puts on a server of its own, Cloudflare
// in the plan (docs/19-guncelleme.md) - whether a newer version is out, and says so; with the user's yes it
// downloads the file, checks its sha256 and its ed25519 signature (the public key is in the program, the
// secret key only where the release is made), puts it in place and starts it. The old program is kept: if
// the new one does not get through its first start, it is put back. The library is never touched, and a
// backup is taken before. The check sends nothing but the request itself, and can be turned off.

// updateURL and updateKey are put in at the build of a release, as the version is:
//
//	-X main.updateURL=https://… -X main.updateKey=<base64 of the public key>
var (
	updateURL = ""
	updateKey = ""
)

// A Release is the manifest.
type Release struct {
	Version string           `json:"version"`
	Channel string           `json:"channel,omitempty"` // "" or "stable"; "beta" for a trial
	Date    string           `json:"date,omitempty"`
	Notes   string           `json:"notes,omitempty"` // Markdown
	Assets  map[string]Asset `json:"assets"`          // by "os-arch"
}

// An Asset is the file of a release for one system.
type Asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Sig    string `json:"sig"`            // base64 ed25519 signature of sigText(version, sha256)
	Size   int64  `json:"size,omitempty"` // bytes
	Kind   string `json:"kind,omitempty"` // "binary" (default), or "app-zip": a macOS .app, zipped
}

// sigText is what is signed: the version and the file's hash, so that a signature cannot be moved to
// another version.
func sigText(version, sha string) []byte { return []byte(version + "\n" + strings.ToLower(sha)) }

// SignAsset makes the signature of an asset, for the release's script.
func SignAsset(priv ed25519.PrivateKey, version, sha string) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(priv, sigText(version, sha)))
}

// verifyAsset checks a signature against a base64 public key.
func verifyAsset(pubB64, version, sha, sigB64 string) error {
	pub, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return errors.New("güncelleme anahtarı geçersiz: imzasız güncelleme kurulmaz")
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil || !ed25519.Verify(ed25519.PublicKey(pub), sigText(version, sha), sig) {
		return errors.New("imza doğrulanamadı")
	}
	return nil
}

// ---- versions ----

// parseVersion reads "v1.2.3", "1.2" or "1.2.3-beta.2": the numbers, and whether it is a trial.
func parseVersion(s string) (nums [3]int, pre string, ok bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "+"); i >= 0 {
		s = s[:i]
	}
	core, pre, _ := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) == 0 || len(parts) > 3 || core == "" {
		return nums, "", false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nums, "", false
		}
		nums[i] = n
	}
	return nums, pre, true
}

// versionLess reports whether a is older than b. A trial of a version is older than the version.
func versionLess(a, b string) bool {
	an, ap, aok := parseVersion(a)
	bn, bp, bok := parseVersion(b)
	if !aok || !bok {
		return false // "dev" is older than nothing: a build from the source does not ask to be replaced
	}
	for i := range an {
		if an[i] != bn[i] {
			return an[i] < bn[i]
		}
	}
	switch {
	case ap == bp:
		return false
	case ap != "" && bp == "":
		return true
	case ap == "" && bp != "":
		return false
	}
	return ap < bp
}

// ---- asking ----

// updateKeyOf is the key releases are checked with: the setting's for a test, else the program's.
func (a *app) updateKeyOf() string {
	if a.updKey != "" {
		return a.updKey
	}
	return updateKey
}

// updateURLOf is where the manifest is: the setting's, else the program's.
func (a *app) updateURLOf() string {
	if a.settings.UpdateURL != "" {
		return a.settings.UpdateURL
	}
	return updateURL
}

// allowHTTP lets a manifest or a file come over plain HTTP: only a test does.
var allowHTTP = false

func httpsOnly(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" && !(allowHTTP && u.Scheme == "http") {
		return errors.New("yalnız HTTPS adresinden güncellenir: " + raw)
	}
	return nil
}

// fetchRelease reads the manifest at u and gives the release for this system, newer than current - or nil.
func fetchRelease(ctx context.Context, u, current, channel string, skip string) (*Release, error) {
	if err := httpsOnly(u); err != nil {
		return nil, err
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("User-Agent", "ryudit/"+current) // the only thing that is said: which version asks
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("güncelleme sunucusu %d döndü", resp.StatusCode)
	}
	var rel Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("güncelleme bilgisi okunamadı: %w", err)
	}
	if rel.Channel == "beta" && channel != "beta" {
		return nil, nil // a trial is for those who asked for trials
	}
	if !versionLess(current, rel.Version) || rel.Version == skip {
		return nil, nil
	}
	if _, ok := rel.Assets[runtime.GOOS+"-"+runtime.GOARCH]; !ok {
		return nil, fmt.Errorf("%s için bu sürümde dosya yok", runtime.GOOS+"/"+runtime.GOARCH)
	}
	return &rel, nil
}

// checkUpdate asks in the background, when the settings allow it and it is time; the answer is kept for
// the window to show.
func (a *app) checkUpdate(force bool) {
	u := a.updateURLOf()
	if u == "" || a.updBusy.Swap(true) {
		if force && u == "" {
			a.notice = "Güncelleme adresi ayarlanmamış (Ayarlar ▸ Güncelleme)."
		}
		return
	}
	if !force {
		if a.settings.UpdateOff || time.Since(time.Unix(a.settings.UpdateAt, 0)) < a.updateEvery() || time.Now().Before(time.Unix(a.settings.UpdateSnooze, 0)) {
			a.updBusy.Store(false)
			return
		}
	}
	cur, channel, skip := version, a.settings.UpdateChannel, a.settings.UpdateSkip
	if force {
		skip = ""
	}
	go func() {
		defer a.guard()
		defer a.updBusy.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		rel, err := fetchRelease(ctx, u, cur, channel, skip)
		a.do(func() error {
			a.settings.UpdateAt = time.Now().Unix()
			a.saveSettingsSoon()
			switch {
			case err != nil:
				a.updErr = err.Error()
				if force {
					a.notice = "Güncelleme denetlenemedi: " + err.Error()
				}
			case rel == nil:
				a.updErr = ""
				if force {
					a.notice = "Güncelsin: " + version
				}
			default:
				a.updErr, a.updRel = "", rel
				if a.settings.UpdateAuto {
					a.updAuto = true
				} else if !a.settings.UpdateQuiet || force {
					a.updOpen = true
				}
			}
			return nil
		})
	}()
}

// updateEvery is the time between two checks.
func (a *app) updateEvery() time.Duration {
	if h := a.settings.UpdateHours; h >= 1 && h <= 24*30 {
		return time.Duration(h) * time.Hour
	}
	return 24 * time.Hour
}

// ---- downloading and putting in place ----

// download takes the release's asset for this system into dir, checking its size, hash and signature.
func download(ctx context.Context, rel *Release, pubKey, dir string) (string, Asset, error) {
	as, ok := rel.Assets[runtime.GOOS+"-"+runtime.GOARCH]
	if !ok {
		return "", as, errors.New("bu sistem için dosya yok")
	}
	if err := httpsOnly(as.URL); err != nil {
		return "", as, err
	}
	if as.SHA256 == "" || as.Sig == "" {
		return "", as, errors.New("imzasız güncelleme kurulmaz")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", as, err
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", as.URL, nil)
	resp, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return "", as, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", as, fmt.Errorf("indirme %d döndü", resp.StatusCode)
	}
	tmp := filepath.Join(dir, "ryudit-"+rel.Version+".part")
	f, err := os.Create(tmp)
	if err != nil {
		return "", as, err
	}
	h := sha256.New()
	limit := int64(1 << 30)
	if as.Size > 0 {
		limit = as.Size + 1
	}
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, limit))
	f.Close()
	if err != nil {
		os.Remove(tmp)
		return "", as, err
	}
	if as.Size > 0 && n != as.Size {
		os.Remove(tmp)
		return "", as, fmt.Errorf("dosya boyutu tutmadı: %d, olması gereken %d", n, as.Size)
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, as.SHA256) {
		os.Remove(tmp)
		return "", as, errors.New("sağlama toplamı tutmadı: dosya bozuk ya da değiştirilmiş")
	}
	if err := verifyAsset(pubKey, rel.Version, as.SHA256, as.Sig); err != nil {
		os.Remove(tmp)
		return "", as, err
	}
	return tmp, as, nil
}

// updateMark is the file that says an update was put in place and has not yet proved itself: it holds
// the version and the path of the old program.
func updateMark() string {
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "ryudit", "update-pending")
	}
	return ""
}

// installBinary puts the downloaded file in the place of the program at exe: the old one is moved aside
// as exe.old (a running program can be renamed, on Windows too), the new one takes its name.
func installBinary(newFile, exe string) (old string, err error) {
	old = exe + ".old"
	os.Remove(old)
	if err := os.Chmod(newFile, 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(exe, old); err != nil {
		return "", err
	}
	if err := os.Rename(newFile, exe); err != nil {
		os.Rename(old, exe) // not put in place: the old one stays what it was
		return "", err
	}
	return old, nil
}

// installApp unpacks a zipped macOS .app beside the program's bundle and swaps the two; app is the
// bundle's path (…/ryudit.app). The old bundle is kept as app.old.
func installApp(zipFile, app string) (old string, err error) {
	zr, err := zip.OpenReader(zipFile)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	staging := app + ".new"
	os.RemoveAll(staging)
	root := ""
	for _, f := range zr.File {
		name := filepath.Clean(f.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			os.RemoveAll(staging)
			return "", errors.New("zip'te geçersiz yol: " + f.Name)
		}
		if root == "" {
			root = strings.Split(name, string(filepath.Separator))[0]
		}
		dst := filepath.Join(staging, strings.TrimPrefix(strings.TrimPrefix(name, root), string(filepath.Separator)))
		if f.FileInfo().IsDir() {
			os.MkdirAll(dst, 0o755)
			continue
		}
		os.MkdirAll(filepath.Dir(dst), 0o755)
		in, err := f.Open()
		if err != nil {
			os.RemoveAll(staging)
			return "", err
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode()|0o600)
		if err != nil {
			in.Close()
			os.RemoveAll(staging)
			return "", err
		}
		_, err = io.Copy(out, in)
		in.Close()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.RemoveAll(staging)
			return "", err
		}
	}
	old = app + ".old"
	os.RemoveAll(old)
	if err := os.Rename(app, old); err != nil {
		os.RemoveAll(staging)
		return "", err
	}
	if err := os.Rename(staging, app); err != nil {
		os.Rename(old, app)
		return "", err
	}
	return old, nil
}

// rollbackIfBroken, at the start: an update that was put in place and whose first start ended badly is
// taken back - the old program is put where the new one is. It reports what it did.
func rollbackIfBroken(mark string, unclean bool) string {
	if mark == "" {
		return ""
	}
	b, err := os.ReadFile(mark)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) < 3 {
		os.Remove(mark)
		return ""
	}
	ver, target, old := lines[0], lines[1], lines[2]
	if !unclean {
		return "" // the new one is on its first start: it proves itself (updateProved)
	}
	os.Remove(mark)
	if _, err := os.Stat(old); err != nil {
		return ""
	}
	os.RemoveAll(target + ".failed")
	if err := os.Rename(target, target+".failed"); err != nil {
		return ""
	}
	if err := os.Rename(old, target); err != nil {
		os.Rename(target+".failed", target)
		return ""
	}
	return "Sürüm " + ver + " açılışta düştü: önceki sürüme dönüldü."
}

// updateProved, some time into a start, says the update works: the mark goes, and the old file after a day.
func updateProved(mark string) {
	if mark == "" {
		return
	}
	b, err := os.ReadFile(mark)
	if err != nil {
		return
	}
	if lines := strings.Split(strings.TrimSpace(string(b)), "\n"); len(lines) >= 3 {
		old := lines[2]
		time.AfterFunc(24*time.Hour, func() { os.RemoveAll(old) })
	}
	os.Remove(mark)
}

// startUpdate downloads the release in the background, puts it in place and offers to restart.
func (a *app) startUpdate() {
	rel := a.updRel
	if rel == nil || a.updBusy.Swap(true) {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		a.updBusy.Store(false)
		a.notice = "Güncellenemedi: " + err.Error()
		return
	}
	exe, _ = filepath.EvalSymlinks(exe)
	pub := a.updateKeyOf()
	a.updState = "indiriliyor…"
	a.backupBefore("güncelleme")
	go func() {
		defer a.guard()
		defer a.updBusy.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		file, as, err := download(ctx, rel, pub, filepath.Join(os.TempDir(), "ryudit-update"))
		var old, target string
		if err == nil {
			if as.Kind == "app-zip" {
				target = appBundleOf(exe)
				if target == "" {
					err = errors.New("macOS paketi bulunamadı: program bir .app içinde değil")
				} else {
					old, err = installApp(file, target)
				}
			} else {
				target = exe
				old, err = installBinary(file, exe)
			}
		}
		a.do(func() error {
			if err != nil {
				a.updState = ""
				a.updErr = err.Error()
				a.notice = "Güncellenemedi: " + err.Error()
				return nil
			}
			if m := updateMark(); m != "" {
				os.WriteFile(m, []byte(rel.Version+"\n"+target+"\n"+old+"\n"), 0o644)
			}
			a.settings.UpdateNotes, a.settings.UpdateFor = rel.Notes, rel.Version
			a.saveSettings()
			a.updState = "hazır"
			a.updOpen = true
			return nil
		})
	}()
}

// appBundleOf is the .app that holds the program at exe, or "".
func appBundleOf(exe string) string {
	for d := filepath.Dir(exe); d != "/" && d != "." && d != ""; d = filepath.Dir(d) {
		if strings.HasSuffix(d, ".app") {
			return d
		}
	}
	return ""
}

// whatsNew reports whether this is the first start of a version that was installed by an update: its notes are shown.
func (a *app) whatsNew() bool {
	return a.settings.UpdateFor != "" && a.settings.UpdateFor == version && a.settings.UpdateSeen != version
}
