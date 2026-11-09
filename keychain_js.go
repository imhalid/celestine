//go:build js

package main

// systemKeychain is none in the browser: the settings keep the secrets, as the browser keeps the settings.
func systemKeychain() secretKeeper { return nil }
