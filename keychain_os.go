//go:build !js

package main

import keyring "github.com/zalando/go-keyring"

// osKeychain is the system's keychain, through go-keyring: no cgo, it speaks to macOS's security tool, the Secret
// Service over D-Bus and Windows' Credential Manager.
type osKeychain struct{}

const keychainService = "ryudit"

func (osKeychain) Get(name string) (string, error) { return keyring.Get(keychainService, name) }
func (osKeychain) Set(name, secret string) error   { return keyring.Set(keychainService, name, secret) }
func (osKeychain) Delete(name string) error        { return keyring.Delete(keychainService, name) }
func systemKeychain() secretKeeper                 { return osKeychain{} }
