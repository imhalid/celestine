package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// What a release signed is checked with the public key the programs are built with, before anything goes up (#174):
// a key in the CI that is not that key's half, or a signing gone wrong, stops the release instead of sending files
// no app can check.
//
//	go run ./scripts/release verify -pub <public key> -version v1.3.0 -channel "" -notes notes.md -dir dist update.json
//	go run ./scripts/release verify-sums -pub <public key> -version v1.3.0 -channel "" dist/SHA256SUMS dist/SHA256SUMS.sig

// verifyManifest checks the manifest m: each asset's file in dir has its size and hash, and its three signatures are
// pub's, of the texts the app checks.
func verifyManifest(pub ed25519.PublicKey, rel signed, dir string, m []byte) error {
	var man struct {
		Version string           `json:"version"`
		Channel string           `json:"channel"`
		Notes   string           `json:"notes"`
		Assets  map[string]asset `json:"assets"`
	}
	if err := json.Unmarshal(m, &man); err != nil {
		return err
	}
	if man.Version != rel.version || man.Channel != rel.channel || man.Notes != rel.notes {
		return fmt.Errorf("the manifest is of %s %q, not %s %q, or its notes are others", man.Version, man.Channel, rel.version, rel.channel)
	}
	if len(man.Assets) == 0 {
		return errors.New("the manifest has no files")
	}
	ok := func(sig string, text []byte) bool {
		b, err := base64.StdEncoding.DecodeString(sig)
		return err == nil && ed25519.Verify(pub, text, b)
	}
	for key, a := range man.Assets {
		name := path.Base(a.URL)
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		h := sha256.New()
		n, err := io.Copy(h, f)
		f.Close()
		if err != nil {
			return err
		}
		if sum := hex.EncodeToString(h.Sum(nil)); sum != a.SHA256 || n != a.Size {
			return fmt.Errorf("%s: the manifest's hash or size is not the file's", name)
		}
		text, err := sigText(rel.version, key, name, a.Kind, a.SHA256)
		if err != nil {
			return err
		}
		text3, err := sig3Text(rel.version, rel.channel, key, name, a.Kind, a.SHA256, rel.notes)
		if err != nil {
			return err
		}
		if !ok(a.Sig, legacySigText(rel.version, a.SHA256)) || !ok(a.Sig2, text) || !ok(a.Sig3, text3) {
			return fmt.Errorf("%s: a signature is not the public key's", name)
		}
	}
	return nil
}

// verifySums checks sig, an armored signature of sshSign, as `ssh-keygen -Y verify` would: made by pub, in the
// namespace of the sums, of the sums text of version and channel.
func verifySums(pub ed25519.PublicKey, version, channel string, sums []byte, sig string) error {
	text, err := sumsText(version, channel, sums)
	if err != nil {
		return err
	}
	armored := strings.TrimSpace(sig)
	armored = strings.TrimPrefix(armored, "-----BEGIN SSH SIGNATURE-----")
	armored = strings.TrimSuffix(armored, "-----END SSH SIGNATURE-----")
	blob, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(armored), ""))
	if err != nil {
		return err
	}
	rest, found := bytes.CutPrefix(blob, []byte("SSHSIG"))
	if !found || len(rest) < 4 || binary.BigEndian.Uint32(rest) != 1 {
		return errors.New("not an SSH signature of version 1")
	}
	rest = rest[4:]
	var fields [5][]byte // the public key, the namespace, reserved, the hash, the signature
	for i := range fields {
		if fields[i], rest, err = sshField(rest); err != nil {
			return err
		}
	}
	if !bytes.Equal(fields[0], sshPublicKeyBlob(pub)) || string(fields[1]) != sumsNamespace || string(fields[3]) != "sha512" {
		return errors.New("the signature is of another key, namespace or hash")
	}
	kind, rest, err := sshField(fields[4])
	if err != nil {
		return err
	}
	raw, _, err := sshField(rest)
	if err != nil || string(kind) != "ssh-ed25519" {
		return errors.New("not an ed25519 signature")
	}
	h := sha512.Sum512(text)
	signedData := []byte("SSHSIG")
	signedData = sshString(signedData, []byte(sumsNamespace))
	signedData = sshString(signedData, nil)
	signedData = sshString(signedData, []byte("sha512"))
	signedData = sshString(signedData, h[:])
	if !ed25519.Verify(pub, signedData, raw) {
		return errors.New("the signature of the sums is not the public key's")
	}
	return nil
}

// sshField reads an SSH wire string from b: its bytes, and what follows.
func sshField(b []byte) (field, rest []byte, err error) {
	if len(b) < 4 {
		return nil, nil, errors.New("a signature cut short")
	}
	n := binary.BigEndian.Uint32(b)
	if uint64(len(b)-4) < uint64(n) {
		return nil, nil, errors.New("a signature cut short")
	}
	return b[4 : 4+n], b[4+n:], nil
}

// readPub reads a public key as `keygen` prints it.
func readPub(s string) ed25519.PublicKey {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	check(err)
	if len(raw) != ed25519.PublicKeySize {
		check(fmt.Errorf("invalid public key length: got %d, want %d", len(raw), ed25519.PublicKeySize))
	}
	return ed25519.PublicKey(raw)
}
