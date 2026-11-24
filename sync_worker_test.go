package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/internal/library"
)

// A test against the real Worker (cloudflare/sync/worker.js), left out unless it is asked for:
//
//	CELESTINE_SYNC_URL=http://localhost:8799 CELESTINE_SYNC_TOKEN=… go test -run TestWorkerLive -v
//
// or, against a Worker of this machine with an administrator's key, scripts/worker-test.sh, which makes the account.
//
// CELESTINE_SYNC_TOKEN is the device token of a real account made for tests (an invite, then a sign-in:
// the token the app keeps for that device); the Worker knows no other kind of token. What the test writes
// is deleted at the end, but a deletion leaves a tombstone, and a tombstone stays.
func TestWorkerLive(t *testing.T) {
	url, token := os.Getenv("CELESTINE_SYNC_URL"), os.Getenv("CELESTINE_SYNC_TOKEN")
	if url != "" && token == "" && os.Getenv("CELESTINE_ADMIN_TOKEN") != "" { // an account of its own (scripts/worker-test.sh)
		token, _, _ = liveAccount(t, url, os.Getenv("CELESTINE_ADMIN_TOKEN"))
	}
	if url == "" || token == "" {
		t.Skip("CELESTINE_SYNC_URL and CELESTINE_SYNC_TOKEN (or CELESTINE_ADMIN_TOKEN) are not set")
	}
	library.AllowHTTP = strings.HasPrefix(url, "http://") // a Worker on this machine (wrangler dev) is plain HTTP
	defer func() { library.AllowHTTP = false }()
	remote := func() Remote { return NewHTTPRemote(url, token) }

	// plain: a file goes up, comes down on another device, is changed, is deleted
	a, b := newDevice(t), newDevice(t)
	a.write("Canlı deneme/Türkçe ad/page.json", "bir")
	if r := a.sync(remote()); r.Pushed != 1 {
		t.Fatalf("push: %+v", r)
	}
	if r := b.sync(remote()); r.Pulled != 1 || b.read("Canlı deneme/Türkçe ad/page.json") != "bir" {
		t.Fatalf("pull: %+v", r)
	}
	b.write("Canlı deneme/Türkçe ad/page.json", "iki")
	b.sync(remote())
	if r := a.sync(remote()); r.Pulled != 1 || a.read("Canlı deneme/Türkçe ad/page.json") != "iki" {
		t.Fatalf("update: %+v", r)
	}
	// both change: the remote's stays, ours is kept beside it
	a.write("Canlı deneme/Türkçe ad/page.json", "a'nın")
	b.write("Canlı deneme/Türkçe ad/page.json", "b'nin")
	b.sync(remote())
	if r := a.sync(remote()); len(r.Conflicts) != 1 {
		t.Fatalf("conflict: %+v", r)
	}
	if a.read("Canlı deneme/Türkçe ad/page.json") != "b'nin" {
		t.Fatal("the remote's did not stay")
	}
	a.sync(remote()) // the copy goes up
	b.sync(remote())

	// sealed: the Worker holds nothing readable
	enc := func() Remote {
		e, err := OpenEnc(remote(), "parola")
		if err != nil {
			t.Fatal(err)
		}
		return e
	}
	c, d := newDevice(t), newDevice(t)
	c.write("Şifreli/Gizli not/page.json", "çok gizli")
	if r := c.sync(enc()); r.Pushed != 1 {
		t.Fatalf("sealed push: %+v", r)
	}
	if r := d.sync(enc()); r.Pulled != 1 || d.read("Şifreli/Gizli not/page.json") != "çok gizli" {
		t.Fatalf("sealed pull: %+v", r)
	}
	if r := c.sync(enc()); r.Pushed+r.Pulled+len(r.Conflicts) != 0 {
		t.Fatalf("a second sealed pass did something: %+v", r)
	}
	list, err := remote().List()
	if err != nil {
		t.Fatal(err)
	}
	for n := range list {
		if strings.Contains(n, "Gizli") || strings.Contains(n, "Şifreli") {
			t.Fatalf("a readable name on the Worker: %q", n)
		}
	}

	// clean up: whatever is on the Worker goes
	list, _ = remote().List()
	for n, e := range list {
		if !e.Deleted {
			if err := remote().Delete(n, e.Hash); err != nil {
				t.Errorf("clean up %s: %v", n, err)
			}
		}
	}
}

