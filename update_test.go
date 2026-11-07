package main

import (
	"archive/zip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestVersionOrder(t *testing.T) {
	for _, c := range []struct {
		a, b string
		less bool
	}{
		{"v1.2.3", "v1.2.4", true}, {"v1.2.4", "v1.2.3", false}, {"1.2", "1.10", true}, {"v1.9.0", "v1.10.0", true},
		{"v2.0.0-beta.1", "v2.0.0", true}, {"v2.0.0", "v2.0.0-beta.1", false}, {"v2.0.0-beta.1", "v2.0.0-beta.2", true},
		{"v1.0.0", "v1.0.0", false}, {"dev", "v9.9.9", false}, {"v1.0.0", "garbage", false}, {"v0.1.0+build5", "v0.1.1", true},
	} {
		if got := versionLess(c.a, c.b); got != c.less {
			t.Errorf("%s < %s: %v", c.a, c.b, got)
		}
	}
}

func testKey(t *testing.T) (ed25519.PrivateKey, string) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return priv, base64.StdEncoding.EncodeToString(pub)
}

func TestSignatures(t *testing.T) {
	priv, pub := testKey(t)
	sha := strings.Repeat("ab", 32)
	sig := SignAsset(priv, "v1.3.0", sha)
	if err := verifyAsset(pub, "v1.3.0", sha, sig); err != nil {
		t.Fatal(err)
	}
	if verifyAsset(pub, "v1.3.1", sha, sig) == nil {
		t.Fatal("a signature moved to another version")
	}
	if verifyAsset(pub, "v1.3.0", strings.Repeat("cd", 32), sig) == nil {
		t.Fatal("a signature moved to another file")
	}
	_, other := testKey(t)
	if verifyAsset(other, "v1.3.0", sha, sig) == nil {
		t.Fatal("another key accepted it")
	}
	if verifyAsset("", "v1.3.0", sha, sig) == nil || verifyAsset("zz", "v1.3.0", sha, sig) == nil {
		t.Fatal("no key: still accepted")
	}
}

// releaseServer serves a manifest and a file; mut can change the manifest.
func releaseServer(t *testing.T, priv ed25519.PrivateKey, ver string, file []byte, mut func(*Release)) (*httptest.Server, *Release) {
	t.Helper()
	allowHTTP = true
	t.Cleanup(func() { allowHTTP = false })
	sum := sha256.Sum256(file)
	sha := hex.EncodeToString(sum[:])
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	rel := &Release{Version: ver, Date: "2026-11-07", Notes: "# Yenilikler\n- hızlandı", Assets: map[string]Asset{
		runtime.GOOS + "-" + runtime.GOARCH: {URL: srv.URL + "/file", SHA256: sha, Sig: SignAsset(priv, ver, sha), Size: int64(len(file))},
	}}
	if mut != nil {
		mut(rel)
	}
	mux.HandleFunc("/update.json", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(rel) })
	mux.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) { w.Write(file) })
	return srv, rel
}

