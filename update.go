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
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/imhalid/celestine/locales"
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
		return errors.New(locales.UpdateErrKey())
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil || !ed25519.Verify(ed25519.PublicKey(pub), sigText(version, sha), sig) {
		return errors.New(locales.UpdateErrSignature())
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

// fetchRelease reads the manifest at u and gives the release for this system, newer than current - or nil.
func fetchRelease(ctx context.Context, u, current, channel string, skip string) (*Release, error) {
	if err := HTTPSOnly(u); err != nil {
		return nil, err
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("User-Agent", "celestine/"+current) // the only thing that is said: which version asks
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New(locales.UpdateErrServer(strconv.Itoa(resp.StatusCode)))
	}
	var rel Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("%s: %w", locales.UpdateErrUnreadable(), err)
	}
	if rel.Channel == "beta" && channel != "beta" {
		return nil, nil // a trial is for those who asked for trials
	}
	if !versionLess(current, rel.Version) || rel.Version == skip {
		return nil, nil
	}
	if _, ok := rel.Assets[runtime.GOOS+"-"+runtime.GOARCH]; !ok {
		return nil, errors.New(locales.UpdateErrNoBuildFor(runtime.GOOS + "/" + runtime.GOARCH))
	}
	return &rel, nil
}

// checkUpdate asks in the background, when the settings allow it and it is time; the answer is kept for
// the window to show.
func (a *app) checkUpdate(force bool) {
	u := a.updateURLOf()
	if u == "" || a.updBusy.Swap(true) {
		if force && u == "" {
			a.say(toastWarn, locales.UpdateCannotCheck())
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
					a.say(toastErr, locales.UpdateCheckFailed(err.Error()))
				}
			case rel == nil:
				a.updErr = ""
				if force {
					a.say(toastOK, locales.UpdateUpToDate(shownVersion(version)))
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

// An updateState is where an update the user asked for is.
type updateState int

const (
	updIdle        updateState = iota // none, or it failed
	updDownloading                    // downloading, checking and putting it in place
	updReady                          // in place: a restart opens it
)

// ---- downloading and putting in place ----

// updProgress is how far a download is: the bytes come in, and the bytes it has, 0 when that is not told. The
// download writes it and the chip reads it, each on its own goroutine.
type updProgress struct{ got, size atomic.Int64 }

// Write counts the bytes as they come in.
func (u *updProgress) Write(b []byte) (int, error) {
	u.got.Add(int64(len(b)))
	return len(b), nil
}

// frac is how far it is, 0 to 1, or -1 when its size is not told.
func (u *updProgress) frac() float32 {
	n := u.size.Load()
	if n <= 0 {
		return -1
	}
	return float32(min(float64(u.got.Load())/float64(n), 1))
}

// download takes the release's asset for this system into a new file in dir, checking its size, hash and
// signature. dir is the caller's own (updateDir): beside the program, so the file is renamed into place on one
// filesystem, and closed to anyone else, so what was checked is what is put in place. prog, when given, counts it
// as it comes.
func download(ctx context.Context, rel *Release, pubKey, dir string, prog *updProgress) (string, Asset, error) {
	as, ok := rel.Assets[runtime.GOOS+"-"+runtime.GOARCH]
	if !ok {
		return "", as, errors.New(locales.UpdateErrNoBuild())
	}
	if err := HTTPSOnly(as.URL); err != nil {
		return "", as, err
	}
	if as.SHA256 == "" || as.Sig == "" {
		return "", as, errors.New(locales.UpdateErrUnsigned())
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", as.URL, nil)
	resp, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return "", as, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", as, errors.New(locales.UpdateErrDownload(strconv.Itoa(resp.StatusCode)))
	}
	f, err := os.CreateTemp(dir, "celestine-"+rel.Version+"-*.part") // a name of its own, made new: nothing left there is followed
	if err != nil {
		return "", as, err
	}
	tmp := f.Name()
	h := sha256.New()
	limit := int64(1 << 30)
	if as.Size > 0 {
		limit = as.Size + 1
	}
	var w io.Writer = io.MultiWriter(f, h)
	if prog != nil {
		size := as.Size
		if size <= 0 {
			size = resp.ContentLength
		}
		prog.got.Store(0)
		prog.size.Store(size)
		w = io.MultiWriter(f, h, prog)
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, limit))
	f.Close()
	if err != nil {
		os.Remove(tmp)
		return "", as, err
	}
	if as.Size > 0 && n != as.Size {
		os.Remove(tmp)
		return "", as, errors.New(locales.UpdateErrSize(int(n), int(as.Size)))
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, as.SHA256) {
		os.Remove(tmp)
		return "", as, errors.New(locales.UpdateErrChecksum())
	}
	if err := verifyAsset(pubKey, rel.Version, as.SHA256, as.Sig); err != nil {
		os.Remove(tmp)
		return "", as, err
	}
	return tmp, as, nil
}

// updateDir makes the folder an update is downloaded into: beside what it replaces (the bundle on macOS, else
// the program), so the swap is a rename on one filesystem and not a copy from a temp folder on another; and
// readable by this user alone, so nobody can change the file between its check and the swap. The caller
// removes it.
func updateDir(exe, bundle string) (string, error) {
	target := exe
	if bundle != "" {
		target = bundle
	}
	dir, err := os.MkdirTemp(filepath.Dir(target), ".celestine-update-")
	if err != nil {
		return "", fmt.Errorf("%s: %w", locales.UpdateErrFolder(), err)
	}
	return dir, nil
}

// updateMark is the file that says an update was put in place and has not yet proved itself: it holds
// the version and the path of the old program.
func updateMark() string {
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "celestine", "update-pending")
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
// bundle's path (…/celestine.app). The old bundle is kept as app.old.
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
			return "", errors.New(locales.UpdateErrZipPath(f.Name))
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

// startedStamp is the line an update's first start adds to its mark before anything else can fail.
const startedStamp = "started"

// readMark gives the lines of an update's mark: the version, what was replaced, the old one kept, and
// startedStamp once the new version began its first start. A mark that is not one is removed.
func readMark(mark string) []string {
	if mark == "" {
		return nil
	}
	b, err := os.ReadFile(mark)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) < 3 {
		os.Remove(mark)
		return nil
	}
	return lines
}

// rollbackIfBroken, as early in a start as it can be: the update's own version stamps its first start in the
// mark; a start that finds that stamp and no proof (updateProved) follows one that died, and the old program is
// put back where the new one is. Only the stamp counts, not how the last session ended: an old copy killed after
// "Later" leaves the new one's first start clean, and a new one that dies before its window is still caught
// (#234). cur is this program's version: a mark of another version is not this program's to judge. It reports
// what it did.
func rollbackIfBroken(mark, cur string) string {
	lines := readMark(mark)
	if lines == nil || lines[0] != cur {
		return ""
	}
	ver, target, old := lines[0], lines[1], lines[2]
	if len(lines) < 4 || lines[3] != startedStamp {
		os.WriteFile(mark, []byte(strings.Join(lines[:3], "\n")+"\n"+startedStamp+"\n"), 0o644)
		return "" // the first start: it proves itself (updateProved)
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
	return "version " + ver + " died at its first start: the previous version is back"
}

// updateProved says the update works, twenty seconds into its first start or at a quit that came sooner: the
// mark goes, and the old program with it, now and not a day later, which a short session never reached.
func updateProved(mark, cur string) {
	lines := readMark(mark)
	if lines == nil || lines[0] != cur {
		return
	}
	os.RemoveAll(lines[2])
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
		a.say(toastErr, locales.UpdateFailed(err.Error()))
		return
	}
	exe, _ = filepath.EvalSymlinks(exe)
	bundle := ""
	if as := rel.Assets[runtime.GOOS+"-"+runtime.GOARCH]; as.Kind == "app-zip" {
		if bundle = appBundleOf(exe); bundle == "" {
			a.updBusy.Store(false)
			a.say(toastErr, locales.UpdateFailed(locales.UpdateErrNotInApp()))
			return
		}
	}
	dir, err := updateDir(exe, bundle)
	if err != nil {
		a.updBusy.Store(false)
		a.say(toastErr, locales.UpdateFailed(err.Error()))
		return
	}
	pub := a.updateKeyOf()
	a.updState = updDownloading
	a.backupBefore("güncelleme")
	go func() {
		defer a.guard()
		defer a.updBusy.Store(false)
		defer os.RemoveAll(dir)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		file, _, err := download(ctx, rel, pub, dir, &a.updProg)
		var old, target string
		if err == nil {
			if bundle != "" {
				target = bundle
				old, err = installApp(file, target)
			} else {
				target = exe
				old, err = installBinary(file, exe)
			}
		}
		a.do(func() error {
			if err != nil {
				a.updState = updIdle
				a.updErr = err.Error()
				a.say(toastErr, locales.UpdateFailed(err.Error()))
				return nil
			}
			if m := updateMark(); m != "" {
				os.WriteFile(m, []byte(rel.Version+"\n"+target+"\n"+old+"\n"), 0o644)
			}
			a.settings.UpdateNotes, a.settings.UpdateFor = rel.Notes, rel.Version
			a.saveSettings()
			a.updState = updReady // the card, if it is open, says restart; else the chip does
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
