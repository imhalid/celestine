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
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/locales"
)

func TestVersionOrder(t *testing.T) {
	for _, c := range []struct {
		a, b string
		less bool
	}{
		{"v1.2.3", "v1.2.4", true}, {"v1.2.4", "v1.2.3", false}, {"1.2", "1.10", true}, {"v1.9.0", "v1.10.0", true},
		{"v2.0.0-beta.1", "v2.0.0", true}, {"v2.0.0", "v2.0.0-beta.1", false}, {"v2.0.0-beta.1", "v2.0.0-beta.2", true},
		{"v1.0.0", "v1.0.0", false}, {"dev", "v9.9.9", false}, {"v1.0.0", "garbage", false}, {"v0.1.0+build5", "v0.1.1", true},
		{"v2.0.0-beta.2", "v2.0.0-beta.10", true}, {"v2.0.0-beta.10", "v2.0.0-beta.2", false}, {"v2.0.0-1", "v2.0.0-alpha", true},
		{"v2.0.0-alpha", "v2.0.0-alpha.1", true}, {"v2.0.0-alpha.1", "v2.0.0-alpha", false}, {"v2.0.0-alpha.beta", "v2.0.0-beta", true},
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
	rel := &Release{Version: "v1.3.0", Notes: "- faster"}
	as := Asset{URL: "https://dl.example.com/v1.3.0/celestine-darwin-arm64.zip", SHA256: strings.Repeat("ab", 32), Kind: "app-zip"}
	as.Sig3 = SignAsset(priv, rel, "darwin-arm64", as)
	if err := verifyAsset(pub, rel, "darwin-arm64", as); err != nil {
		t.Fatal(err)
	}
	refused := func(why, platform string, mutRel func(*Release), mut func(*Asset), key string) {
		t.Helper()
		r, a := *rel, as
		mutRel(&r)
		mut(&a)
		if verifyAsset(key, &r, platform, a) == nil {
			t.Errorf("%s: accepted", why)
		}
	}
	same := func(*Asset) {}
	sameRel := func(*Release) {}
	refused("moved to another version", "darwin-arm64", func(r *Release) { r.Version = "v1.3.1" }, same, pub)
	refused("moved to another file", "darwin-arm64", sameRel, func(a *Asset) { a.SHA256 = strings.Repeat("cd", 32) }, pub)
	// #455: a darwin-arm64 file, rightly signed, served to linux-amd64 under its key
	refused("moved to another system", "linux-amd64", sameRel, same, pub)
	refused("renamed", "darwin-arm64", sameRel, func(a *Asset) { a.URL = "https://dl.example.com/v1.3.0/celestine-linux-amd64" }, pub)
	refused("moved to another folder's name", "darwin-arm64", sameRel, func(a *Asset) { a.URL = "https://dl.example.com/v1.3.0/x/" }, pub)
	refused("another kind", "darwin-arm64", sameRel, func(a *Asset) { a.Kind = "binary" }, pub)
	// #168: the channel and the notes are the signer's too
	refused("moved to the beta channel", "darwin-arm64", func(r *Release) { r.Channel = "beta" }, same, pub)
	refused("other notes", "darwin-arm64", func(r *Release) { r.Notes = "- faster\n- and free money: https://example.com" }, same, pub)
	refused("no notes", "darwin-arm64", func(r *Release) { r.Notes = "" }, same, pub)
	refused("no third signature", "darwin-arm64", sameRel, func(a *Asset) { a.Sig3 = "" }, pub)
	// an old-format manifest - only the signatures the released clients check, which leave the channel and the
	// notes open (and the first, the system and the name) - is not accepted here: a manifest stripped of sig3
	// would bring #168 back
	refused("only the signatures of #455 and before", "darwin-arm64", sameRel, func(a *Asset) {
		a.Sig = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, legacySigText("v1.3.0", a.SHA256)))
		a.Sig2 = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, must(sigText2("v1.3.0", "darwin-arm64", "celestine-darwin-arm64.zip", "app-zip", a.SHA256))))
		a.Sig3 = ""
	}, pub)
	refused("the second signature as the third", "darwin-arm64", sameRel, func(a *Asset) {
		a.Sig3 = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, must(sigText2("v1.3.0", "darwin-arm64", "celestine-darwin-arm64.zip", "app-zip", a.SHA256))))
	}, pub)
	refused("the old signature as the third", "darwin-arm64", sameRel, func(a *Asset) {
		a.Sig3 = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, legacySigText("v1.3.0", a.SHA256)))
	}, pub)
	_, other := testKey(t)
	refused("another key", "darwin-arm64", sameRel, same, other)
	refused("no key", "darwin-arm64", sameRel, same, "")
	refused("a key that is not one", "darwin-arm64", sameRel, same, "zz")
	refused("a line break in the version", "darwin-arm64", func(r *Release) { r.Version = "v1.3.0\ndarwin-arm64" }, same, pub)

	if !channelServes("", "") || !channelServes("", "beta") || !channelServes("stable", "") || !channelServes("beta", "beta") {
		t.Error("a release refused to a channel it is for")
	}
	if channelServes("beta", "") || channelServes("beta", "stable") || channelServes("nightly", "beta") {
		t.Error("a trial, or a channel there is none of, offered")
	}
}

// sigText2 is the text of the second signature, which the clients of #455 check (sig2): frozen here, as the
// release keeps writing it for them and scripts/release's test holds it to the same literal.
func sigText2(version, platform, name, kind, sha string) ([]byte, bool) {
	if kind == "" {
		kind = "binary"
	}
	parts := []string{"celestine-update-2", version, platform, name, kind, strings.ToLower(sha)}
	for _, p := range parts[1:] {
		if p == "" || strings.ContainsAny(p, "\r\n") {
			return nil, false
		}
	}
	return []byte(strings.Join(parts, "\n")), true
}

// legacySigText is the text of the first signature, the version and the hash, which the clients before #455
// check (sig): frozen here as sigText2 is.
func legacySigText(version, sha string) []byte { return []byte(version + "\n" + strings.ToLower(sha)) }

// must is a text that could be made, or the test's end.
func must(b []byte, ok bool) []byte {
	if !ok {
		panic("no text")
	}
	return b
}