func TestFetchRelease(t *testing.T) {
	priv, _ := testKey(t)
	srv, _ := releaseServer(t, priv, "v2.0.0", []byte("yeni"), nil)
	ctx := context.Background()
	rel, err := fetchRelease(ctx, srv.URL+"/update.json", "v1.0.0", "", "")
	if err != nil || rel == nil || rel.Version != "v2.0.0" {
		t.Fatalf("%v %v", rel, err)
	}
	if rel, _ := fetchRelease(ctx, srv.URL+"/update.json", "v2.0.0", "", ""); rel != nil {
		t.Fatal("the same version offered")
	}
	if rel, _ := fetchRelease(ctx, srv.URL+"/update.json", "v3.0.0", "", ""); rel != nil {
		t.Fatal("an older version offered")
	}
	if rel, _ := fetchRelease(ctx, srv.URL+"/update.json", "v1.0.0", "", "v2.0.0"); rel != nil {
		t.Fatal("a skipped version offered")
	}
	if rel, _ := fetchRelease(ctx, srv.URL+"/update.json", "dev", "", ""); rel != nil {
		t.Fatal("a build from the source asked to be replaced")
	}
	beta, _ := releaseServer(t, priv, "v2.1.0-beta.1", []byte("x"), func(r *Release) { r.Channel = "beta" })
	if rel, _ := fetchRelease(ctx, beta.URL+"/update.json", "v1.0.0", "", ""); rel != nil {
		t.Fatal("a trial offered to the stable channel")
	}
	if rel, _ := fetchRelease(ctx, beta.URL+"/update.json", "v1.0.0", "beta", ""); rel == nil {
		t.Fatal("a trial not offered to the trial channel")
	}
	none, _ := releaseServer(t, priv, "v2.0.0", []byte("x"), func(r *Release) { r.Assets = map[string]Asset{"plan9-mips": {}} })
	if _, err := fetchRelease(ctx, none.URL+"/update.json", "v1.0.0", "", ""); err == nil {
		t.Fatal("no file for this system, and no error")
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "x", 500) }))
	defer bad.Close()
	if _, err := fetchRelease(ctx, bad.URL, "v1.0.0", "", ""); err == nil {
		t.Fatal("a server error taken as no news")
	}
	junk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>")) }))
	defer junk.Close()
	if _, err := fetchRelease(ctx, junk.URL, "v1.0.0", "", ""); err == nil {
		t.Fatal("html taken as a manifest")
	}
	allowHTTP = false
	if _, err := fetchRelease(ctx, srv.URL+"/update.json", "v1.0.0", "", ""); err == nil {
		t.Fatal("plain HTTP accepted")
	}
}

func TestFetchSendsOnlyTheVersion(t *testing.T) {
	var got http.Header
	allowHTTP = true
	defer func() { allowHTTP = false }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.Write([]byte(`{"version":"v1.0.0","assets":{}}`))
	}))
	defer srv.Close()
	fetchRelease(context.Background(), srv.URL, "v1.0.0", "", "")
	if ua := got.Get("User-Agent"); ua != "ryudit/v1.0.0" {
		t.Fatalf("user agent %q", ua)
	}
	if got.Get("Cookie") != "" || got.Get("Authorization") != "" {
		t.Fatalf("more was sent: %v", got)
	}
}

func TestDownloadChecks(t *testing.T) {
	priv, pub := testKey(t)
	file := []byte("yeni program baytları")
	srv, rel := releaseServer(t, priv, "v2.0.0", file, nil)
	dir := t.TempDir()
	path, _, err := download(context.Background(), rel, pub, dir)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != string(file) {
		t.Fatal("not the file")
	}
	key := runtime.GOOS + "-" + runtime.GOARCH
	try := func(name string, mut func(*Asset), pubKey string) {
		t.Helper()
		r := *rel
		r.Assets = map[string]Asset{key: rel.Assets[key]}
		a := r.Assets[key]
		mut(&a)
		r.Assets[key] = a
		if p, _, err := download(context.Background(), &r, pubKey, t.TempDir()); err == nil {
			t.Errorf("%s: accepted (%s)", name, p)
		}
	}
	try("wrong hash", func(a *Asset) { a.SHA256 = strings.Repeat("0", 64) }, pub)
	try("hash of another file with its own signature", func(a *Asset) {
		s := sha256.Sum256([]byte("başka"))
		a.SHA256 = hex.EncodeToString(s[:])
		a.Sig = SignAsset(priv, "v2.0.0", a.SHA256)
	}, pub)
	try("no signature", func(a *Asset) { a.Sig = "" }, pub)
	try("signed by another key", func(a *Asset) {}, func() string { _, o := testKey(t); return o }())
	try("a size that is not the file's", func(a *Asset) { a.Size = 3 }, pub)
	try("not https", func(a *Asset) { a.URL = strings.Replace(srv.URL, "http://", "ftp://", 1) + "/file" }, pub)
	_ = srv
}

