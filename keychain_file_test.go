package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// memKeeper is a keychain of a map, which counts what it is asked.
type memKeeper struct {
	m    map[string]string
	gets int
	err  error
}

func (k *memKeeper) Get(name string) (string, error) {
	k.gets++
	if k.err != nil {
		return "", k.err
	}
	if v, ok := k.m[name]; ok {
		return v, nil
	}
	return "", errNoSecret
}
func (k *memKeeper) Set(name, v string) error { k.m[name] = v; return nil }
func (k *memKeeper) Delete(name string) error { delete(k.m, name); return nil }

// The file keychain of macOS keeps the secrets in a file of the user's alone, moves one the Keychain held over once,
// and does not bring back from there one deleted here (#626).
func TestFileKeychain(t *testing.T) {
	dir := t.TempDir()
	old := &memKeeper{m: map[string]string{"a#syncToken": "tok", "a#syncKey": "key"}}
	k := &fileKeychain{path: filepath.Join(dir, "celestine", "secrets.json"), old: old}
	if v, err := k.Get("a#syncToken"); err != nil || v != "tok" {
		t.Fatalf("moved over: %q %v", v, err)
	}
	if v, err := k.Get("a#syncToken"); err != nil || v != "tok" || old.gets != 1 {
		t.Fatalf("again: %q %v, the old keychain asked %d times", v, err, old.gets)
	}
	if _, err := k.Get("a#mcpToken"); err != errNoSecret {
		t.Fatalf("none: %v", err)
	}
	k.Get("a#mcpToken")
	if old.gets != 2 {
		t.Fatalf("a name looked for twice in the old keychain: %d", old.gets)
	}
	if err := k.Delete("a#syncKey"); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Get("a#syncKey"); err != errNoSecret || old.gets != 2 {
		t.Fatalf("deleted came back: %v, %d", err, old.gets)
	}
	if err := k.Set("b#syncKey", "new"); err != nil {
		t.Fatal(err)
	}
	k2 := &fileKeychain{path: k.path, old: old}
	if v, _ := k2.Get("b#syncKey"); v != "new" {
		t.Fatalf("read again: %q", v)
	}
	st, err := os.Stat(k.path)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("file %v %v", st, err)
	}
	if st, _ := os.Stat(filepath.Dir(k.path)); st.Mode().Perm() != 0o700 {
		t.Fatalf("folder %v", st.Mode())
	}
}

func TestFileKeychainCorruptRecoveryAndOldError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "celestine", "secrets.json")
	old := &memKeeper{m: map[string]string{"a#syncToken": "tok"}, err: errors.New("keychain locked")}
	k := &fileKeychain{path: path, old: old}

	// A real Keychain error must be returned and must not permanently mark the key Tried.
	if _, err := k.Get("a#syncToken"); err == nil || errors.Is(err, errNoSecret) {
		t.Fatalf("expected keychain error, got %v", err)
	}
	old.err = nil
	if v, err := k.Get("a#syncToken"); err != nil || v != "tok" {
		t.Fatalf("retry after unlock failed: %q %v", v, err)
	}

	// Corrupt JSON must not permanently wedge Set.
	if err := os.WriteFile(path, []byte("{broken json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := k.Set("a#syncToken", "recovered"); err != nil {
		t.Fatalf("Set on corrupt JSON failed: %v", err)
	}
	if v, err := k.Get("a#syncToken"); err != nil || v != "recovered" {
		t.Fatalf("after recovery: %q %v", v, err)
	}
}
