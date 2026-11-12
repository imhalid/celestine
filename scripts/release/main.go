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

type asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Sig    string `json:"sig"`
	Size   int64  `json:"size"`
	Kind   string `json:"kind,omitempty"`
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
			name := filepath.Base(f)
			// the .tar.gz of Linux is for people (scripts/package.sh); the update takes the bare binary
			if strings.HasSuffix(name, ".tar.gz") {
				continue
			}
			// celestine-linux-amd64, celestine-windows-amd64.exe, celestine-darwin-arm64.zip
			parts := strings.Split(strings.TrimSuffix(strings.TrimSuffix(name, ".exe"), ".zip"), "-")
			if len(parts) < 3 {
				continue
			}
			key := parts[1] + "-" + parts[2]
			if _, ok := assets[key]; ok {
				check(fmt.Errorf("two files for %s: %s and another (a bare darwin binary beside its zip?)", key, name))
			}
			h := sha256.New()
			in, err := os.Open(f)
			check(err)
			n, err := io.Copy(h, in)
			in.Close()
			check(err)
			sha := hex.EncodeToString(h.Sum(nil))
			sig := ed25519.Sign(priv, []byte(*version+"\n"+sha))
			a := asset{URL: strings.TrimRight(*base, "/") + "/" + name, SHA256: sha, Sig: base64.StdEncoding.EncodeToString(sig), Size: n}
			if strings.HasSuffix(name, ".zip") {
				a.Kind = "app-zip"
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
