package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// What manifest and sums sign checks with their public key, and not with another; a file changed after its
// signing, the notes or the channel taken for others, and sums of another version are refused (#174).
func TestVerifyRelease(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	dir := t.TempDir()
	bin := filepath.Join(dir, "celestine-linux-amd64")
	os.WriteFile(bin, []byte("the program"), 0o755)
	rel := signed{version: "v1.2.3", notes: "- a fix\n"}
	key, a, ok, err := signFile(priv, rel, "https://dl.example.com/v1.2.3", bin)
	if err != nil || !ok {
		t.Fatal(err)
	}
	m, _ := json.Marshal(map[string]any{"version": rel.version, "notes": rel.notes, "assets": map[string]asset{key: a}})
	if err := verifyManifest(pub, rel, dir, m); err != nil {
		t.Fatalf("its own key: %v", err)
	}
	if err := verifyManifest(other, rel, dir, m); err == nil {
		t.Fatal("another key took the manifest")
	}
	if err := verifyManifest(pub, signed{version: rel.version, channel: "beta", notes: rel.notes}, dir, m); err == nil {
		t.Fatal("a stable manifest was taken for a trial")
	}
	os.WriteFile(bin, []byte("another program"), 0o755)
	if err := verifyManifest(pub, rel, dir, m); err == nil {
		t.Fatal("a file changed after its signing was taken")
	}

	sums := []byte("abc  celestine-linux-amd64\n")
	text, _ := sumsText("v1.2.3", "", sums)
	sig := sshSign(priv, sumsNamespace, text)
	if err := verifySums(pub, "v1.2.3", "", sums, sig); err != nil {
		t.Fatalf("the sums with their key: %v", err)
	}
	if err := verifySums(other, "v1.2.3", "", sums, sig); err == nil {
		t.Fatal("another key took the sums")
	}
	if err := verifySums(pub, "v1.2.4", "", sums, sig); err == nil {
		t.Fatal("the sums of another version were taken")
	}
	if err := verifySums(pub, "v1.2.3", "", append(sums, 'x'), sig); err == nil {
		t.Fatal("changed sums were taken")
	}

	// the key from the environment, not a file
	t.Setenv("RELEASE_KEY_TEST", base64.StdEncoding.EncodeToString(priv))
	if got := readKey("env:RELEASE_KEY_TEST"); !got.Equal(priv) {
		t.Fatal("the key from the environment is another")
	}
}
