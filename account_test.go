package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/internal/library"
)

func TestAcctKeys(t *testing.T) {
	kdf, err := newKDF()
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"", kdf} { // an account of before, and one stretched with Argon2id (#154)
		a1, k1, err := acctKeys("Ali", "gizli parola", k)
		if err != nil {
			t.Fatal(err)
		}
		a2, k2, _ := acctKeys(" ali ", "gizli parola", k)
		if a1 != a2 || !bytes.Equal(k1, k2) {
			t.Fatal("the name's case and spaces changed the keys")
		}
		a3, k3, _ := acctKeys("ali", "başka parola", k)
		if a1 == a3 || bytes.Equal(k1, k3) {
			t.Fatal("two passwords gave the same keys")
		}
		if len(a1) != 64 || strings.Trim(a1, "0123456789abcdef") != "" {
			t.Fatalf("auth is not 64 hex: %q", a1)
		}
		if bytes.Equal(k1, mustHex(t, a1)) {
			t.Fatal("the key that seals the data key is the proof sent to the server")
		}
		data := bytes.Repeat([]byte{7}, 32)
		w, err := wrapDataKey(k1, data)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := unwrapDataKey(k1, w); err != nil || !bytes.Equal(got, data) {
			t.Fatalf("unwrap: %v", err)
		}
		if _, err := unwrapDataKey(k3, w); err == nil {
			t.Fatal("a wrong key opened the sealed data key")
		}
		if !validWrappedShape(w) {
			t.Fatalf("a sealed key of %d characters is outside what the server takes", len(w))
		}
	}
	other, _ := newKDF()
	a1, _, _ := acctKeys("ali", "gizli parola", kdf)
	if a2, _, _ := acctKeys("ali", "gizli parola", other); a1 == a2 {
		t.Fatal("two salts gave the same proof")
	}
	// a server cannot ask for a stretching cheaper than the library's lowest, nor one that takes the memory
	for _, bad := range []string{"pbkdf2", "argon2id$1$65536$4$00112233445566778899aabbccddeeff", "argon2id$3$1024$4$00112233445566778899aabbccddeeff",
		"argon2id$3$4194304$4$00112233445566778899aabbccddeeff", "argon2id$3$65536$4$0011", "argon2id$3$65536$0$00112233445566778899aabbccddeeff"} {
		if _, _, err := acctKeys("ali", "gizli parola", bad); err == nil {
			t.Errorf("took the stretching %q", bad)
		}
	}
	if recoveryAuth(bytes.Repeat([]byte{7}, 32)) == recoveryAuthOfBefore(bytes.Repeat([]byte{7}, 32)) {
		t.Fatal("the recovery proof is the one of before")
	}
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	var b []byte
	if _, err := fmt.Sscanf(s, "%x", &b); err != nil {
		t.Fatal(err)
	}
	return b
}

// the server takes 20..400 characters of base64
func validWrappedShape(s string) bool { return len(s) >= 20 && len(s) <= 400 }

