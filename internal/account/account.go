// Package account speaks to the account part of the sync server (cloudflare/sync/auth.js,
// docs/22-hesap-ve-bekleme-listesi.md), for the desktop app and the phone's alike (#634): an email address, a password
// and, once, the access code the waitlist sent to that address (an account of before has a username where the address
// is). The password does not leave the device. From it, and the address, two things are made here: "auth" (what the
// server takes as proof of who one is) and a key that seals the data key. The data key is made at random when the
// account is opened; the server keeps it sealed and cannot open it. It is the key of end-to-end encryption
// (internal/library's sync): whoever has the password has it on any device, and whoever has the server has nothing.
// Forget the password and the data is gone, unless the recovery key (the data key in words) was kept.
package account

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/imhalid/celestine/internal/library"
)

// DefaultServer is where accounts are opened and the waitlist is.
const DefaultServer = "https://celestine.halid.dev"

// iterations is PBKDF2's for an account of before.
const iterations = 200_000

// An account's password is stretched as its kdf says (#154), which the server keeps beside the account and gives
// before a sign-in (/api/prelogin): "argon2id$time$memory$threads$salt", the salt random and in hex, for an account
// made or given a password since; "" for one of before, whose stretching is PBKDF2 with the address for salt. A
// leak of the server then has what checks a guess at a cost in memory as well as time, and no salt shared by every
// server that has the address.

// NewKDF is the stretching of a password set now: Argon2id at the library's cost, with a salt of its own.
func NewKDF() (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	return fmt.Sprintf("argon2id$%d$%d$%d$%x", library.ArgonTime, library.ArgonMemory, library.ArgonThreads, salt), nil
}

// ParseKDF reads a kdf the server gave. Its costs are held to bounds: a server cannot ask for a stretching that
// costs a guess less than the library's lowest, nor one that takes the device's memory.
func ParseKDF(kdf string) (t, m uint32, p uint8, salt []byte, err error) {
	f := strings.Split(kdf, "$")
	bad := errors.New("the server gave a stretching this app does not take")
	if len(f) != 5 || f[0] != "argon2id" {
		return 0, 0, 0, nil, bad
	}
	nt, e1 := strconv.ParseUint(f[1], 10, 32)
	nm, e2 := strconv.ParseUint(f[2], 10, 32)
	np, e3 := strconv.ParseUint(f[3], 10, 8)
	salt, e4 := hex.DecodeString(f[4])
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || nt < 2 || nt > 10 || nm < 19*1024 || nm > 1<<20 || np < 1 || np > 16 || len(salt) < 16 {
		return 0, 0, 0, nil, bad
	}
	return uint32(nt), uint32(nm), uint8(np), salt, nil
}

// Keys makes, from the name, the password and how the account stretches it, the proof for the server and the key that
// seals the data key.
func Keys(username, password, kdf string) (auth string, kek []byte, err error) {
	var master []byte
	if kdf == "" { // an account of before
		master, err = pbkdf2.Key(sha256.New, password, []byte("celestine/v1/"+Name(username)), iterations, 32)
	} else {
		var t, m uint32
		var p uint8
		var salt []byte
		if t, m, p, salt, err = ParseKDF(kdf); err == nil {
			master = library.StretchPassword(password, salt, t, m, p)
		}
	}
	if err != nil {
		return "", nil, err
	}
	part := func(label string) []byte {
		m := hmac.New(sha256.New, master)
		m.Write([]byte(label))
		return m.Sum(nil)
	}
	return hex.EncodeToString(part("auth")), part("kek"), nil
}

// RecoveryAuth is what the recovery key proves to the server (/api/recover): made from the data key, so that it is
// the same wherever the key is, and one way, so that the server, holding only its hash, cannot make the key from it.
// It is a key of its own made from the data key (library.RecoveryProof, #162); RecoveryAuthOfBefore is what an
// account of before has the hash of.
func RecoveryAuth(key []byte) string { return library.RecoveryProof(key) }

func RecoveryAuthOfBefore(key []byte) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte("celestine/recovery-auth"))
	return hex.EncodeToString(m.Sum(nil))
}

// WrapDataKey seals the data key with kek, as the server keeps it.
func WrapDataKey(kek, key []byte) (string, error) {
	blk, err := aes.NewCipher(kek)
	if err != nil {
		return "", err
	}
	g, err := cipher.NewGCM(blk)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(g.Seal(nonce, nonce, key, nil)), nil
}

