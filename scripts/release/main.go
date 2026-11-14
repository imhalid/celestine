// Command release makes what a release needs: a key pair, the manifest the app reads, signed, and the changelog the
// site shows (changelog.go).
//
//	go run ./scripts/release keygen                       writes release.key (secret) and prints the public key
//	go run ./scripts/release manifest -version v1.3.0 -base https://dl.example.com/v1.3.0 \
//	    -notes notes.md -key release.key dist/celestine-*    prints update.json
//	go run ./scripts/release changelog                    prints changelog.json, every release from the git tags
//
// The secret key lives only where the release is made (a secret of the CI); the public key goes into the
// program at the build: -X main.updateKey=<public key>.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// An asset is one file of the manifest, as the app's Asset (update.go) reads it.
type asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Sig    string `json:"sig"`  // of legacySigText: all the clients before #455 check
	Sig2   string `json:"sig2"` // of sigText: what the app requires since #455
	Size   int64  `json:"size"`
	Kind   string `json:"kind,omitempty"`
}

// sigText is the text of the second signature, the app's sigText (update.go) word for word: the version, the
// system the file is for, its name, its kind and its hash. Both tests hold it to the same literal.
func sigText(version, platform, name, kind, sha string) ([]byte, error) {
	if kind == "" {
		kind = "binary"
	}
	parts := []string{"celestine-update-2", version, platform, name, kind, strings.ToLower(sha)}
	for _, p := range parts[1:] {
		if p == "" || strings.ContainsAny(p, "\r\n") {
			return nil, fmt.Errorf("cannot sign %q: empty, or a line break in it", p)
		}
	}
	return []byte(strings.Join(parts, "\n")), nil
}

// legacySigText is the text of the first signature, the version and the hash: the clients released before #455
// check only this, so a release keeps signing it for them.
func legacySigText(version, sha string) []byte { return []byte(version + "\n" + strings.ToLower(sha)) }

// signFile hashes and signs the file at f for the manifest, served at base: the "os-arch" it is listed under,
// from its name (celestine-linux-amd64, celestine-windows-amd64.exe, celestine-darwin-arm64.zip), and its asset
// with both signatures. An AppImage (celestine-linux-amd64.AppImage, scripts/appimage.sh) is listed apart, under
// linux-amd64-appimage with the kind appimage: the update of a program run from one (update.go: updatePlatform).
// ok is false for a file that is not an update (the .tar.gz of Linux is for people, scripts/package.sh).
func signFile(priv ed25519.PrivateKey, version, base, f string) (key string, a asset, ok bool, err error) {
	name := filepath.Base(f)
	if strings.HasSuffix(name, ".tar.gz") {
		return "", a, false, nil
	}
	appImage := strings.HasSuffix(name, ".AppImage")
	parts := strings.Split(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(name, ".exe"), ".zip"), ".AppImage"), "-")
	if len(parts) < 3 {
		return "", a, false, nil
	}
	key = parts[1] + "-" + parts[2]
	if appImage {
		key += "-appimage"
	}
	in, err := os.Open(f)
	if err != nil {
		return "", a, false, err
	}
	defer in.Close()
	h := sha256.New()
	n, err := io.Copy(h, in)
	if err != nil {
		return "", a, false, err
	}
	a = asset{URL: strings.TrimRight(base, "/") + "/" + name, SHA256: hex.EncodeToString(h.Sum(nil)), Size: n}
	if strings.HasSuffix(name, ".zip") {
		a.Kind = "app-zip"
	} else if appImage {
		a.Kind = "appimage"
	}
	text, err := sigText(version, key, name, a.Kind, a.SHA256)
	if err != nil {
		return "", a, false, err
	}
	a.Sig = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, legacySigText(version, a.SHA256)))
	a.Sig2 = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, text))
	return key, a, true, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "keygen | manifest | changelog")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "keygen":
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		check(err)
		check(os.WriteFile("release.key", []byte(base64.StdEncoding.EncodeToString(priv)), 0o600))
		fmt.Println(base64.StdEncoding.EncodeToString(pub))
	case "manifest":
		fs := flag.NewFlagSet("manifest", flag.ExitOnError)
		version := fs.String("version", "", "the version, v1.3.0")
		base := fs.String("base", "", "where the files are served from")
		notes := fs.String("notes", "", "a Markdown file of the notes")
		keyFile := fs.String("key", "release.key", "the secret key")
		channel := fs.String("channel", "", "beta for a trial")
		fs.Parse(os.Args[2:])
		kb, err := os.ReadFile(*keyFile)
		check(err)
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(kb)))
		check(err)
		if len(raw) != ed25519.PrivateKeySize {
			check(fmt.Errorf("invalid private key length: got %d, want %d", len(raw), ed25519.PrivateKeySize))
		}
		priv := ed25519.PrivateKey(raw)
		out := map[string]any{"version": *version, "date": time.Now().Format("2006-01-02")}
		if *channel != "" {
			out["channel"] = *channel
		}
		if *notes != "" {
			b, err := os.ReadFile(*notes)
			check(err)
			out["notes"] = string(b)
		}
		assets := map[string]asset{}
		for _, f := range fs.Args() {
			key, a, ok, err := signFile(priv, *version, *base, f)
			check(err)
			if !ok {
				continue
			}
			if _, ok := assets[key]; ok {
				check(fmt.Errorf("two files for %s: %s and another (a bare darwin binary beside its zip?)", key, f))
			}
			assets[key] = a
		}
		out["assets"] = assets
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		check(enc.Encode(out))
	case "changelog":
		tags, err := gitTags()
		check(err)
		c, err := buildChangelog(tags, gitTitles, time.Now())
		check(err)
		enc := json.NewEncoder(os.Stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		check(enc.Encode(c))
	default:
		fmt.Fprintln(os.Stderr, "keygen | manifest | changelog")
		os.Exit(2)
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