// The whole life of an account against a real Worker (wrangler dev, or the live one), left out unless asked for:
//
//	CELESTINE_SYNC_URL=http://localhost:8799 CELESTINE_ADMIN_TOKEN=… go test -run TestAccountLive -v
func TestAccountLive(t *testing.T) {
	base, admin := os.Getenv("CELESTINE_SYNC_URL"), os.Getenv("CELESTINE_ADMIN_TOKEN")
	if base == "" || admin == "" {
		t.Skip("CELESTINE_SYNC_URL and CELESTINE_ADMIN_TOKEN are not set")
	}
	library.AllowHTTP = strings.HasPrefix(base, "http://")
	defer func() { library.AllowHTTP = false }()
	cl := newAcctClient(base)
	stamp := fmt.Sprint(time.Now().UnixNano())
	email := "deneme-" + stamp + "@example.com"
	user := email // the account is the address the invite went to

	post := func(path, bearer string, in, out any) error {
		b, _ := json.Marshal(in)
		req, _ := http.NewRequest("POST", base+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			return fmt.Errorf("%s: %d", path, resp.StatusCode)
		}
		if out != nil {
			return json.NewDecoder(resp.Body).Decode(out)
		}
		return nil
	}
	if err := post("/api/admin/waitlist", admin, map[string]string{"email": email}, nil); err != nil { // (the public form has a bot check)
		t.Fatal(err)
	}
	// the administrator finds the one that was just written, and approves it
	req, _ := http.NewRequest("GET", base+"/api/admin/waitlist?status=pending", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("admin list: %v %v", err, resp)
	}
	var list []struct {
		ID    int    `json:"id"`
		Email string `json:"email"`
	}
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	id := 0
	for _, w := range list {
		if w.Email == email {
			id = w.ID
		}
	}
	if id == 0 {
		t.Fatal("the signup is not on the waitlist")
	}
	var approved struct {
		Code string `json:"code"`
	}
	if err := post(fmt.Sprintf("/api/admin/waitlist/%d/approve", id), admin, struct{}{}, &approved); err != nil || !strings.HasPrefix(approved.Code, "RYU-") || len(approved.Code) != 10 {
		t.Fatalf("approve: %v %+v", err, approved)
	}

	// the code is checked with the address before the password is asked for, and only with that address; checking spends nothing
	if err := cl.CheckInvite(email, "RYU-ZZZZZZ"); err == nil {
		t.Fatal("a wrong code was taken")
	}
	if err := cl.CheckInvite("baska-"+email, approved.Code); err == nil {
		t.Fatal("the code was taken with another address")
	}
	for range 2 {
		if err := cl.CheckInvite(strings.ToUpper(email), strings.ToLower(approved.Code)); err != nil {
			t.Fatalf("check invite: %v", err)
		}
	}
	if _, _, err := cl.Register(email, "parola-1234", "RYU-ZZZZZZ"); err == nil {
		t.Fatal("a wrong code opened an account")
	}
	if _, _, err := cl.Register("baska-"+email, "parola-1234", approved.Code); err == nil {
		t.Fatal("the code opened an account for another address")
	}
	tokA, keyA, err := cl.Register(email, "parola-1234", strings.ToLower(approved.Code))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, _, err := cl.Register(email, "parola-1234", approved.Code); err == nil {
		t.Fatal("an invite code was used twice")
	}
	if err := cl.CheckInvite(email, approved.Code); err == nil {
		t.Fatal("a spent code is still taken")
	}
	// a second computer: the same key comes back, opened with the password
	tokB, keyB, err := cl.Login(user, "parola-1234")
	if err != nil || !bytes.Equal(keyA, keyB) {
		t.Fatalf("login: %v, same key: %v", err, bytes.Equal(keyA, keyB))
	}
	if _, _, err := cl.Login(user, "yanlış"); err == nil {
		t.Fatal("a wrong password signed in")
	}
	// a computer named in the list (#161)
	if err := cl.RenameDevice(tokB, "Masaüstü"); err != nil {
		t.Fatal(err)
	}
	if me, err := cl.Me(tokB); err != nil || !slices.ContainsFunc(me.Devices, func(d DeviceInfo) bool { return d.Current && d.Name == "Masaüstü" }) {
		t.Fatalf("the name in the list: %v %+v", err, me.Devices)
	}

	// sealed files go from one computer to the other, through the account's prefix
	a, b := newDevice(t), newDevice(t)
	a.write("Not/Gizli/page.json", "hesap ile şifreli")
	mk := func(tok string, key []byte) Remote {
		e, err := NewEncRemote(NewHTTPRemote(base, tok), key)
		if err != nil {
			t.Fatal(err)
		}
		return e
	}
	if r := a.sync(mk(tokA, keyA)); r.Pushed != 1 {
		t.Fatalf("push: %+v", r)
	}
	if r := b.sync(mk(tokB, keyB)); r.Pulled != 1 || b.read("Not/Gizli/page.json") != "hesap ile şifreli" {
		t.Fatalf("pull: %+v", r)
	}
	me, err := cl.Me(tokA)
	if err != nil || me.Username != user || me.Bytes == 0 || len(me.Devices) != 2 {
		t.Fatalf("me: %v %+v", err, me)
	}

	// changing the password: the data key stays, the other computer is signed out, the old password is no good
	if err := cl.ChangePassword(tokA, user, "yanlış", "yeni-parola-1", keyA); err == nil {
		t.Fatal("the password changed with a wrong old one")
	}
	if err := cl.ChangePassword(tokA, user, "parola-1234", "yeni-parola-1", keyA); err != nil {
		t.Fatal(err)
	}
	if _, err := cl.Me(tokB); err == nil {
		t.Fatal("the other computer stayed signed in")
	}
	if _, _, err := cl.Login(user, "parola-1234"); err == nil {
		t.Fatal("the old password still signs in")
	}
	tokC, keyC, err := cl.Login(user, "yeni-parola-1")
	if err != nil || !bytes.Equal(keyA, keyC) {
		t.Fatalf("new password: %v, same key: %v", err, bytes.Equal(keyA, keyC))
	}
	// removing a device, then deleting the account takes the files with it
	me, _ = cl.Me(tokC)
	for _, d := range me.Devices {
		if !d.Current && d.Name != "" && len(me.Devices) > 1 {
			if err := cl.RemoveDevice(tokC, d.ID); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if err := cl.DeleteAccount(tokC, user, "yanlış"); err == nil {
		t.Fatal("the account was deleted with a wrong password")
	}
	if err := cl.DeleteAccount(tokC, user, "yeni-parola-1"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := cl.Login(user, "yeni-parola-1"); err == nil {
		t.Fatal("a deleted account signed in")
	}
	if _, err := NewHTTPRemote(base, tokC).List(); err == nil {
		t.Fatal("a deleted account's token listed files")
	}
}

// The sync settings, with the server chosen: no account yet, the form is there; with one, the account is.
func TestAccountSettingsPage(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.settings.SyncMode = "server"
	a.openSettings()
	a.settingsTab = tabSync
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	for _, want := range []string{"Giriş yap", "Davetle hesap aç", "E-posta", "Erişim kodu"} { // the invite first
		if !tt.HasText(want) {
			t.Errorf("signed out: no %q: %v", want, tt.Texts())
		}
	}
	if err := tt.Click("Giriş yap"); err != nil { // the sign-in's tab (#286)
		t.Fatal(err)
	}
	tt.Advance(100 * time.Millisecond)
	if a.ac.mode != "login" || !tt.HasText("Şifre") {
		t.Errorf("sign-in: mode %q, no password field: %v", a.ac.mode, tt.Texts())
	}

	a.settings.AccountUser, a.settings.SyncToken, a.ac.meAt = "ali", "tok", time.Now() // no call to a server
	a.ac.me = &AcctMe{Username: "ali", Bytes: 5 << 20, Quota: 200 << 20, Devices: []DeviceInfo{{ID: "aaaaaaaaaaaa", Name: "MacBook", Seen: 1, Current: true}, {ID: "bbbbbbbbbbbb", Name: "Telefon", Seen: 1}}}
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	for _, want := range []string{"Hesap: ali", "Çıkış yap", "Profil", "Bağlantıyı dene"} { // the rest of the account is in the profile
		if !tt.HasText(want) {
			t.Errorf("signed in: no %q: %v", want, tt.Texts())
		}
	}
	for _, not := range []string{"Şifreyi değiştir", "Hesabı sil", "MacBook"} {
		if tt.HasText(not) {
			t.Errorf("signed in: %q is in the settings, it belongs to the profile", not)
		}
	}
	if tt.HasText("Bu cihazın jetonu") || tt.HasText("Uçtan uca şifreleme") {
		t.Error("the token and passphrase rows show with an account")
	}
}

// A forgotten password, and the versions the server keeps, against a real Worker (see TestAccountLive for how to run it).
func TestAccountRecoveryAndVersionsLive(t *testing.T) {
	base, admin := os.Getenv("CELESTINE_SYNC_URL"), os.Getenv("CELESTINE_ADMIN_TOKEN")
	if base == "" || admin == "" {
		t.Skip("CELESTINE_SYNC_URL and CELESTINE_ADMIN_TOKEN are not set")
	}
	library.AllowHTTP = strings.HasPrefix(base, "http://")
	defer func() { library.AllowHTTP = false }()
	cl := newAcctClient(base)
	stamp := fmt.Sprint(time.Now().UnixNano())
	email := "kurtar-" + stamp + "@example.com"
	user := email
	call := func(method, path, bearer string, in, out any) (int, error) {
		b, _ := json.Marshal(in)
		req, _ := http.NewRequest(method, base+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()
		if out != nil && resp.StatusCode/100 == 2 {
			return resp.StatusCode, json.NewDecoder(resp.Body).Decode(out)
		}
		return resp.StatusCode, nil
	}
	// the administrator puts someone on the list by hand, and approves: the id comes back, the code once
	var added struct {
		ID int `json:"id"`
	}
	if code, err := call("POST", "/api/admin/waitlist", admin, map[string]string{"email": email, "name": "Deneme"}, &added); err != nil || code != 200 || added.ID == 0 {
		t.Fatalf("add by hand: %d %v %+v", code, err, added)
	}
	if code, _ := call("POST", "/api/admin/waitlist", "yanlış", map[string]string{"email": email}, nil); code != 401 {
		t.Fatalf("a wrong admin key added someone: %d", code)
	}
	var approved struct {
		Code string `json:"code"`
	}
	if code, err := call("POST", fmt.Sprintf("/api/admin/waitlist/%d/approve", added.ID), admin, struct{}{}, &approved); err != nil || code != 200 || approved.Code == "" {
		t.Fatalf("approve: %d %v", code, err)
	}
	tokA, keyA, err := cl.Register(user, "ilk-parola-1", approved.Code)
	if err != nil {
		t.Fatal(err)
	}

	// the server keeps earlier versions of a file, sealed: the second write keeps the first, a third at once adds none
	enc, err := NewEncRemote(NewHTTPRemote(base, tokA), keyA)
	if err != nil {
		t.Fatal(err)
	}
	const name = "Not/Sayfa/page.json"
	put := func(text string) {
		t.Helper()
		prev := ""
		if l, err := enc.List(); err == nil {
			prev = l[name].Hash
		}
		if err := enc.Put(name, []byte(text), prev); err != nil {
			t.Fatal(err)
		}
	}
	put("bir")
	if v, err := enc.Versions(name); err != nil || len(v) != 0 {
		t.Fatalf("versions of a file written once: %v %v", v, err)
	}
	put("iki")
	put("üç")
	vers, err := enc.Versions(name)
	if err != nil || len(vers) != 1 {
		t.Fatalf("versions after three writes: %v %v", vers, err)
	}
	if b, err := enc.VersionData(name, vers[0].At); err != nil || string(b) != "bir" {
		t.Fatalf("the kept version: %q %v", b, err)
	}
	me, _ := cl.Me(tokA)
	if me.Bytes == 0 {
		t.Fatal("no bytes are counted")
	}
	// another account sees none of it, and a name with a dot-dot is refused
	if _, err := NewHTTPRemote(base, "yok").Versions("x"); err == nil {
		t.Fatal("a token that is not one listed versions")
	}
	if code, _ := call("GET", "/versions/..%2Fu1%2Fx", tokA, nil, nil); code != 400 && code != 404 {
		t.Fatalf("a path out of the account's: %d", code)
	}

	// the password is forgotten: the recovery key sets a new one, the data key and so the files stay, every computer is signed out
	if _, _, err := cl.Recover(user, "AAAA-AAAA", "yeni-parola-2"); err == nil {
		t.Fatal("a recovery key that is not one recovered")
	}
	wrong := RecoveryKey(bytes.Repeat([]byte{9}, 32))
	if _, _, err := cl.Recover(user, wrong, "yeni-parola-2"); err == nil {
		t.Fatal("a wrong recovery key recovered")
	}
	if _, _, err := cl.Recover("yok"+user, RecoveryKey(keyA), "yeni-parola-2"); err == nil {
		t.Fatal("a name that is not there recovered")
	}
	tokR, keyR, err := cl.Recover(user, strings.ToLower(RecoveryKey(keyA)), "yeni-parola-2")
	if err != nil || !bytes.Equal(keyR, keyA) {
		t.Fatalf("recover: %v, same key: %v", err, bytes.Equal(keyR, keyA))
	}
	if _, err := cl.Me(tokA); err == nil {
		t.Fatal("the computer that was signed in stayed so after a recovery")
	}
	if _, _, err := cl.Login(user, "ilk-parola-1"); err == nil {
		t.Fatal("the forgotten password still signs in")
	}
	tokN, keyN, err := cl.Login(user, "yeni-parola-2")
	if err != nil || !bytes.Equal(keyN, keyA) {
		t.Fatalf("login with the new password: %v", err)
	}
	// and what was written before is read with the same key
	again, _ := NewEncRemote(NewHTTPRemote(base, tokN), keyN)
	if b, err := again.Get(name); err != nil || string(b) != "üç" {
		t.Fatalf("a file after the recovery: %q %v", b, err)
	}
	if v, err := again.Versions(name); err != nil || len(v) != 1 {
		t.Fatalf("versions after the recovery: %v %v", v, err)
	}
	// deleting the account takes the versions too
	if err := cl.DeleteAccount(tokR, user, "yeni-parola-2"); err != nil { // the token the recovery gave is a computer like any
		t.Fatal(err)
	}
	if _, err := NewHTTPRemote(base, tokN).Versions(name); err == nil {
		t.Fatal("a deleted account's token listed versions")
	}
}