// ErrDecrypt is the data key the server gave not opening with the password's key: the server said the proof was good,
// so what it keeps is damaged.
var ErrDecrypt = errors.New("the data key does not open")

// UnwrapDataKey opens the data key the server keeps with kek.
func UnwrapDataKey(kek []byte, s string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, ErrDecrypt
	}
	blk, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(blk)
	if err != nil {
		return nil, err
	}
	if len(raw) < g.NonceSize() {
		return nil, ErrDecrypt
	}
	key, err := g.Open(nil, raw[:g.NonceSize()], raw[g.NonceSize():], nil)
	if err != nil {
		return nil, ErrDecrypt
	}
	return key, nil
}

// Name is an address or a username as the server and the keys take it.
func Name(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// A Client speaks to the account part of the server.
type Client struct {
	base string
	hc   *http.Client
	// Device is the name of this device in the account's list of devices, asked as it signs in.
	Device func() string
	// Messages says ErrDecrypt and a server's answer without words of its own in the app's language; nil keeps them.
	Messages func(err error) error
}

// New is a client of the server at base (DefaultServer when empty), its requests through transport (nil for Go's).
func New(base string, transport http.RoundTripper, device func() string) *Client {
	if strings.TrimSpace(base) == "" {
		base = DefaultServer
	}
	return &Client{base: strings.TrimRight(base, "/"), hc: &http.Client{Timeout: 30 * time.Second, Transport: transport}, Device: device}
}

// A DeviceInfo is a device signed in to the account.
type DeviceInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Created int64  `json:"created"`
	Seen    int64  `json:"seen"`
	Current bool   `json:"current"`
}

// Me is what the server says of the account.
type Me struct {
	Username string       `json:"username"`
	Bytes    int64        `json:"bytes"`
	Quota    int64        `json:"quota"`
	Devices  []DeviceInfo `json:"devices"`
}

// StatusError is a server's answer of a status that is not 2xx and no words of its own.
type StatusError struct{ Status int }

func (e StatusError) Error() string { return "the server answered " + strconv.Itoa(e.Status) }

// said is err in the app's words, as Messages says it.
func (c *Client) said(err error) error {
	if err != nil && c.Messages != nil {
		return c.Messages(err)
	}
	return err
}

func (c *Client) device() string {
	if c.Device != nil {
		if n := strings.TrimSpace(c.Device()); n != "" {
			return n
		}
	}
	return "device"
}

func (c *Client) call(method, path, token string, in, out any) error {
	if err := library.HTTPSOnly(c.base); err != nil {
		return err
	}
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &e) == nil && e.Error != "" {
			return errors.New(e.Error)
		}
		return c.said(StatusError{resp.StatusCode})
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// RenameDevice gives this device, the one of token, another name in the account's list.
func (c *Client) RenameDevice(token, name string) error {
	return c.call("POST", "/api/device", token, map[string]string{"name": name}, nil)
}

// CheckInvite asks whether code is the access code sent to email, unused, and the address has no account yet; it
// spends nothing.
func (c *Client) CheckInvite(email, code string) error {
	return c.call("POST", "/api/invite", "", map[string]string{"email": Name(email), "code": code}, nil)
}

// Register opens an account for email with the access code sent to it. It makes the data key; the token is this
// device's.
func (c *Client) Register(email, password, code string) (token string, key []byte, err error) {
	kdf, err := NewKDF()
	if err != nil {
		return "", nil, err
	}
	auth, kek, err := Keys(email, password, kdf)
	if err != nil {
		return "", nil, err
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", nil, err
	}
	wrapped, err := WrapDataKey(kek, key)
	if err != nil {
		return "", nil, err
	}
	var out struct {
		Token string `json:"token"`
	}
	err = c.call("POST", "/api/register", "", map[string]string{
		"email": Name(email), "auth": auth, "wrapped": wrapped, "code": code, "device": c.device(), "recovery": RecoveryAuth(key), "kdf": kdf,
	}, &out)
	if err != nil {
		return "", nil, err
	}
	return out.Token, key, nil
}