// TestSigTextIsTheReleases: the text the app checks is the one scripts/release signs (its own test holds the same
// literal), and the old texts are still the ones released clients check.
func TestSigTextIsTheReleases(t *testing.T) {
	got, ok := sigText("v1.3.0-beta.1", "beta", "darwin-arm64", "celestine-darwin-arm64.zip", "app-zip", strings.Repeat("AB", 32), "- faster")
	if want := "celestine-update-3\nv1.3.0-beta.1\nbeta\ndarwin-arm64\ncelestine-darwin-arm64.zip\napp-zip\n" + strings.Repeat("ab", 32) +
		"\nc9d3478adaa449914687b73062b06018452b2f73e56431869abaacb933675946"; !ok || string(got) != want {
		t.Fatalf("%q, want %q", got, want)
	}
	if got, _ := sigText("v1.3.0", "", "linux-amd64", "celestine-linux-amd64", "", "ab", ""); string(got) != "celestine-update-3\nv1.3.0\nstable\nlinux-amd64\ncelestine-linux-amd64\nbinary\nab\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("no channel is not stable, or no kind not binary: %q", got)
	}
	if got, _ := sigText2("v1.3.0", "darwin-arm64", "celestine-darwin-arm64.zip", "app-zip", strings.Repeat("AB", 32)); string(got) != "celestine-update-2\nv1.3.0\ndarwin-arm64\ncelestine-darwin-arm64.zip\napp-zip\n"+strings.Repeat("ab", 32) {
		t.Fatalf("second text %q", got)
	}
	if got := legacySigText("v1.3.0", "AB"); string(got) != "v1.3.0\nab" {
		t.Fatalf("old text %q", got)
	}
	if got := assetName("https://dl.example.com/v1.3.0/celestine-windows-amd64.exe?x=1"); got != "celestine-windows-amd64.exe" {
		t.Fatalf("name %q", got)
	}
}

// TestReleaseToolManifest: what scripts/release writes holds for every client - the third signature for this one,
// the second for the clients of #455 and the first for those before (#168) - and its channel and notes are the
// ones signed.
func TestReleaseToolManifest(t *testing.T) {
	if testing.Short() {
		t.Skip("builds scripts/release")
	}
	pubRaw, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pub := base64.StdEncoding.EncodeToString(pubRaw)
	dir := t.TempDir()
	keyFile, notesFile := filepath.Join(dir, "release.key"), filepath.Join(dir, "notes.md")
	os.WriteFile(keyFile, []byte(base64.StdEncoding.EncodeToString(priv)), 0o600)
	os.WriteFile(notesFile, []byte("Since v1.2.0:\n- faster <and> \"quieter\" & ılık\n"), 0o644)
	files := map[string]string{ // the asset's key, and its file
		"darwin-arm64":         "celestine-darwin-arm64.zip",
		"linux-amd64":          "celestine-linux-amd64",
		"linux-amd64-appimage": "celestine-linux-amd64.AppImage",
		"windows-amd64":        "celestine-windows-amd64.exe",
	}
	args := []string{"run", "./scripts/release", "manifest", "-version", "v1.3.0-beta.1", "-base", "https://dl.example.com/v1.3.0-beta.1",
		"-channel", "beta", "-notes", notesFile, "-key", keyFile}
	for _, name := range files {
		f := filepath.Join(dir, name)
		os.WriteFile(f, []byte("the build "+name), 0o644)
		args = append(args, f)
	}
	cmd := exec.Command("go", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var rel Release
	if err := json.Unmarshal(out, &rel); err != nil {
		t.Fatal(err)
	}
	if rel.Channel != "beta" || !strings.Contains(rel.Notes, "ılık") {
		t.Fatalf("channel %q, notes %q", rel.Channel, rel.Notes)
	}
	key := ed25519.PublicKey(pubRaw)
	sig := func(s string) []byte { b, _ := base64.StdEncoding.DecodeString(s); return b }
	for platform, name := range files {
		as, ok := rel.Assets[platform]
		if !ok || assetName(as.URL) != name {
			t.Fatalf("%s: %+v", platform, as)
		}
		if err := verifyAsset(pub, &rel, platform, as); err != nil {
			t.Errorf("%s: this client refuses it: %v", platform, err)
		}
		if !ed25519.Verify(key, must(sigText2(rel.Version, platform, name, as.Kind, as.SHA256)), sig(as.Sig2)) {
			t.Errorf("%s: the clients of #455 refuse it", platform)
		}
		if !ed25519.Verify(key, legacySigText(rel.Version, as.SHA256), sig(as.Sig)) {
			t.Errorf("%s: the clients before #455 refuse it", platform)
		}
		stable := rel
		stable.Channel = ""
		if verifyAsset(pub, &stable, platform, as) == nil {
			t.Errorf("%s: the trial holds as a stable release", platform)
		}
	}
}

// releaseServer serves a manifest and a file, signed by priv; mut can change the manifest before it is signed.
func releaseServer(t *testing.T, priv ed25519.PrivateKey, ver string, file []byte, mut func(*Release)) (*httptest.Server, *Release) {
	t.Helper()
	library.AllowHTTP = true
	t.Cleanup(func() { library.AllowHTTP = false })
	sum := sha256.Sum256(file)
	sha := hex.EncodeToString(sum[:])
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	platform := runtime.GOOS + "-" + runtime.GOARCH
	as := Asset{URL: srv.URL + "/file", SHA256: sha, Size: int64(len(file))}
	rel := &Release{Version: ver, Date: "2026-11-07", Notes: "# Yenilikler\n- hızlandı", Assets: map[string]Asset{platform: as}}
	if mut != nil {
		mut(rel)
	}
	for k, a := range rel.Assets { // signed as it is served: a test changes the returned release to tamper with it
		a.Sig3 = SignAsset(priv, rel, k, a)
		rel.Assets[k] = a
	}
	mux.HandleFunc("/update.json", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(rel) })
	mux.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) { w.Write(file) })
	return srv, rel
}

