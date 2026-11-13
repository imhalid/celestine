package main

import "path/filepath"

// The secrets of the settings - the key of end-to-end encryption, the device's sync token, the MCP server's token -
// are kept in the system's keychain (macOS's Keychain, the Secret Service on Linux, Windows' Credential Manager)
// where there is one, and not in the settings, which lie in a file anyone with the disk can read. Where none answers,
// and in the browser, they stay in the settings as before. A secret the settings held before is moved over at the
// first start that finds a keychain.

// A secretKeeper keeps secrets by name: the system's keychain, or a test's.
type secretKeeper interface {
	Get(name string) (string, error)
	Set(name, secret string) error
	Delete(name string) error
}

// keychain is where the secrets go; nil, the settings keep them (a test, the browser). main sets it to the system's.
var keychain secretKeeper

// secretFields are the settings kept in the keychain, by the name they are kept under there.
func (s *Settings) secretFields() []struct {
	name string
	v    *string
} {
	return []struct {
		name string
		v    *string
	}{{"syncKey", &s.SyncKey}, {"syncToken", &s.SyncToken}, {"mcpToken", &s.Token}}
}

// secretScope is what tells this library's secrets from another's in the keychain: the file its settings are kept in.
func (a *app) secretScope() string {
	f := a.settingsFile
	if a.settingsStore != nil {
		f = a.settingsStore.File
	}
	if abs, err := filepath.Abs(f); err == nil {
		f = abs
	}
	return f
}

// loadSecrets fills the settings' secrets from the keychain, after the settings were read. One the settings still
// hold is put in the keychain instead, and the answer says so: the settings are then written again without it.
func (a *app) loadSecrets() (moved bool) { return a.loadSecretsWith(false) }

// loadSecretsWith fills settings' secrets from the keychain and migrates plaintext secrets unless readOnly is true.
func (a *app) loadSecretsWith(readOnly bool) (moved bool) {
	if keychain == nil {
		return false
	}
	a.inKeychain = map[string]string{}
	scope := a.secretScope()
	for _, f := range a.settings.secretFields() {
		if *f.v != "" { // kept in the settings: moved over (unless reading read-only, #165)
			if !readOnly && keychain.Set(scope+"#"+f.name, *f.v) == nil {
				a.inKeychain[f.name] = *f.v
				moved = true
			}
			continue
		}
		if v, err := keychain.Get(scope + "#" + f.name); err == nil && v != "" {
			*f.v = v
			a.inKeychain[f.name] = v
		}
	}
	return moved
}

// settingsToSave is the settings as they are written: without the secrets the keychain holds. A secret is written to
// the keychain only when it changed, the settings being saved often; one the keychain does not take stays in them.
func (a *app) settingsToSave() Settings {
	out := a.settings
	if keychain == nil {
		return out
	}
	if a.inKeychain == nil {
		a.inKeychain = map[string]string{}
	}
	scope := a.secretScope()
	for _, f := range out.secretFields() {
		old, had := a.inKeychain[f.name]
		switch {
		case *f.v == "":
			if had { // signed out: it goes from the keychain too
				keychain.Delete(scope + "#" + f.name)
				delete(a.inKeychain, f.name)
			}
		case had && old == *f.v:
			*f.v = ""
		case keychain.Set(scope+"#"+f.name, *f.v) == nil:
			a.inKeychain[f.name] = *f.v
			*f.v = ""
		default:
			delete(a.inKeychain, f.name)
		}
	}
	return out
}
