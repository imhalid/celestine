package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSigTextIsTheApps: the text signed is the one the app checks (update.go's test holds the same literal).
func TestSigTextIsTheApps(t *testing.T) {
	got, err := sigText("v1.3.0", "darwin-arm64", "celestine-darwin-arm64.zip", "app-zip", strings.Repeat("AB", 32))
	if want := "celestine-update-2\nv1.3.0\ndarwin-arm64\ncelestine-darwin-arm64.zip\napp-zip\n" + strings.Repeat("ab", 32); err != nil || string(got) != want {
		t.Fatalf("%q %v, want %q", got, err, want)
	}
	if got, _ := sigText("v1.3.0", "linux-amd64", "celestine-linux-amd64", "", "ab"); string(got) != "celestine-update-2\nv1.3.0\nlinux-amd64\ncelestine-linux-amd64\nbinary\nab" {
		t.Fatalf("no kind is not binary: %q", got)
	}
	if _, err := sigText("v1.3.0\nx", "linux-amd64", "n", "", "ab"); err == nil {
		t.Fatal("a line break was signed")
	}
	if got := legacySigText("v1.3.0", "AB"); string(got) != "v1.3.0\nab" {
		t.Fatalf("old text %q", got)
	}
	// the third text, the one the app requires since #168: the sha256 of "- faster" closes it
	got, err = sig3Text("v1.3.0-beta.1", "beta", "darwin-arm64", "celestine-darwin-arm64.zip", "app-zip", strings.Repeat("AB", 32), "- faster")
	if want := "celestine-update-3\nv1.3.0-beta.1\nbeta\ndarwin-arm64\ncelestine-darwin-arm64.zip\napp-zip\n" + strings.Repeat("ab", 32) +
		"\nc9d3478adaa449914687b73062b06018452b2f73e56431869abaacb933675946"; err != nil || string(got) != want {
		t.Fatalf("%q %v, want %q", got, err, want)
	}
	if got, _ := sig3Text("v1.3.0", "", "linux-amd64", "celestine-linux-amd64", "", "ab", ""); string(got) != "celestine-update-3\nv1.3.0\nstable\nlinux-amd64\ncelestine-linux-amd64\nbinary\nab\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("no channel is not stable, or no kind not binary: %q", got)
	}
}

// TestSignFile: each file gets both signatures - the old one released clients check, and the second one bound to
// its system, name and kind (#455) - and the second one does not hold for another system or name.
func TestSignFile(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	write := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("build of "+name), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	verify := func(text []byte, sig string) bool {
		b, err := base64.StdEncoding.DecodeString(sig)
		return err == nil && ed25519.Verify(pub, text, b)
	}

	rel := signed{version: "v1.3.0", notes: "- faster"}
	key, a, ok, err := signFile(priv, rel, "https://dl.example.com/v1.3.0/", write("celestine-darwin-arm64.zip"))
	if err != nil || !ok || key != "darwin-arm64" || a.Kind != "app-zip" || a.URL != "https://dl.example.com/v1.3.0/celestine-darwin-arm64.zip" {
		t.Fatalf("%s %+v %v %v", key, a, ok, err)
	}
	if !verify(legacySigText("v1.3.0", a.SHA256), a.Sig) {
		t.Fatal("the old signature, for released clients, does not verify")
	}
	text, _ := sigText("v1.3.0", "darwin-arm64", "celestine-darwin-arm64.zip", "app-zip", a.SHA256)
	if !verify(text, a.Sig2) {
		t.Fatal("the second signature does not verify")
	}
	if !verify(must(sig3Text("v1.3.0", "", "darwin-arm64", "celestine-darwin-arm64.zip", "app-zip", a.SHA256, "- faster")), a.Sig3) {
		t.Fatal("the third signature does not verify")
	}
	// #168: the third signature holds the channel and the notes too
	for why, text := range map[string][]byte{
		"the beta channel": must(sig3Text("v1.3.0", "beta", "darwin-arm64", "celestine-darwin-arm64.zip", "app-zip", a.SHA256, "- faster")),
		"other notes":      must(sig3Text("v1.3.0", "", "darwin-arm64", "celestine-darwin-arm64.zip", "app-zip", a.SHA256, "- slower")),
		"another system":   must(sig3Text("v1.3.0", "", "linux-amd64", "celestine-darwin-arm64.zip", "app-zip", a.SHA256, "- faster")),
	} {
		if verify(text, a.Sig3) {
			t.Errorf("%s: the third signature holds", why)
		}
	}
	for why, text := range map[string][]byte{
		"another system": must(sigText("v1.3.0", "linux-amd64", "celestine-darwin-arm64.zip", "app-zip", a.SHA256)),
		"another name":   must(sigText("v1.3.0", "darwin-arm64", "celestine-linux-amd64", "app-zip", a.SHA256)),
		"another kind":   must(sigText("v1.3.0", "darwin-arm64", "celestine-darwin-arm64.zip", "binary", a.SHA256)),
	} {
		if verify(text, a.Sig2) {
			t.Errorf("%s: the second signature holds", why)
		}
	}

	key, a, ok, err = signFile(priv, rel, "https://dl.example.com/v1.3.0", write("celestine-windows-amd64.exe"))
	text, _ = sigText("v1.3.0", "windows-amd64", "celestine-windows-amd64.exe", "", a.SHA256)
	if err != nil || !ok || key != "windows-amd64" || a.Kind != "" || !verify(text, a.Sig2) {
		t.Fatalf("windows: %s %+v %v %v", key, a, ok, err)
	}
	// an AppImage is listed apart from the bare binary, as the update of a program run from one (#513)
	key, a, ok, err = signFile(priv, rel, "https://dl.example.com/v1.3.0", write("celestine-linux-arm64.AppImage"))
	text, _ = sigText("v1.3.0", "linux-arm64-appimage", "celestine-linux-arm64.AppImage", "appimage", a.SHA256)
	if err != nil || !ok || key != "linux-arm64-appimage" || a.Kind != "appimage" || !verify(text, a.Sig2) {
		t.Fatalf("appimage: %s %+v %v %v", key, a, ok, err)
	}
	if verify(must(sigText("v1.3.0", "linux-arm64", "celestine-linux-arm64.AppImage", "binary", a.SHA256)), a.Sig2) {
		t.Error("the AppImage's signature holds for the bare binary's place")
	}
	if _, _, ok, err := signFile(priv, rel, "https://x", write("celestine-linux-amd64.tar.gz")); ok || err != nil {
		t.Fatalf("the .tar.gz for people was taken: %v %v", ok, err)
	}
}

func must(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return b
}
