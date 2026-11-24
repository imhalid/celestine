package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSumsText: the text the install script builds (scripts/install.sh: verify_sums) is the one signed.
func TestSumsText(t *testing.T) {
	got, err := sumsText("v1.3.0", "", []byte("ab  celestine-linux-amd64\n"))
	if want := "celestine-sums-1\nv1.3.0\nstable\nab  celestine-linux-amd64\n"; err != nil || string(got) != want {
		t.Fatalf("%q %v, want %q", got, err, want)
	}
	if got, _ := sumsText("v1.3.0-beta.1", "beta", nil); string(got) != "celestine-sums-1\nv1.3.0-beta.1\nbeta\n" {
		t.Fatalf("a trial's %q", got)
	}
	if _, err := sumsText("v1.3.0\nstable", "", nil); err == nil {
		t.Fatal("a line break was signed")
	}
	pub := make(ed25519.PublicKey, ed25519.PublicKeySize)
	if got := sshPublicKey(pub); got != "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" {
		t.Fatalf("ssh key %q", got)
	}
}

// fakeRelease is a release on disk as the update server serves it, for the install script: update.json, and in
// the version's folder the download for one system, SHA256SUMS and its signature (sign writes it).
type fakeRelease struct {
	dir, version, file string
	sums               []byte
}

// newFakeRelease writes a release of version whose program is build into a new folder, for goos on arm64: the
// zipped celestine.app for darwin, the .tar.gz for linux.
func newFakeRelease(t *testing.T, goos, version string, build []byte) *fakeRelease {
	t.Helper()
	r := &fakeRelease{dir: t.TempDir(), version: version}
	var buf bytes.Buffer
	switch goos {
	case "darwin":
		r.file = "celestine-darwin-arm64.zip"
		zw := zip.NewWriter(&buf)
		w, _ := zw.Create("celestine.app/Contents/MacOS/celestine")
		w.Write(build)
		w, _ = zw.Create("celestine.app/Contents/Info.plist")
		w.Write([]byte("<plist/>"))
		zw.Close()
	default:
		name := "celestine-linux-arm64"
		r.file = name + ".tar.gz"
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		for _, f := range []struct {
			name string
			body []byte
		}{
			{"celestine", build},
			{"celestine.png", []byte("png")},
			{"celestine.desktop", []byte("[Desktop Entry]\nName=Celestine\nExec=celestine %F\nIcon=celestine\n")},
		} {
			tw.WriteHeader(&tar.Header{Name: name + "/" + f.name, Mode: 0o755, Size: int64(len(f.body)), Typeflag: tar.TypeReg})
			tw.Write(f.body)
		}
		tw.Close()
		gz.Close()
	}
	sum := sha256.Sum256(buf.Bytes())
	r.sums = []byte(hex.EncodeToString(sum[:]) + "  " + r.file + "\n")
	os.MkdirAll(filepath.Join(r.dir, version), 0o755)
	os.WriteFile(filepath.Join(r.dir, version, r.file), buf.Bytes(), 0o644)
	os.WriteFile(filepath.Join(r.dir, version, "SHA256SUMS"), r.sums, 0o644)
	os.WriteFile(filepath.Join(r.dir, "update.json"), []byte("{\n  \"version\": \""+version+"\",\n  \"assets\": {}\n}\n"), 0o644)
	return r
}

// sign writes the signature of SHA256SUMS, made with priv for the version and channel given.
func (r *fakeRelease) sign(t *testing.T, priv ed25519.PrivateKey, version, channel string) {
	t.Helper()
	text, err := sumsText(version, channel, r.sums)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(r.dir, r.version, "SHA256SUMS.sig"), []byte(sshSign(priv, sumsNamespace, text)), 0o644)
}