func TestInstallBinaryAndRollback(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "ryudit")
	os.WriteFile(exe, []byte("eski"), 0o755)
	nf := filepath.Join(dir, "yeni.part")
	os.WriteFile(nf, []byte("yeni"), 0o644)
	old, err := installBinary(nf, exe)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(exe); string(b) != "yeni" {
		t.Fatal("not in place")
	}
	if b, _ := os.ReadFile(old); string(b) != "eski" {
		t.Fatal("the old one is not kept")
	}
	if st, _ := os.Stat(exe); st.Mode()&0o111 == 0 {
		t.Fatal("not executable")
	}
	mark := filepath.Join(dir, "update-pending")
	os.WriteFile(mark, []byte("v2.0.0\n"+exe+"\n"+old+"\n"), 0o644)
	// a first start that is going on: nothing is taken back
	if note := rollbackIfBroken(mark, false); note != "" {
		t.Fatalf("rolled back a start that is fine: %q", note)
	}
	if b, _ := os.ReadFile(exe); string(b) != "yeni" {
		t.Fatal("changed")
	}
	// the next start finds the last one ended badly
	note := rollbackIfBroken(mark, true)
	if !strings.Contains(note, "önceki sürüme dönüldü") {
		t.Fatalf("note %q", note)
	}
	if b, _ := os.ReadFile(exe); string(b) != "eski" {
		t.Fatalf("not back: %q", b)
	}
	if b, _ := os.ReadFile(exe + ".failed"); string(b) != "yeni" {
		t.Fatal("the failed one is not kept for a look")
	}
	if _, err := os.Stat(mark); err == nil {
		t.Fatal("the mark stayed")
	}
	// a start that proved itself takes the mark away
	os.WriteFile(mark, []byte("v2.0.0\n"+exe+"\n"+old+"\n"), 0o644)
	updateProved(mark)
	if _, err := os.Stat(mark); err == nil {
		t.Fatal("proved, and the mark stayed")
	}
	// a program that cannot be replaced stays what it was
	if _, err := installBinary(filepath.Join(dir, "yok"), exe); err == nil {
		t.Fatal("installed a file that is not there")
	}
	if b, _ := os.ReadFile(exe); string(b) != "eski" {
		t.Fatal("the program was lost by a failed install")
	}
}

func TestInstallApp(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "ryudit.app")
	os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755)
	os.WriteFile(filepath.Join(app, "Contents", "MacOS", "ryudit"), []byte("eski"), 0o755)
	zp := filepath.Join(dir, "yeni.zip")
	f, _ := os.Create(zp)
	zw := zip.NewWriter(f)
	w, _ := zw.CreateHeader(&zip.FileHeader{Name: "ryudit.app/Contents/MacOS/ryudit", Method: zip.Deflate})
	w.Write([]byte("yeni"))
	zw.Close()
	f.Close()
	old, err := installApp(zp, app)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(app, "Contents", "MacOS", "ryudit")); string(b) != "yeni" {
		t.Fatalf("%q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(old, "Contents", "MacOS", "ryudit")); string(b) != "eski" {
		t.Fatal("old bundle not kept")
	}
	// a zip with a path that climbs out is refused
	evil := filepath.Join(dir, "evil.zip")
	f, _ = os.Create(evil)
	zw = zip.NewWriter(f)
	w, _ = zw.Create("ryudit.app/../../kaçtı")
	w.Write([]byte("x"))
	zw.Close()
	f.Close()
	if _, err := installApp(evil, app); err == nil {
		t.Fatal("a zip that climbs out was unpacked")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "kaçtı")); err == nil {
		t.Fatal("a file got out")
	}
}

func TestAppBundleOf(t *testing.T) {
	if got := appBundleOf("/Applications/ryudit.app/Contents/MacOS/ryudit"); got != "/Applications/ryudit.app" {
		t.Fatal(got)
	}
	if appBundleOf("/usr/local/bin/ryudit") != "" {
		t.Fatal("a bundle where there is none")
	}
}