// liveAccount opens an account on the Worker at base, as its administrator approves one, and gives its token, its
// data key and its id.
func liveAccount(t *testing.T, base, admin string) (token string, key []byte, id int) {
	t.Helper()
	library.AllowHTTP = strings.HasPrefix(base, "http://")
	t.Cleanup(func() { library.AllowHTTP = false })
	email := fmt.Sprintf("canli-%d@example.com", time.Now().UnixNano())
	var added struct {
		ID int `json:"id"`
	}
	if err := adminCall(base, admin, "POST", "/api/admin/waitlist", map[string]string{"email": email}, &added); err != nil {
		t.Fatal(err)
	}
	var approved struct {
		Code string `json:"code"`
	}
	if err := adminCall(base, admin, "POST", fmt.Sprintf("/api/admin/waitlist/%d/approve", added.ID), struct{}{}, &approved); err != nil {
		t.Fatal(err)
	}
	token, key, err := newAcctClient(base).Register(email, "canli-parola-1", approved.Code)
	if err != nil {
		t.Fatal(err)
	}
	var users []struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
	}
	if err := adminCall(base, admin, "GET", "/api/admin/users", nil, &users); err != nil {
		t.Fatal(err)
	}
	for _, u := range users {
		if u.Username == email {
			id = u.ID
		}
	}
	return token, key, id
}

// adminCall calls the Worker's administrator's API.
func adminCall(base, admin, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, base+path, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+admin)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s %s: %d", method, path, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// The Worker's limits (#164): an account's quota refuses a write past it and counts nothing for it, a write that
// lost a race gives its room back, and a name's sign-ins from one address are counted.
//
//	scripts/worker-test.sh
func TestWorkerLimitsLive(t *testing.T) {
	base, admin := os.Getenv("CELESTINE_SYNC_URL"), os.Getenv("CELESTINE_ADMIN_TOKEN")
	if base == "" || admin == "" {
		t.Skip("CELESTINE_SYNC_URL and CELESTINE_ADMIN_TOKEN are not set")
	}
	token, _, id := liveAccount(t, base, admin)
	if err := adminCall(base, admin, "POST", fmt.Sprintf("/api/admin/users/%d/quota", id), map[string]int{"quota": 3000}, nil); err != nil {
		t.Fatal(err)
	}
	cl, r := newAcctClient(base), NewHTTPRemote(base, token)
	used := func() int64 {
		t.Helper()
		me, err := cl.Me(token)
		if err != nil {
			t.Fatal(err)
		}
		return me.Bytes
	}
	if err := r.Put("bir", bytes.Repeat([]byte{1}, 2000), ""); err != nil {
		t.Fatal(err)
	}
	if n := used(); n != 2000 {
		t.Fatalf("counted %d for a file of 2000", n)
	}
	if err := r.Put("iki", bytes.Repeat([]byte{2}, 2000), ""); !errors.Is(err, library.ErrQuotaFull) {
		t.Fatalf("a write past the quota: %v", err)
	}
	if err := r.Put("üç", bytes.Repeat([]byte{3}, 500), "0000"); err == nil { // against a copy that is not there: stale
		t.Fatal("a write against a copy that is not there was taken")
	}
	if n := used(); n != 2000 {
		t.Fatalf("a refused write kept its room: %d", n)
	}
	// a name's sign-ins from one address: ten in a quarter of an hour, then refused
	name := fmt.Sprintf("yok-%d@example.com", time.Now().UnixNano())
	var last error
	for range 11 {
		_, _, last = cl.Login(name, "yanlış")
	}
	if last == nil || !strings.Contains(last.Error(), "çok fazla") {
		t.Fatalf("the eleventh sign-in: %v", last)
	}
}
