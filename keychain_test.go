//go:build !js

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	keyring "github.com/zalando/go-keyring"
)

// withKeychain puts k in the place of the system's keychain for the test.
func withKeychain(t *testing.T, k secretKeeper) {
	old := keychain
	keychain = k
	t.Cleanup(func() { keychain = old })
}

// settingsApp is an app whose settings are kept in a file of the test's.
func settingsApp(t *testing.T, file string) *app {
	a := newApp(testLibrary(t, "# Sayfa\n"))
	a.settingsFile, a.settings = file, Settings{}
	a.loadSettings()
	return a
}

// The secrets of the settings are kept in the keychain, not in the file; the next start reads them back from there.
func TestKeychainKeepsSecrets(t *testing.T) {
	keyring.MockInit()
	withKeychain(t, osKeychain{})
	file := filepath.Join(t.TempDir(), "settings.json")
	a := settingsApp(t, file)
	a.settings.SyncKey, a.settings.SyncToken, a.settings.SyncMode = "S3CRETKEY", "TOKEN123", "server"
	a.saveSettings()
	data, _ := os.ReadFile(file)
	for _, s := range []string{"S3CRETKEY", "TOKEN123", a.settings.Token} {
		if strings.Contains(string(data), s) {
			t.Fatalf("a secret is in the settings file: %s", data)
		}
	}
	if !strings.Contains(string(data), `"server"`) {
		t.Fatalf("the rest of the settings is not in the file: %s", data)
	}
	b := settingsApp(t, file)
	if b.settings.SyncKey != "S3CRETKEY" || b.settings.SyncToken != "TOKEN123" || b.settings.Token != a.settings.Token {
		t.Fatalf("the secrets did not come back from the keychain: %+v", b.settings)
	}
	// signed out: they go from the keychain too
	b.settings.SyncKey, b.settings.SyncToken = "", ""
	b.saveSettings()
	if _, err := keyring.Get(keychainService, b.secretScope()+"#syncKey"); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("the key is still in the keychain: %v", err)
	}
}

// Secrets a settings file of before holds are moved to the keychain at the first start, and taken out of the file.
func TestKeychainMovesOldSecrets(t *testing.T) {
	keyring.MockInit()
	withKeychain(t, osKeychain{})
	file := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(file, []byte(`{"token":"MCPOLD","syncKey":"KEYOLD","syncToken":"TOKOLD","syncMode":"server"}`), 0o600)
	a := settingsApp(t, file)
	if a.settings.SyncKey != "KEYOLD" || a.settings.Token != "MCPOLD" {
		t.Fatalf("the secrets were lost: %+v", a.settings)
	}
	data, _ := os.ReadFile(file)
	if strings.Contains(string(data), "OLD") {
		t.Fatalf("the secrets were left in the file: %s", data)
	}
	if v, _ := keyring.Get(keychainService, a.secretScope()+"#syncToken"); v != "TOKOLD" {
		t.Fatalf("the keychain has %q", v)
	}
}

// With no keychain that answers (none on the system, or none in the browser), the settings file keeps the secrets as before.
func TestKeychainFallback(t *testing.T) {
	for name, k := range map[string]secretKeeper{"none": nil, "unreachable": osKeychain{}} {
		keyring.MockInitWithError(errors.New("no secret service"))
		withKeychain(t, k)
		file := filepath.Join(t.TempDir(), "settings.json")
		a := settingsApp(t, file)
		a.settings.SyncKey = "S3CRETKEY"
		a.saveSettings()
		data, _ := os.ReadFile(file)
		if !strings.Contains(string(data), "S3CRETKEY") || !strings.Contains(string(data), a.settings.Token) {
			t.Fatalf("%s: the file lost a secret: %s", name, data)
		}
		if b := settingsApp(t, file); b.settings.SyncKey != "S3CRETKEY" {
			t.Fatalf("%s: the key did not come back", name)
		}
	}
}
