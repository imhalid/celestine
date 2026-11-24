//go:build !js

package main

import (
	"errors"

	"github.com/egoist/mygo"
)

// osKeychain is the system's keychain, through MyGo's Secrets (#336): macOS's Keychain, the Secret Service through
// libsecret on Linux, Windows' Credential Manager. Where there is none (Linux without libsecret or a Secret Service),
// it fails, and the settings keep the secrets (keychain.go). The secrets go-keyring kept before are not read: a
// device signs in again, and a passphrase is entered again, once.
type osKeychain struct{}

// keychainService is the service the secrets are kept under, whatever the app's identifier.
const keychainService = "celestine"

var secrets = mygo.Secrets.Service(keychainService)

func (osKeychain) Get(name string) (string, error) {
	v, err := secrets.Get(name)
	if errors.Is(err, mygo.ErrSecretNotFound) {
		return "", errNoSecret
	}
	return string(v), err
}

func (osKeychain) Set(name, secret string) error { return secrets.Set(name, []byte(secret)) }
func (osKeychain) Delete(name string) error      { return secrets.Delete(name) }
func systemKeychain() secretKeeper               { return osKeychain{} }
