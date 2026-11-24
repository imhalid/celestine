package library

import (
	"bytes"
	"crypto/pbkdf2"
	"crypto/sha256"
	"testing"
)

// The data key is not used as it is (#162): the key that seals, the key that names files and what the recovery key
// proves are each made from it with a label of their own, so that no file's name is what the server keeps the hash
// of, and nothing sealed opens with the data key; a remote an older build made goes on with the data key.
func TestSyncKeysApart(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	e, err := NewEncRemote(inner, sealKey)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(e.names, sealKey) || bytes.Equal(e.names, subKey(sealKey, "celestine/seal/v2")) {
		t.Fatal("the names' key is the data key, or the sealing one")
	}
	proof := RecoveryProof(sealKey)
	for _, p := range []string{"celestine/recovery-auth", "celestine/recovery-proof/v2", proof} {
		if e.opaque(p) == proof {
			t.Fatalf("a file at %q is named what the recovery key proves", p)
		}
	}
	d := newDevice(t)
	d.write("Not/Bir/page.json", "bir")
	d.sync(e)
	raw := mustGet(t, inner, IndexFile)
	old, _ := newGCM(sealKey)
	if _, _, err := openWith(old, raw, indexAAD); err == nil {
		t.Fatal("the index opens with the data key itself")
	}
	if got := d.read("Not/Bir/page.json"); got != "bir" {
		t.Fatalf("read back: %q", got)
	}

	// a remote of before: sealed and named with the data key, which this version goes on with
	before := DirRemote{Root: t.TempDir()}
	madeBefore(t, before)
	b := newDevice(t)
	b.write("Not/İki/page.json", "iki")
	eb, _ := NewEncRemote(before, sealKey)
	res := b.sync(eb)
	if res.Pushed != 1 || !bytes.Equal(eb.names, sealKey) {
		t.Fatalf("a remote of before: %+v, its names' key the data key %v", res, bytes.Equal(eb.names, sealKey))
	}
	if _, _, err := openWith(old, mustGet(t, before, IndexFile), indexAAD); err != nil {
		t.Fatalf("the index of a remote of before is not sealed as it was: %v", err)
	}
}

// A passphrase is stretched with Argon2id on a remote made now, its salt saying so, and with PBKDF2 on one whose
// salt is of before (#154).
func TestSyncPassphraseStretch(t *testing.T) {
	inner := DirRemote{Root: t.TempDir()}
	e, err := OpenEnc(inner, "uzun bir parola")
	if err != nil {
		t.Fatal(err)
	}
	salt := mustGet(t, inner, SaltFile)
	s, ok := bytes.CutPrefix(salt, argonSalt)
	if !ok || len(s) != 16 {
		t.Fatalf("the salt of a new remote: %q", salt)
	}
	if want := StretchPassword("uzun bir parola", s, ArgonTime, ArgonMemory, ArgonThreads); !bytes.Equal(e.Key, want) {
		t.Fatal("the key is not Argon2id's")
	}
	if _, err := OpenEnc(inner, "başka bir parola"); err != ErrWrongPassphrase {
		t.Fatalf("another passphrase: %v", err)
	}
	if again, err := OpenEnc(inner, "uzun bir parola"); err != nil || !bytes.Equal(again.Key, e.Key) {
		t.Fatalf("the same passphrase again: %v", err)
	}

	old := DirRemote{Root: t.TempDir()}
	oldSalt := bytes.Repeat([]byte{7}, 16)
	old.Put(SaltFile, oldSalt, "")
	k, err := DeriveKey("eski parola", oldSalt)
	want, _ := pbkdf2.Key(sha256.New, "eski parola", oldSalt, 200_000, 32)
	if err != nil || !bytes.Equal(k, want) {
		t.Fatal("a salt of before is not PBKDF2's")
	}
}