func TestFetchRelease(t *testing.T) {
	priv, pub := testKey(t)
	srv, _ := releaseServer(t, priv, "v2.0.0", []byte("yeni"), nil)
	ctx := context.Background()
	rel, err := fetchRelease(ctx, srv.URL+"/update.json", pub, "v1.0.0", "", "")
	if err != nil || rel == nil || rel.Version != "v2.0.0" {
		t.Fatalf("%v %v", rel, err)
	}
	if rel, _ := fetchRelease(ctx, srv.URL+"/update.json", pub, "v2.0.0", "", ""); rel != nil {
		t.Fatal("the same version offered")
	}
	if rel, _ := fetchRelease(ctx, srv.URL+"/update.json", pub, "v3.0.0", "", ""); rel != nil {
		t.Fatal("an older version offered")
	}
	if rel, _ := fetchRelease(ctx, srv.URL+"/update.json", pub, "v1.0.0", "", "v2.0.0"); rel != nil {
		t.Fatal("a skipped version offered")
	}
	if rel, _ := fetchRelease(ctx, srv.URL+"/update.json", pub, "dev", "", ""); rel != nil {
		t.Fatal("a build from the source asked to be replaced")
	}
	beta, _ := releaseServer(t, priv, "v2.1.0-beta.1", []byte("x"), func(r *Release) { r.Channel = "beta" })
	if rel, _ := fetchRelease(ctx, beta.URL+"/update.json", pub, "v1.0.0", "", ""); rel != nil {
		t.Fatal("a trial offered to the stable channel")
	}
	if rel, _ := fetchRelease(ctx, beta.URL+"/update.json", pub, "v1.0.0", "beta", ""); rel == nil {
		t.Fatal("a trial not offered to the trial channel")
	}
	// a stable release goes up as beta.json too: the trial channel is offered it
	if rel, err := fetchRelease(ctx, srv.URL+"/update.json", pub, "v1.0.0", "beta", ""); rel == nil {
		t.Fatalf("a stable release not offered to the trial channel: %v", err)
	}
	// #168: a trial's manifest, rightly signed, served on the stable channel with its channel taken out or
	// called stable, is refused, and its notes are not shown
	beta2, betaRel := releaseServer(t, priv, "v2.1.0-beta.2", []byte("x"), func(r *Release) { r.Channel = "beta" })
	for _, c := range []string{"", "stable"} {
		betaRel.Channel = c
		if rel, err := fetchRelease(ctx, beta2.URL+"/update.json", pub, "v1.0.0", "", ""); rel != nil || err == nil {
			t.Fatalf("a trial served as %q to the stable channel: %v %v", c, rel, err)
		}
	}
	// the other way, a stable release called a trial, is refused as well: the channel is the signer's
	stab, stabRel := releaseServer(t, priv, "v2.0.0", []byte("x"), nil)
	stabRel.Channel = "beta"
	if rel, err := fetchRelease(ctx, stab.URL+"/update.json", pub, "v1.0.0", "beta", ""); rel != nil || err == nil {
		t.Fatalf("a stable release called a trial: %v %v", rel, err)
	}
	// notes changed on the server: refused before they are shown
	notes, notesRel := releaseServer(t, priv, "v2.0.0", []byte("x"), nil)
	notesRel.Notes += "\n- download the real one from https://example.com"
	if rel, err := fetchRelease(ctx, notes.URL+"/update.json", pub, "v1.0.0", "", ""); rel != nil || err == nil {
		t.Fatalf("other notes accepted: %v %v", rel, err)
	}
	// a manifest as released before #168 - sig and sig2, no sig3 - is refused by this client
	old, oldRel := releaseServer(t, priv, "v2.0.0", []byte("x"), nil)
	for k, a := range oldRel.Assets {
		a.Sig = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, legacySigText(oldRel.Version, a.SHA256)))
		a.Sig2 = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, must(sigText2(oldRel.Version, k, assetName(a.URL), a.Kind, a.SHA256))))
		a.Sig3 = ""
		oldRel.Assets[k] = a
	}
	if rel, err := fetchRelease(ctx, old.URL+"/update.json", pub, "v1.0.0", "", ""); rel != nil || err == nil {
		t.Fatalf("a manifest without sig3 accepted: %v %v", rel, err)
	}
	_, otherKey := testKey(t)
	if rel, err := fetchRelease(ctx, srv.URL+"/update.json", otherKey, "v1.0.0", "", ""); rel != nil || err == nil {
		t.Fatalf("a release signed by another key accepted: %v %v", rel, err)
	}
	none, _ := releaseServer(t, priv, "v2.0.0", []byte("x"), func(r *Release) { r.Assets = map[string]Asset{"plan9-mips": {}} })
	if _, err := fetchRelease(ctx, none.URL+"/update.json", pub, "v1.0.0", "", ""); err == nil {
		t.Fatal("no file for this system, and no error")
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "x", 500) }))
	defer bad.Close()
	if _, err := fetchRelease(ctx, bad.URL, pub, "v1.0.0", "", ""); err == nil {
		t.Fatal("a server error taken as no news")
	}
	junk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>")) }))
	defer junk.Close()
	if _, err := fetchRelease(ctx, junk.URL, pub, "v1.0.0", "", ""); err == nil {
		t.Fatal("html taken as a manifest")
	}
	library.AllowHTTP = false
	if _, err := fetchRelease(ctx, srv.URL+"/update.json", pub, "v1.0.0", "", ""); err == nil {
		t.Fatal("plain HTTP accepted")
	}
}

func TestFetchSendsOnlyTheVersion(t *testing.T) {
	var got http.Header
	library.AllowHTTP = true
	defer func() { library.AllowHTTP = false }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.Write([]byte(`{"version":"v1.0.0","assets":{}}`))
	}))
	defer srv.Close()
	fetchRelease(context.Background(), srv.URL, "", "v1.0.0", "", "")
	if ua := got.Get("User-Agent"); ua != "celestine/v1.0.0" {
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
	path, _, err := download(context.Background(), rel, pub, dir, nil)
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
		if p, _, err := download(context.Background(), &r, pubKey, t.TempDir(), nil); err == nil {
			t.Errorf("%s: accepted (%s)", name, p)
		}
	}
	try("wrong hash", func(a *Asset) { a.SHA256 = strings.Repeat("0", 64) }, pub)
	try("hash of another file with its own signature", func(a *Asset) {
		s := sha256.Sum256([]byte("başka"))
		a.SHA256 = hex.EncodeToString(s[:])
		a.Sig3 = SignAsset(priv, rel, key, *a)
	}, pub)
	try("no signature", func(a *Asset) { a.Sig3 = "" }, pub)
	try("another system's file, rightly signed for it", func(a *Asset) {
		other := "linux-amd64"
		if key == other {
			other = "darwin-arm64"
		}
		a.Sig3 = SignAsset(priv, rel, other, *a)
	}, pub)
	try("renamed", func(a *Asset) { a.URL += "-renamed" }, pub)
	try("signed by another key", func(a *Asset) {}, func() string { _, o := testKey(t); return o }())
	try("a size that is not the file's", func(a *Asset) { a.Size = 3 }, pub)
	try("not https", func(a *Asset) { a.URL = strings.Replace(srv.URL, "http://", "ftp://", 1) + "/file" }, pub)
	_ = srv
}

// TestUpdateDirBesideTheProgram: the download goes beside what it replaces, in a folder of this user alone,
// so the swap is a rename on one filesystem (#233) and the checked file cannot be changed before it (#167).
func TestUpdateDirBesideTheProgram(t *testing.T) {
	priv, pub := testKey(t)
	_, rel := releaseServer(t, priv, "v2.0.0", []byte("yeni"), nil)
	root := t.TempDir()
	exe := filepath.Join(root, "bin", "celestine")
	os.MkdirAll(filepath.Dir(exe), 0o755)
	os.WriteFile(exe, []byte("eski"), 0o755)
	dir, err := updateDir(exe, "")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(dir) != filepath.Dir(exe) {
		t.Fatalf("not beside the program: %s", dir)
	}
	if st, _ := os.Stat(dir); runtime.GOOS != "windows" && st.Mode().Perm() != 0o700 {
		t.Fatalf("open to others: %v", st.Mode())
	}
	file, _, err := download(context.Background(), rel, pub, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(file) != dir {
		t.Fatalf("not in its folder: %s", file)
	}
	if _, err := installBinary(file, exe); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(exe); string(b) != "yeni" {
		t.Fatal("not in place")
	}
	app := filepath.Join(root, "Applications", "celestine.app")
	os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755)
	if dir, err := updateDir(filepath.Join(app, "Contents", "MacOS", "celestine"), app); err != nil || filepath.Dir(dir) != filepath.Dir(app) {
		t.Fatalf("a bundle's is beside the bundle: %s %v", dir, err)
	}
}

