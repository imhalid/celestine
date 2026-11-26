package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

// fileKeychain keeps the settings' secrets in a file of the app's own, readable by the user alone (0600, in a 0700
// folder), outside the library so that they never travel with it: the keychain of macOS (#626). Its Keychain trusts
// a program by its signature, and the releases are signed ad hoc, so each update was another program to it, which
// asked for the login password before it gave each secret. A secret the Keychain held before (old) is read from it
// once, at the first Get of its name, and moved over; a name tried is not tried again, so that one deleted here (a
// sign-out) does not come back from there, and the Keychain is not asked again after it was refused.
type fileKeychain struct {
	path string
	old  secretKeeper // the keychain the secrets were kept in before; nil for none
	mu   sync.Mutex
}

// fileSecrets is the file of a fileKeychain: the secrets by name, and the names already looked for in the old keychain.
type fileSecrets struct {
	Secrets map[string]string `json:"secrets"`
	Tried   []string          `json:"tried,omitempty"`
}

// secretsFile is where the secrets are kept on macOS: secrets.json in the app's folder of the user's configuration.
func secretsFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "celestine", "secrets.json")
}

// read is the file's secrets; a missing file is none.
func (k *fileKeychain) read() (fileSecrets, error) {
	s := fileSecrets{Secrets: map[string]string{}}
	b, err := os.ReadFile(k.path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return fileSecrets{Secrets: map[string]string{}}, err
	}
	if s.Secrets == nil {
		s.Secrets = map[string]string{}
	}
	return s, nil
}

// write puts s in the file, whole: written beside it and renamed over it, so that a crash leaves the old file or the new.
func (k *fileKeychain) write(s fileSecrets) error {
	dir := filepath.Dir(k.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	hardenPrivateDir(dir)
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := k.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	_ = os.Chmod(tmp, 0o600)
	if err := os.Rename(tmp, k.path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func (k *fileKeychain) Get(name string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	s, err := k.read()
	if err != nil {
		return "", err
	}
	if v, ok := s.Secrets[name]; ok {
		return v, nil
	}
	if k.old == nil || slices.Contains(s.Tried, name) {
		return "", errNoSecret
	}
	s.Tried = append(s.Tried, name)
	if v, err := k.old.Get(name); err == nil && v != "" {
		s.Secrets[name] = v
		_ = k.write(s) // read all the same: one not written is moved at another start
		return v, nil
	}
	_ = k.write(s)
	return "", errNoSecret
}

func (k *fileKeychain) Set(name, secret string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	s, err := k.read()
	if err != nil {
		return err
	}
	s.Secrets[name] = secret
	if !slices.Contains(s.Tried, name) { // set here: what the old keychain has under it is older
		s.Tried = append(s.Tried, name)
	}
	return k.write(s)
}

func (k *fileKeychain) Delete(name string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	s, err := k.read()
	if err != nil {
		return err
	}
	_, had := s.Secrets[name]
	delete(s.Secrets, name)
	if !slices.Contains(s.Tried, name) {
		s.Tried = append(s.Tried, name)
	} else if !had {
		return nil
	}
	return k.write(s)
}