// The check: off, not due, snoozed - nothing is asked; forced - it is; a found release opens the dialog, or waits when quiet.
func TestCheckUpdateSettings(t *testing.T) {
	priv, pub := testKey(t)
	srv, _ := releaseServer(t, priv, "v9.0.0", []byte("x"), nil)
	old := version
	version = "v1.0.0"
	defer func() { version = old }()
	a, tt := newTestApp(t, "x\n")
	a.updKey = pub
	a.settings.UpdateURL = srv.URL + "/update.json"
	wait := func() {
		for i := 0; i < 300 && a.updBusy.Load(); i++ {
			time.Sleep(10 * time.Millisecond)
		}
		time.Sleep(30 * time.Millisecond)
		tt.Frame()
	}
	a.settings.UpdateOff = true
	a.checkUpdate(false)
	wait()
	if a.updRel != nil {
		t.Fatal("asked with the check off")
	}
	a.settings.UpdateOff = false
	a.settings.UpdateSnooze = time.Now().Add(time.Hour).Unix()
	a.checkUpdate(false)
	wait()
	if a.updRel != nil {
		t.Fatal("asked while snoozed")
	}
	a.settings.UpdateSnooze = 0
	a.settings.UpdateAt = time.Now().Unix()
	a.checkUpdate(false)
	wait()
	if a.updRel != nil {
		t.Fatal("asked again within the day")
	}
	a.settings.UpdateAt = 0
	a.settings.UpdateQuiet = true
	a.checkUpdate(false)
	wait()
	if a.updRel == nil || a.updOpen {
		t.Fatalf("quiet: release %v open %v", a.updRel, a.updOpen)
	}
	a.updRel = nil
	a.settings.UpdateQuiet = false
	a.settings.UpdateAt = 0
	a.checkUpdate(false)
	wait()
	if a.updRel == nil || !a.updOpen || !tt.HasText("Sürüm v9.0.0 hazır") {
		t.Fatalf("not shown: %v", tt.Texts())
	}
	for _, want := range []string{"Şimdi güncelle", "3 gün sonra hatırlat", "Bu sürümü atla", "hızlandı"} {
		if !tt.HasText(want) {
			t.Errorf("no %q: %v", want, tt.Texts())
		}
	}
	if err := tt.Click("Bu sürümü atla"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(100 * time.Millisecond)
	if a.settings.UpdateSkip != "v9.0.0" || a.updOpen {
		t.Fatalf("skip %q open %v", a.settings.UpdateSkip, a.updOpen)
	}
	a.updRel = nil
	a.settings.UpdateAt = 0
	a.checkUpdate(false)
	wait()
	if a.updRel != nil {
		t.Fatal("a skipped version came again")
	}
	// a forced check says what it found
	a.checkUpdate(true)
	wait()
	if a.updRel == nil {
		t.Fatal("forced, and nothing")
	}
}

func TestNoURLSaysSo(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	a.checkUpdate(true)
	if !strings.Contains(a.notice, "adresi ayarlanmamış") {
		t.Fatalf("%q", a.notice)
	}
}

func TestWhatsNew(t *testing.T) {
	old := version
	version = "v3.0.0"
	defer func() { version = old }()
	a, tt := newTestApp(t, "x\n")
	a.settings.UpdateFor, a.settings.UpdateNotes = "v3.0.0", "- yeni şeyler"
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if !tt.HasText("Bu sürümde neler yeni: v3.0.0") {
		t.Fatalf("%v", tt.Texts())
	}
	tt.Click("Tamam")
	tt.Advance(100 * time.Millisecond)
	if a.settings.UpdateSeen != "v3.0.0" || a.updOpen {
		t.Fatal("not marked as seen")
	}
	tt.Frame()
	if tt.HasText("Bu sürümde neler yeni: v3.0.0") {
		t.Fatal("shown again")
	}
}