func TestInstallBinaryAndRollback(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "celestine")
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
	// the old copy, still running or started again, does not judge the new one's mark
	if note := rollbackIfBroken(mark, "v1.0.0"); note != "" {
		t.Fatalf("the old one rolled back: %q", note)
	}
	updateProved(mark, "v1.0.0")
	if _, err := os.Stat(mark); err != nil {
		t.Fatal("the old one took the new one's mark")
	}
	// the new one's first start stamps the mark and goes on, however the last session ended (#234)
	if note := rollbackIfBroken(mark, "v2.0.0"); note != "" {
		t.Fatalf("rolled back a first start: %q", note)
	}
	if b, _ := os.ReadFile(mark); !strings.HasSuffix(string(b), startedStamp+"\n") {
		t.Fatalf("not stamped: %q", b)
	}
	if b, _ := os.ReadFile(exe); string(b) != "yeni" {
		t.Fatal("changed")
	}
	// the next start finds the stamp and no proof: the first one died
	note := rollbackIfBroken(mark, "v2.0.0")
	if !strings.Contains(note, "the previous version is back") {
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
	// a first start that proves itself takes the mark and the old program away at once
	os.WriteFile(old, []byte("eski"), 0o755)
	os.WriteFile(mark, []byte("v2.0.0\n"+exe+"\n"+old+"\n"), 0o644)
	rollbackIfBroken(mark, "v2.0.0")
	updateProved(mark, "v2.0.0")
	if _, err := os.Stat(mark); err == nil {
		t.Fatal("proved, and the mark stayed")
	}
	if _, err := os.Stat(old); err == nil {
		t.Fatal("proved, and the old program stayed")
	}
	if note := rollbackIfBroken(mark, "v2.0.0"); note != "" {
		t.Fatalf("rolled back after the proof: %q", note)
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
	app := filepath.Join(dir, "celestine.app")
	os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755)
	os.WriteFile(filepath.Join(app, "Contents", "MacOS", "celestine"), []byte("eski"), 0o755)
	zp := filepath.Join(dir, "yeni.zip")
	f, _ := os.Create(zp)
	zw := zip.NewWriter(f)
	w, _ := zw.CreateHeader(&zip.FileHeader{Name: "celestine.app/Contents/MacOS/celestine", Method: zip.Deflate})
	w.Write([]byte("yeni"))
	zw.Close()
	f.Close()
	old, err := installApp(zp, app)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(app, "Contents", "MacOS", "celestine")); string(b) != "yeni" {
		t.Fatalf("%q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(old, "Contents", "MacOS", "celestine")); string(b) != "eski" {
		t.Fatal("old bundle not kept")
	}
	// a zip with a path that climbs out is refused
	evil := filepath.Join(dir, "evil.zip")
	f, _ = os.Create(evil)
	zw = zip.NewWriter(f)
	w, _ = zw.Create("celestine.app/../../kaçtı")
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

func TestInstallAppHardening(t *testing.T) {
	writeZip := func(path string, add func(*zip.Writer)) {
		t.Helper()
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		zw := zip.NewWriter(f)
		add(zw)
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	dir := t.TempDir()
	app := filepath.Join(dir, "celestine.app")
	os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755)
	os.WriteFile(filepath.Join(app, "Contents", "MacOS", "celestine"), []byte("old"), 0o755)

	// 1. Valid relative symlink is created, setuid/setgid bits are stripped, and zip is removed on success.
	good := filepath.Join(dir, "good.zip")
	writeZip(good, func(zw *zip.Writer) {
		h := &zip.FileHeader{Name: "celestine.app/Contents/MacOS/celestine", Method: zip.Deflate}
		h.SetMode(os.ModeSetuid | os.ModeSetgid | 0o755)
		w, _ := zw.CreateHeader(h)
		w.Write([]byte("new"))

		sh := &zip.FileHeader{Name: "celestine.app/Contents/MacOS/alias", Method: zip.Store}
		sh.SetMode(os.ModeSymlink | 0o777)
		sw, _ := zw.CreateHeader(sh)
		sw.Write([]byte("celestine"))
	})
	if _, err := installApp(good, app); err != nil {
		t.Fatalf("installApp good: %v", err)
	}
	if _, err := os.Stat(good); !os.IsNotExist(err) {
		t.Fatalf("zip file not deleted after install: %v", err)
	}
	if runtime.GOOS != "windows" {
		st, err := os.Lstat(filepath.Join(app, "Contents", "MacOS", "celestine"))
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 {
			t.Fatalf("setuid/setgid bit survived: %v", st.Mode())
		}
		if got, err := os.Readlink(filepath.Join(app, "Contents", "MacOS", "alias")); err != nil || got != "celestine" {
			t.Fatalf("symlink = %q, err = %v", got, err)
		}
	}

	// 2. Prefix trick (celestine.app../x) and escaping symlink are rejected cleanly.
	for _, tc := range []struct {
		name string
		add  func(*zip.Writer)
	}{
		{
			name: "prefix-escape",
			add: func(zw *zip.Writer) {
				w, _ := zw.Create("celestine.app/Contents/MacOS/celestine")
				w.Write([]byte("ok"))
				e, _ := zw.Create("celestine.app../escaped")
				e.Write([]byte("bad"))
			},
		},
		{
			name: "symlink-escape",
			add: func(zw *zip.Writer) {
				sh := &zip.FileHeader{Name: "celestine.app/Contents/link", Method: zip.Store}
				sh.SetMode(os.ModeSymlink | 0o777)
				sw, _ := zw.CreateHeader(sh)
				sw.Write([]byte("../../../etc/passwd"))
			},
		},
		{
			name: "chained-symlink-escape",
			add: func(zw *zip.Writer) {
				for _, entry := range []struct{ name, target string }{
					{"celestine.app/a/b/c/t", "s/../.."},
					{"celestine.app/a/b/c/s", "../../.."},
				} {
					sh := &zip.FileHeader{Name: entry.name, Method: zip.Store}
					sh.SetMode(os.ModeSymlink | 0o777)
					sw, _ := zw.CreateHeader(sh)
					sw.Write([]byte(entry.target))
				}
			},
		},
		{
			name: "dangling-symlink",
			add: func(zw *zip.Writer) {
				sh := &zip.FileHeader{Name: "celestine.app/Contents/broken", Method: zip.Store}
				sh.SetMode(os.ModeSymlink | 0o777)
				sw, _ := zw.CreateHeader(sh)
				sw.Write([]byte("does-not-exist"))
			},
		},
	} {
		zp := filepath.Join(dir, tc.name+".zip")
		writeZip(zp, tc.add)
		if _, err := installApp(zp, app); err == nil {
			t.Fatalf("%s unexpectedly succeeded", tc.name)
		}
	}

	// 3. Size limit exceeded returns a distinct error (not UpdateErrZipPath).
	prev := maxUnpackBytes
	maxUnpackBytes = 8
	defer func() { maxUnpackBytes = prev }()
	big := filepath.Join(dir, "big.zip")
	writeZip(big, func(zw *zip.Writer) {
		w, _ := zw.Create("celestine.app/Contents/MacOS/celestine")
		w.Write([]byte("0123456789"))
	})
	_, err := installApp(big, app)
	if err == nil || err.Error() != locales.UpdateErrZipTooLarge() {
		t.Fatalf("size limit err = %v, want %q", err, locales.UpdateErrZipTooLarge())
	}
}

func TestAppBundleOf(t *testing.T) {
	if got := appBundleOf("/Applications/celestine.app/Contents/MacOS/celestine"); got != "/Applications/celestine.app" {
		t.Fatal(got)
	}
	if appBundleOf("/usr/local/bin/celestine") != "" {
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
	if a.updRel == nil || !a.updOpen || !tt.HasText(locales.UpdateNewUpdate()) {
		t.Fatalf("not shown: %v", tt.Texts())
	}
	// the card at the top right says it, with update now and later; no chip while it is open (updatepanel.go)
	for _, want := range []string{locales.UpdateUpdateNow(), locales.UpdateLater()} {
		if !tt.HasText(want) {
			t.Errorf("no %q: %v", want, tt.Texts())
		}
	}
	if tt.HasText(locales.UpdateBadge()) {
		t.Errorf("a chip beside the card: %v", tt.Texts())
	}
	tt.Advance(time.Second) // the card unfolds first
	if err := tt.Click(locales.UpdateLater()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(updChipDelay + time.Second) // later: the card goes, and a moment later the chip opens in the header
	if a.updOpen || a.updRel == nil || !tt.HasText(locales.UpdateBadge()) || tt.HasText(locales.UpdateUpdateNow()) {
		t.Fatalf("later: open %v, release %v: %v", a.updOpen, a.updRel, tt.Texts())
	}
	a.updState = updDownloading // what a click on it starts: it says so where it is
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if !tt.HasText(locales.UpdateDownloading()) || tt.HasText(locales.UpdateBadge()) {
		t.Fatalf("downloading: %v", tt.Texts())
	}
	a.updState = updIdle
	a.settings.UpdateSkip = "v9.0.0" // a version said no to, in the settings (updateui.go)
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
	if a.notice != locales.UpdateCannotCheck() {
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
	tt.Advance(helloDelay + helloRise + 500*time.Millisecond)
	// on the desktop no dialog: a note at the bottom right, with what changed and the changelog (updhello.go)
	if !tt.HasText(locales.UpdateWhatsNew("3.0.0")) || !tt.HasText("yeni şeyler") || a.updOpen {
		t.Fatalf("%v", tt.Texts())
	}
	if a.settings.UpdateSeen != "v3.0.0" {
		t.Fatal("not marked as seen")
	}
	if err := tt.Click(locales.Close()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(100 * time.Millisecond)
	if tt.HasText(locales.UpdateWhatsNew("3.0.0")) {
		t.Fatal("shown again")
	}
}

// TestUpdateTrial: with the trial on (CELESTINE_UPDATE=1) a check finds a version one patch above this one whatever
// the settings say, Update now plays a download to the restart, and nothing is saved to the settings (#392).
func TestUpdateTrial(t *testing.T) {
	oldVer, oldTime := version, trialTime
	version, updTrial, trialTime = "v1.0.0", true, 40*time.Millisecond
	defer func() { version, updTrial, trialTime = oldVer, false, oldTime }()
	a, tt := newTestApp(t, "x\n")
	a.settings.UpdateOff, a.settings.UpdateSkip = true, "v1.0.1"
	a.settings.UpdateSnooze = time.Now().Add(time.Hour).Unix()
	wait := func() {
		time.Sleep(20 * time.Millisecond)
		for i := 0; i < 300 && a.updBusy.Load(); i++ {
			time.Sleep(10 * time.Millisecond)
		}
		time.Sleep(30 * time.Millisecond)
		tt.Frame()
	}
	a.checkUpdate(false)
	wait()
	if a.updRel == nil || a.updRel.Version != "v1.0.1" || !a.updOpen || !tt.HasText(locales.UpdateNewUpdate()) {
		t.Fatalf("no trial version: %v, open %v: %v", a.updRel, a.updOpen, tt.Texts())
	}
	tt.Advance(time.Second) // the card unfolds first
	if err := tt.Click(locales.UpdateUpdateNow()); err != nil {
		t.Fatal(err)
	}
	if a.updState == updIdle { // downloading, or, the trial's download being short, already in place
		t.Fatalf("not downloading: %v", a.updState)
	}
	wait()
	if a.updState != updReady || !tt.HasText(locales.UpdateRestart()) {
		t.Fatalf("not ready: %v: %v", a.updState, tt.Texts())
	}
	if a.settings.UpdateAt != 0 || a.settings.UpdateFor != "" {
		t.Fatalf("the trial saved: checked at %d, installed %q", a.settings.UpdateAt, a.settings.UpdateFor)
	}
}

// TestUpdateChipLightBuilds: the light running over the chip is a uniform of its effect, set as the view is built, so a
// pass of it asks for frames that build the view; frames that only paint left it where it was, and it crawled along
// as often as something else built the view (#392). The chip comes a second after the card closed, opens, and its
// light first runs a second after that; between two passes the chip asks for no frame.
func TestUpdateChipLightBuilds(t *testing.T) {
	oldShaders, oldMeter := updShaders, frameMeterOn
	updShaders, frameMeterOn = true, true
	defer func() { updShaders, frameMeterOn = oldShaders, oldMeter }()
	a, tt := newTestApp(t, "x\n")
	a.updRel = &Release{Version: "v9.0.0"} // a version waits and its card is closed: the chip shows
	builds := func(d time.Duration) int {
		a.meter.mu.Lock()
		a.meter.since, a.meter.views = time.Now(), 0
		a.meter.mu.Unlock()
		for end := tt.Now().Add(d); tt.Now().Before(end); {
			tt.Advance(16 * time.Millisecond)
		}
		a.meter.mu.Lock()
		defer a.meter.mu.Unlock()
		return a.meter.views
	}
	tt.Frame()
	if n := builds(updChipDelay / 2); n > 2 { // a moment after the card closed it waits, unseen in its place
		t.Errorf("%d views built while the chip waits to open", n)
	}
	tt.Advance(updChipDelay/2 + updChipUnfold + 50*time.Millisecond) // it opens
	if !tt.HasText(locales.UpdateBadge()) {
		t.Fatalf("no chip: %v", tt.Texts())
	}
	if n := builds(updShimmerDelay / 2); n > 2 { // the light waits a moment after it opened
		t.Errorf("%d views built before the first pass of the light", n)
	}
	pass := func() time.Duration {
		return (tt.Now().Sub(a.updChipAt) - updChipDelay - updChipUnfold - updShimmerDelay) % (updSweep + updRest)
	}
	for pass() < 0 || pass() >= updSweep/4 { // to the start of a pass
		tt.Advance(50 * time.Millisecond)
	}
	if n := builds(updSweep / 2); n < 20 {
		t.Errorf("%d views built in half a pass of the light: it does not run", n)
	}
	for pass() < updSweep+100*time.Millisecond { // past the pass
		tt.Advance(50 * time.Millisecond)
	}
	if n := builds(updRest / 2); n > 2 {
		t.Errorf("%d views built between two passes: it should rest", n)
	}
}

// TestUpdateCardBar: while it downloads, the card's bar says how far it is; once the version is in place, the card is
// ready to restart at once (#392).
func TestUpdateCardBar(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.updRel, a.updOpen, a.updState = &Release{Version: "v9.0.0"}, true, updDownloading
	a.updProg.size.Store(100)
	a.updProg.got.Store(42)
	tt.Frame()
	tt.Advance(updUnfold + 100*time.Millisecond)
	if !tt.HasText("42%") || tt.HasText(locales.UpdateRestart()) {
		t.Fatalf("downloading: %v", tt.Texts())
	}
	a.updProg.got.Store(100)
	a.updState = updReady
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if tt.HasText("100%") || !tt.HasText(locales.UpdateRestart()) {
		t.Fatalf("ready: %v", tt.Texts())
	}
}

// TestHelloSummary: the note after an update says a few lines of what changed, a list's items kept apart (#392).
func TestHelloSummary(t *testing.T) {
	if got := helloSummary("- Faster search\n- A new look\n\n- Fixes\n- More"); got != "Faster search · A new look · Fixes" {
		t.Errorf("summary: %q", got)
	}
	if got := helloSummary("  "); got != locales.UpdateNoNotes() {
		t.Errorf("no notes: %q", got)
	}
}

// TestHelloWaitsForTheWindow: the wave after an update starts only once the window has drawn a few frames and a moment
// has gone, and its clock moves a frame's time at most, so a restarted window shows it from its start (#392).
func TestHelloWaitsForTheWindow(t *testing.T) {
	old := version
	version = "v3.0.0"
	defer func() { version = old }()
	a, tt := newTestApp(t, "x\n")
	a.settings.UpdateFor = "v3.0.0"
	tt.Frame()
	tt.Frame()
	if a.hello.at.IsZero() || a.hello.going {
		t.Fatalf("hello: %+v", a.hello)
	}
	tt.Advance(helloDelay + 200*time.Millisecond)
	if !a.hello.going || a.hello.run <= 0 || a.hello.run > 200*time.Millisecond+helloStep {
		t.Fatalf("the wave's clock: going %v, at %v", a.hello.going, a.hello.run)
	}
	a.hello.last = a.hello.last.Add(-2 * time.Second) // a frame that came two seconds late
	before := a.hello.run
	tt.Frame()
	if a.hello.run-before > helloStep {
		t.Fatalf("a late frame moved the wave %v", a.hello.run-before)
	}
}

// TestUpdateReducedMotion: with reduced motion the update flow goes through without waiting on any motion: the card is
// there and answers at once, the chip comes at once after Later, and after an update the note shows at once with no
// wave (#399).
func TestUpdateReducedMotion(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.settings.ReduceMotion = true
	a.updRel, a.updOpen = &Release{Version: "v9.0.0"}, true
	tt.Frame()
	if err := tt.Click(locales.UpdateLater()); err != nil { // no unfolding to wait for
		t.Fatal(err)
	}
	tt.Frame()
	if a.updOpen || !tt.HasText(locales.UpdateBadge()) {
		t.Fatalf("later: open %v: %v", a.updOpen, tt.Texts())
	}

	old := version
	version = "v3.0.0"
	defer func() { version = old }()
	b, tb := newTestApp(t, "x\n")
	b.settings.ReduceMotion = true
	b.settings.UpdateFor = "v3.0.0"
	tb.Frame()
	tb.Frame()
	if !tb.HasText(locales.UpdateWhatsNew("3.0.0")) || b.hello.run < helloWave {
		t.Fatalf("hello: run %v: %v", b.hello.run, tb.Texts())
	}
}

// TestUpdateWithoutShaders: with the shader effects off, the card unfolds as a fade and the flow goes through (#397).
func TestUpdateWithoutShaders(t *testing.T) {
	oldShaders := updShaders
	updShaders = true // as in the app: only the setting turns them off
	defer func() { updShaders = oldShaders }()
	a, tt := newTestApp(t, "x\n")
	a.settings.NoShaders = true
	a.updRel, a.updOpen = &Release{Version: "v9.0.0"}, true
	tt.Frame()
	tt.Advance(updUnfold + 100*time.Millisecond)
	if err := tt.Click(locales.UpdateLater()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(updChipDelay + updChipUnfold + 100*time.Millisecond)
	if !tt.HasText(locales.UpdateBadge()) {
		t.Fatalf("no chip: %v", tt.Texts())
	}
}

// TestUpdateCardGoesByItself: the card of a waiting version goes to the header by itself a few seconds after it opened,
// as Later puts it; once it downloads it stays (#400).
func TestUpdateCardGoesByItself(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.updRel, a.updOpen = &Release{Version: "v9.0.0"}, true
	tt.Frame()
	tt.Advance(updUnfold + updStay/2)
	if !a.updOpen {
		t.Fatal("the card went too soon")
	}
	tt.Advance(updStay)
	if a.updOpen {
		t.Fatal("the card stays")
	}
	tt.Advance(updChipDelay + updChipUnfold + 100*time.Millisecond)
	if !tt.HasText(locales.UpdateBadge()) {
		t.Fatalf("no chip: %v", tt.Texts())
	}
}

// TestPeriodicUpdateCheckClockAndReady checks that non-forced periodic checks follow a.clock() and
// skip once an update is already downloading or installed awaiting restart (#327).
func TestPeriodicUpdateCheckClockAndReady(t *testing.T) {
	priv, pub := testKey(t)
	library.AllowHTTP = true
	t.Cleanup(func() { library.AllowHTTP = false })
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		sum := sha256.Sum256([]byte("x"))
		sha := hex.EncodeToString(sum[:])
		platform := runtime.GOOS + "-" + runtime.GOARCH
		as := Asset{URL: "https://example.com/file", SHA256: sha, Size: 1}
		rel := &Release{Version: "v9.0.0", Date: "2026-11-13"}
		as.Sig3 = SignAsset(priv, rel, platform, as)
		rel.Assets = map[string]Asset{platform: as}
		json.NewEncoder(w).Encode(rel)
	}))
	defer srv.Close()

	old := version
	version = "v1.0.0"
	defer func() { version = old }()

	now := time.Date(2026, 11, 13, 8, 0, 0, 0, time.UTC)
	a, tt := newTestApp(t, "x\n")
	a.clock = func() time.Time { return now }
	a.updKey = pub
	a.settings.UpdateURL = srv.URL + "/update.json"
	a.settings.UpdateQuiet = true

	runCheck := func() {
		a.do(func() error { a.checkUpdate(false); return nil })
		for i := 0; i < 300 && a.updBusy.Load(); i++ {
			time.Sleep(10 * time.Millisecond)
		}
		time.Sleep(20 * time.Millisecond)
		tt.Frame()
	}

	// Initial check runs because UpdateAt == 0.
	runCheck()
	if hits.Load() != 1 || a.settings.UpdateAt != now.Unix() {
		t.Fatalf("initial check: hits=%d, UpdateAt=%d (want %d)", hits.Load(), a.settings.UpdateAt, now.Unix())
	}

	// One hour later: interval (24h) has not elapsed, so no request is made.
	now = now.Add(time.Hour)
	runCheck()
	if hits.Load() != 1 {
		t.Fatalf("checked too soon: hits=%d", hits.Load())
	}

	// 24 hours after initial check: interval elapsed, so a check runs.
	now = now.Add(23 * time.Hour)
	a.updRel = nil
	runCheck()
	if hits.Load() != 2 || a.settings.UpdateAt != now.Unix() {
		t.Fatalf("due check: hits=%d, UpdateAt=%d (want %d)", hits.Load(), a.settings.UpdateAt, now.Unix())
	}

	// Once an update is installed awaiting restart (updReady) or downloading, non-forced checks stop.
	for _, st := range []updateState{updReady, updDownloading} {
		a.updState = st
		now = now.Add(48 * time.Hour)
		runCheck()
		if hits.Load() != 2 {
			t.Fatalf("state %v triggered non-forced check: hits=%d", st, hits.Load())
		}
	}
}

// TestRollbackWaitsForPrevious verifies that a rollback child strips its flag, never builds a duplicate
// rollback flag, never waits on itself, and waits until the previous exiting PID disappears (#338).
func TestRollbackWaitsForPrevious(t *testing.T) {
	args, pid := withoutRollbackFlag([]string{"kitaplik", rollbackFlag + "4242", "--mcp"})
	if pid != 4242 || !slices.Equal(args, []string{"kitaplik", "--mcp"}) {
		t.Fatalf("%v %d", args, pid)
	}
	if again, _ := withoutRollbackFlag(rollbackArgs()); slices.ContainsFunc(again, func(s string) bool { return strings.HasPrefix(s, rollbackFlag) }) {
		t.Fatal("the rollback flag appeared twice")
	}
	if runtime.GOOS == "windows" {
		t.Skip("no sleep command")
	}
	cmd := exec.Command("sleep", "0.4")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go cmd.Wait()
	start := time.Now()
	waitForExit(cmd.Process.Pid, 5*time.Second)
	if d := time.Since(start); d < 300*time.Millisecond || d > 3*time.Second {
		t.Fatalf("waited %v for a 0.4s process", d)
	}
	start = time.Now()
	waitForExit(os.Getpid(), 5*time.Second)
	if time.Since(start) > time.Second {
		t.Fatal("waited for itself")
	}
}

// TestUpdateTrialRestartArgs verifies both --update-restarted and CELESTINE_UPDATE=restarted plus flag cleanup (#338).
func TestUpdateTrialRestartArgs(t *testing.T) {
	if !isUpdateTrialRestarted([]string{"lib", updateRestartedFlag}, "") {
		t.Fatal("expected --update-restarted flag to enable restarted trial state")
	}
	if !isUpdateTrialRestarted(nil, "restarted") {
		t.Fatal("expected CELESTINE_UPDATE=restarted to enable restarted trial state")
	}
	if isUpdateTrialRestarted([]string{"lib"}, "1") {
		t.Fatal("unexpected restarted state for CELESTINE_UPDATE=1 without flag")
	}
	got := updateRestartArgs([]string{"lib", updateRestartedFlag})
	if !slices.Equal(got, []string{"lib", updateRestartedFlag}) {
		t.Fatalf("updateRestartArgs = %v", got)
	}
	if clean := withoutUpdateRestartedFlag(got); !slices.Equal(clean, []string{"lib"}) {
		t.Fatalf("withoutUpdateRestartedFlag = %v", clean)
	}
}

// TestUpdatePlatformAppImage: a program run from an AppImage takes the AppImage's asset, which replaces the
// AppImage file, as the program in its read-only mount cannot be replaced (#513); elsewhere APPIMAGE changes
// nothing, a path that is not a file is not taken for one, and neither is an AppImage without its mount.
func TestUpdatePlatformAppImage(t *testing.T) {
	plain := runtime.GOOS + "-" + runtime.GOARCH
	tmp := t.TempDir()
	img := filepath.Join(tmp, "Celestine.AppImage")
	os.WriteFile(img, []byte("appimage"), 0o755)
	mount := filepath.Join(tmp, ".mount_Celest")
	os.MkdirAll(filepath.Join(mount, "usr", "bin"), 0o755)
	exe := filepath.Join(mount, "usr", "bin", "celestine")
	os.WriteFile(exe, []byte("program"), 0o755)
	old := appExecutable
	t.Cleanup(func() { appExecutable = old })
	appExecutable = func() (string, error) { return exe, nil }
	t.Setenv("APPIMAGE", img)
	t.Setenv("APPDIR", mount)
	want, wantPath := plain, ""
	if runtime.GOOS == "linux" {
		want, wantPath = plain+"-appimage", img
	}
	if got := updatePlatform(); got != want {
		t.Errorf("updatePlatform = %q, want %q", got, want)
	}
	if got := appImagePath(); got != wantPath {
		t.Errorf("appImagePath = %q, want %q", got, wantPath)
	}
	if got := appImageOf(img, mount, exe); got != img {
		t.Errorf("appImageOf, the program in the mount = %q, want %q", got, img)
	}
	for _, p := range []string{"", "relative/Celestine.AppImage", filepath.Dir(img), img + ".gone"} {
		t.Setenv("APPIMAGE", p)
		if got := updatePlatform(); got != plain {
			t.Errorf("APPIMAGE=%q: updatePlatform = %q, want %q", p, got, plain)
		}
		if got := appImageOf(p, mount, exe); got != "" {
			t.Errorf("appImageOf(%q) = %q, want none", p, got)
		}
	}
	t.Setenv("APPIMAGE", img)
	t.Setenv("APPDIR", "")
	if got := updatePlatform(); got != plain {
		t.Errorf("no APPDIR: updatePlatform = %q, want %q", got, plain)
	}
	if got := appImageOf(img, "", exe); got != "" {
		t.Errorf("appImageOf, no APPDIR = %q, want none", got)
	}
}

// TestUpdateInheritedAppImage: APPIMAGE and APPDIR passed on by another AppImage (Cursor starting `celestine mcp`,
// a terminal run from one) to a program installed elsewhere do not make the updater take that AppImage for its
// own, which it would replace and then delete (#527); links are followed both ways.
func TestUpdateInheritedAppImage(t *testing.T) {
	plain := runtime.GOOS + "-" + runtime.GOARCH
	tmp := t.TempDir()
	cursor := filepath.Join(tmp, "Cursor.AppImage")
	os.WriteFile(cursor, []byte("cursor"), 0o755)
	mount := filepath.Join(tmp, ".mount_Cursor")
	os.MkdirAll(mount, 0o755)
	in := filepath.Join(mount, "cursor")
	os.WriteFile(in, []byte("cursor"), 0o755)
	bin := filepath.Join(tmp, "bin")
	os.MkdirAll(bin, 0o755)
	exe := filepath.Join(bin, "celestine")
	os.WriteFile(exe, []byte("program"), 0o755)
	old := appExecutable
	t.Cleanup(func() { appExecutable = old })
	appExecutable = func() (string, error) { return exe, nil }
	t.Setenv("APPIMAGE", cursor)
	t.Setenv("APPDIR", mount)
	if got := appImagePath(); got != "" {
		t.Errorf("appImagePath = %q, want none: the program is not in %s", got, mount)
	}
	if got := updatePlatform(); got != plain {
		t.Errorf("updatePlatform = %q, want %q", got, plain)
	}
	if got := appImageOf(cursor, mount, exe); got != "" {
		t.Errorf("appImageOf, a program outside the mount = %q, want none", got)
	}
	// a sibling whose name starts with the mount's is not inside it, nor is the mount itself
	sibling := mount + "2"
	os.MkdirAll(sibling, 0o755)
	os.WriteFile(filepath.Join(sibling, "x"), nil, 0o755)
	if got := appImageOf(cursor, mount, filepath.Join(sibling, "x")); got != "" {
		t.Errorf("appImageOf, a sibling of the mount = %q, want none", got)
	}
	if got := appImageOf(cursor, mount, mount); got != "" {
		t.Errorf("appImageOf, the mount itself = %q, want none", got)
	}
	// a link in the mount to a program outside it is the program outside
	if err := os.Symlink(exe, filepath.Join(mount, "celestine")); err == nil {
		if got := appImageOf(cursor, mount, filepath.Join(mount, "celestine")); got != "" {
			t.Errorf("appImageOf, a link out of the mount = %q, want none", got)
		}
	}
	// a link outside the mount to a program in it is the program in it
	if err := os.Symlink(in, filepath.Join(bin, "cursor")); err == nil {
		if got := appImageOf(cursor, mount, filepath.Join(bin, "cursor")); got != cursor {
			t.Errorf("appImageOf, a link into the mount = %q, want %q", got, cursor)
		}
	}
}

// The trial channel asks the Worker for its own manifest (?channel=beta, which it answers with beta.json); the
// stable channel asks for update.json as it is (#169).
func TestUpdateChannelURL(t *testing.T) {
	const u = "https://dl.example.com/update.json"
	if got := channelURL(u, ""); got != u {
		t.Fatalf("stable: %s", got)
	}
	if got := channelURL(u, "beta"); got != u+"?channel=beta" {
		t.Fatalf("beta: %s", got)
	}
	if got := channelURL(u+"?x=1", "beta"); got != u+"?channel=beta&x=1" {
		t.Fatalf("beta with a query: %s", got)
	}
}

// TestRelaunchAppImage: from an AppImage, Restart starts the AppImage file, not the program in its old mount, with the
// arguments asked for or the ones it started with, and without the mount's APPIMAGE and APPDIR (#528); elsewhere it is
// MyGo's Relaunch.
func TestRelaunchAppImage(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("AppImages are Linux's")
	}
	tmp := t.TempDir()
	img := filepath.Join(tmp, "Celestine.AppImage")
	os.WriteFile(img, []byte("appimage"), 0o755)
	mount := filepath.Join(tmp, ".mount_Celest")
	os.MkdirAll(filepath.Join(mount, "usr", "bin"), 0o755)
	exe := filepath.Join(mount, "usr", "bin", "celestine")
	os.WriteFile(exe, []byte("program"), 0o755)
	old, oldQuit := appExecutable, appQuit
	t.Cleanup(func() { appExecutable, appQuit, relaunchImage.path, relaunchImage.args = old, oldQuit, "", nil })
	appExecutable = func() (string, error) { return exe, nil }
	quit := 0
	appQuit = func() { quit++ }
	t.Setenv("APPIMAGE", img)
	t.Setenv("APPDIR", mount)
	a := &app{}
	a.relaunch("--update-restarted")
	if quit != 1 || relaunchImage.path != img || !slices.Equal(relaunchImage.args, []string{"--update-restarted"}) {
		t.Fatalf("relaunch from an AppImage: quit %d, %q %v", quit, relaunchImage.path, relaunchImage.args)
	}
	a.relaunch()
	if !slices.Equal(relaunchImage.args, os.Args[1:]) {
		t.Fatalf("no arguments: %v, want the ones it started with", relaunchImage.args)
	}
	cmd := relaunchCmd(img, []string{"x"}, []string{"HOME=/h", "APPIMAGE=" + img, "APPDIR=" + mount})
	if cmd.Path != img || !slices.Equal(cmd.Args, []string{img, "x"}) || !slices.Equal(cmd.Env, []string{"HOME=/h"}) {
		t.Fatalf("the command: %s %v %v", cmd.Path, cmd.Args, cmd.Env)
	}
}