// KDFOf is how the account of username stretches its password, as the server says. A name with no account gets an
// answer like one's, the same each time, so that what the server says does not tell who has an account.
func (c *Client) KDFOf(username string) (string, error) {
	var out struct {
		KDF string `json:"kdf"`
	}
	err := c.call("POST", "/api/prelogin", "", map[string]string{"username": Name(username)}, &out)
	return out.KDF, err
}

// KeysOf is Keys for the account of username, its stretching asked of the server.
func (c *Client) KeysOf(username, password string) (auth string, kek []byte, err error) {
	kdf, err := c.KDFOf(username)
	if err != nil {
		return "", nil, err
	}
	return Keys(username, password, kdf)
}

// Login signs this device in and brings the data key back, opened here.
func (c *Client) Login(username, password string) (token string, key []byte, err error) {
	auth, kek, err := c.KeysOf(username, password)
	if err != nil {
		return "", nil, err
	}
	var out struct {
		Token   string `json:"token"`
		Wrapped string `json:"wrapped"`
	}
	if err := c.call("POST", "/api/login", "", map[string]string{"username": Name(username), "auth": auth, "device": c.device()}, &out); err != nil {
		return "", nil, err
	}
	key, err = UnwrapDataKey(kek, out.Wrapped)
	if err == nil {
		// an account made before the recovery key could be used has none on the server: it is put there now, with
		// the proof of the password. The server takes it once; a failure here is not one of the sign-in.
		c.call("POST", "/api/recovery", out.Token, map[string]string{"auth": auth, "recovery": RecoveryAuth(key)}, nil)
	}
	return out.Token, key, c.said(err)
}

// Recover is for a forgotten password: the recovery key (the words that were shown when the account was made) says
// who one is, and a new password is set; the data key stays, so what is on the server is still readable. Every other
// device is signed out.
func (c *Client) Recover(username, recovery, newPassword string) (token string, key []byte, err error) {
	key, err = library.KeyFromRecovery(recovery)
	if err != nil {
		return "", nil, err
	}
	kdf, err := NewKDF()
	if err != nil {
		return "", nil, err
	}
	newAuth, newKek, err := Keys(username, newPassword, kdf)
	if err != nil {
		return "", nil, err
	}
	wrapped, err := WrapDataKey(newKek, key)
	if err != nil {
		return "", nil, err
	}
	var out struct {
		Token string `json:"token"`
	}
	// an account of before has the hash of the proof of before: the server takes either, and keeps this one's
	err = c.call("POST", "/api/recover", "", map[string]string{
		"username": Name(username), "recovery": RecoveryAuth(key), "recoveryBefore": RecoveryAuthOfBefore(key),
		"newAuth": newAuth, "newWrapped": wrapped, "newKdf": kdf, "device": c.device(),
	}, &out)
	if err != nil {
		return "", nil, err
	}
	return out.Token, key, nil
}

// Me is what the server says of the account of token.
func (c *Client) Me(token string) (Me, error) {
	var me Me
	return me, c.call("GET", "/api/me", token, nil, &me)
}

// Logout signs the device of token out.
func (c *Client) Logout(token string) error {
	return c.call("POST", "/api/logout", token, struct{}{}, nil)
}

// RemoveDevice signs the device id out of the account.
func (c *Client) RemoveDevice(token, id string) error {
	return c.call("DELETE", "/api/devices/"+id, token, struct{}{}, nil)
}

// ChangePassword seals the data key anew under the new password; the server signs the other devices out.
func (c *Client) ChangePassword(token, username, oldPassword, newPassword string, key []byte) error {
	oldAuth, _, err := c.KeysOf(username, oldPassword)
	if err != nil {
		return err
	}
	kdf, err := NewKDF() // a password set anew is stretched as one set now: an account of before moves on here
	if err != nil {
		return err
	}
	newAuth, newKek, err := Keys(username, newPassword, kdf)
	if err != nil {
		return err
	}
	wrapped, err := WrapDataKey(newKek, key)
	if err != nil {
		return err
	}
	return c.call("POST", "/api/password", token, map[string]string{"auth": oldAuth, "newAuth": newAuth, "newWrapped": wrapped, "newKdf": kdf}, nil)
}

// DeleteAccount removes the account and what is on the server.
func (c *Client) DeleteAccount(token, username, password string) error {
	auth, _, err := c.KeysOf(username, password)
	if err != nil {
		return err
	}
	return c.call("DELETE", "/api/account", token, map[string]string{"auth": auth}, nil)
}