// TestInstallScript runs scripts/install.sh against a release on disk, as macOS and as Linux on any of the two
// (a uname of its own says which): it installs what is signed with the key it is given, and refuses a SHA256SUMS
// changed after it was signed, signed by another key, for another version or for the trial channel, or not signed
// at all (#540). A home and a folder with a space, a "|" and a "&" in their names are where it installs.
func TestInstallScript(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the install script is for macOS and Linux")
	}
	for _, tool := range []string{"sh", "ssh-keygen", "curl"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("no " + tool)
		}
	}
	for _, goos := range []string{"darwin", "linux"} {
		t.Run(goos, func(t *testing.T) {
			if _, err := exec.LookPath("unzip"); err != nil && goos == "darwin" {
				t.Skip("no unzip")
			}
			testInstallScript(t, goos)
		})
	}
}

func testInstallScript(t *testing.T, goos string) {
	script, err := filepath.Abs("../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	shim := t.TempDir()
	uname := "Darwin"
	if goos == "linux" {
		uname = "Linux"
	}
	os.WriteFile(filepath.Join(shim, "uname"), []byte("#!/bin/sh\ncase $1 in -s) echo "+uname+" ;; *) echo arm64 ;; esac\n"), 0o755)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
	root := t.TempDir()
	home := filepath.Join(root, "home a|b&c")
	appDir := filepath.Join(root, "apps x|y&z")
	os.MkdirAll(home, 0o755)

	// extra is put after the environment: a PATH of its own, CELESTINE_UNVERIFIED; scriptAt another copy of the script
	var extra []string
	scriptAt := script
	run := func(r *fakeRelease, key string) (string, error) {
		cmd := exec.Command("sh", scriptAt)
		cmd.Env = append(os.Environ(), "CELESTINE_DL=file://"+r.dir, "CELESTINE_DIR="+appDir, "HOME="+home, "NO_COLOR=1",
			"XDG_DATA_HOME=", "XDG_BIN_HOME=", "CELESTINE_UNVERIFIED=", "PATH="+shim+string(os.PathListSeparator)+os.Getenv("PATH"))
		cmd.Env = append(cmd.Env, extra...)
		if key != "" {
			cmd.Env = append(cmd.Env, "CELESTINE_KEY="+key)
		} else {
			cmd.Env = append(cmd.Env, "CELESTINE_KEY=")
		}
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	installed := func() string {
		p := filepath.Join(appDir, "Celestine.app", "Contents", "MacOS", "celestine")
		if goos == "linux" {
			p = filepath.Join(appDir, "celestine")
		}
		b, _ := os.ReadFile(p)
		return string(b)
	}
	key := sshPublicKey(pub)

	good := newFakeRelease(t, goos, "v1.2.3", []byte("build one"))
	good.sign(t, priv, "v1.2.3", "")
	if out, err := run(good, key); err != nil || installed() != "build one" {
		t.Fatalf("a signed release not installed: %v\n%s", err, out)
	}
	// and over itself, as an update: the old one is replaced whole, nothing left beside it
	next := newFakeRelease(t, goos, "v1.2.4", []byte("build two"))
	next.sign(t, priv, "v1.2.4", "")
	if out, err := run(next, key); err != nil || installed() != "build two" {
		t.Fatalf("an update over an install: %v\n%s", err, out)
	}
	if left, _ := filepath.Glob(filepath.Join(appDir, ".Celestine.app.*")); len(left) != 0 {
		t.Fatalf("left beside the app: %v", left)
	}
	if goos == "linux" {
		b, err := os.ReadFile(filepath.Join(home, ".local", "share", "applications", "celestine.desktop"))
		if want := "Exec=\"" + filepath.Join(appDir, "celestine") + "\" %F\n"; err != nil || !strings.Contains(string(b), want) {
			t.Fatalf("the menu's entry %q, want %q", b, want)
		}
	}

	refused := func(why string, r *fakeRelease, key string) {
		t.Helper()
		out, err := run(r, key)
		if err == nil {
			t.Errorf("%s: installed\n%s", why, out)
		}
		t.Logf("%s: %s", why, strings.TrimSpace(out[strings.LastIndex(out, "\n  ")+1:]))
		if installed() != "build two" {
			t.Fatalf("%s: the installed app was changed: %q", why, installed())
		}
	}
	bad := newFakeRelease(t, goos, "v1.2.5", []byte("not ours"))
	bad.sign(t, otherPriv, "v1.2.5", "")
	refused("signed by another key", bad, key)
	bad.sign(t, priv, "v1.2.4", "")
	refused("signed for another version", bad, key)
	bad.sign(t, priv, "v1.2.5", "beta")
	refused("a trial's signature", bad, key)
	os.Remove(filepath.Join(bad.dir, bad.version, "SHA256SUMS.sig"))
	refused("no signature", bad, key)
	bad.sign(t, priv, "v1.2.5", "")
	refused("no key in the script", bad, "")
	os.WriteFile(filepath.Join(bad.dir, bad.version, "SHA256SUMS"), append([]byte(strings.Repeat("0", 64)+"  other\n"), bad.sums...), 0o644)
	refused("the sums changed after they were signed", bad, key)

	// a script of a release installs none older than it, whatever update.json says (#564)
	src, _ := os.ReadFile(script)
	floored := filepath.Join(t.TempDir(), "install.sh")
	os.WriteFile(floored, []byte(strings.Replace(string(src), "@RELEASE_VERSION@", "v1.2.4", 1)), 0o644)
	scriptAt = floored
	old := newFakeRelease(t, goos, "v1.2.3", []byte("an old one"))
	old.sign(t, priv, "v1.2.3", "")
	refused("a version older than the script's", old, key)
	if out, err := run(next, key); err != nil || installed() != "build two" {
		t.Fatalf("the script's own version: %v\n%s", err, out)
	}
	scriptAt = script

	// an ssh-keygen that cannot tell - before OpenSSH 8.1 (no -Y), or one of FIPS (no ed25519) - stops it, unless
	// CELESTINE_UNVERIFIED=1 says to install on the checksums alone; so does none at all
	cannot := map[string]string{
		"an ssh-keygen with no -Y": "#!/bin/sh\necho 'ssh-keygen: unknown option -- Y' >&2\nexit 1\n",
		"an ssh-keygen of FIPS":    "#!/bin/sh\necho 'ssh-keygen: ed25519 is not allowed' >&2\nexit 255\n",
	}
	none := t.TempDir() // every tool of the PATH, but no ssh-keygen
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		ents, _ := os.ReadDir(dir)
		for _, e := range ents {
			if e.Name() != "ssh-keygen" && e.Name() != "uname" {
				os.Symlink(filepath.Join(dir, e.Name()), filepath.Join(none, e.Name())) // the first of a name stays
			}
		}
	}
	os.Symlink(filepath.Join(shim, "uname"), filepath.Join(none, "uname"))
	paths := map[string]string{"no ssh-keygen": none}
	for why, body := range cannot {
		d := t.TempDir()
		os.WriteFile(filepath.Join(d, "ssh-keygen"), []byte(body), 0o755)
		paths[why] = d + string(os.PathListSeparator) + shim + string(os.PathListSeparator) + os.Getenv("PATH")
	}
	for why, path := range paths {
		third := newFakeRelease(t, goos, "v1.2.6", []byte("build three: "+why))
		third.sign(t, priv, "v1.2.6", "")
		extra = []string{"PATH=" + path}
		refused(why, third, key)
		extra = append(extra, "CELESTINE_UNVERIFIED=1")
		if out, err := run(third, key); err != nil || installed() != "build three: "+why || !strings.Contains(out, "not checked") {
			t.Fatalf("%s, CELESTINE_UNVERIFIED=1: %v\n%s", why, err, out)
		}
		next.sign(t, priv, "v1.2.4", "") // back to build two for the next
		extra = nil
		if out, err := run(next, key); err != nil {
			t.Fatalf("back to build two: %v\n%s", err, out)
		}
	}
}
