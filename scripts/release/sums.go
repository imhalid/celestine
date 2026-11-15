package main

import (
	"crypto/ed25519"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
)

// The install script (scripts/install.sh) checks a release's SHA256SUMS with the key the app checks its updates
// with, so that whoever can write to the server cannot install anything through `curl | sh` that the app's own
// updater would refuse (#540). It verifies with `ssh-keygen -Y verify`, which every Mac has and most Linux systems
// too, so the signature is an OpenSSH one (SSHSIG, PROTOCOL.sshsig of OpenSSH) made with the same ed25519 key.
// Its signed data opens with "SSHSIG", so it can never be read as one of the manifest's signatures.

// sumsNamespace is the SSHSIG namespace of the signature of SHA256SUMS: ssh-keygen checks it.
const sumsNamespace = "celestine-release"

// sumsText is what the signature of SHA256SUMS signs: a line that names the format, the version and the channel
// ("stable" for none), then the file as it is. The install script builds the same bytes from the version it was
// told and the file it downloaded, so a release's sums cannot be served as another version's, nor a trial's as a
// stable release's.
func sumsText(version, channel string, sums []byte) ([]byte, error) {
	if channel == "" {
		channel = "stable"
	}
	for _, p := range []string{version, channel} {
		if p == "" || strings.ContainsAny(p, "\r\n") {
			return nil, fmt.Errorf("cannot sign %q: empty, or a line break in it", p)
		}
	}
	return append([]byte("celestine-sums-1\n"+version+"\n"+channel+"\n"), sums...), nil
}

// sshString appends b as an SSH wire string: its length, then its bytes.
func sshString(out, b []byte) []byte {
	out = binary.BigEndian.AppendUint32(out, uint32(len(b)))
	return append(out, b...)
}

// sshPublicKeyBlob is an ed25519 public key in the SSH wire format.
func sshPublicKeyBlob(pub ed25519.PublicKey) []byte {
	return sshString(sshString(nil, []byte("ssh-ed25519")), pub)
}

// sshPublicKey is pub as a line of an authorized_keys or allowed_signers file: "ssh-ed25519 AAAA…".
func sshPublicKey(pub ed25519.PublicKey) string {
	return "ssh-ed25519 " + base64.StdEncoding.EncodeToString(sshPublicKeyBlob(pub))
}

// sshSign signs msg in the namespace as `ssh-keygen -Y sign` does, with SHA-512, and gives the armored signature
// `ssh-keygen -Y verify` reads.
func sshSign(priv ed25519.PrivateKey, namespace string, msg []byte) string {
	h := sha512.Sum512(msg)
	signed := []byte("SSHSIG")
	signed = sshString(signed, []byte(namespace))
	signed = sshString(signed, nil) // reserved
	signed = sshString(signed, []byte("sha512"))
	signed = sshString(signed, h[:])
	sig := sshString(sshString(nil, []byte("ssh-ed25519")), ed25519.Sign(priv, signed))

	blob := []byte("SSHSIG")
	blob = binary.BigEndian.AppendUint32(blob, 1)
	blob = sshString(blob, sshPublicKeyBlob(priv.Public().(ed25519.PublicKey)))
	blob = sshString(blob, []byte(namespace))
	blob = sshString(blob, nil)
	blob = sshString(blob, []byte("sha512"))
	blob = sshString(blob, sig)

	b64 := base64.StdEncoding.EncodeToString(blob)
	var out strings.Builder
	out.WriteString("-----BEGIN SSH SIGNATURE-----\n")
	for len(b64) > 70 {
		out.WriteString(b64[:70] + "\n")
		b64 = b64[70:]
	}
	out.WriteString(b64 + "\n-----END SSH SIGNATURE-----\n")
	return out.String()
}
