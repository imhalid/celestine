package main

import (
	"os"
	"strings"
	"testing"

	"github.com/imhalid/ryudit/internal/library"
)

// A test against the real Worker (cloudflare/sync/worker.js), left out unless it is asked for:
//
//	RYUDIT_SYNC_URL=http://localhost:8799 RYUDIT_SYNC_TOKEN=… go test -run TestWorkerLive -v
//
// RYUDIT_SYNC_TOKEN is the device token of a real account made for tests (an invite, then a sign-in:
// the token the app keeps for that device); the Worker knows no other kind of token. What the test writes
// is deleted at the end, but a deletion leaves a tombstone, and a tombstone stays.
func TestWorkerLive(t *testing.T) {
	url, token := os.Getenv("RYUDIT_SYNC_URL"), os.Getenv("RYUDIT_SYNC_TOKEN")
	if url == "" || token == "" {
		t.Skip("RYUDIT_SYNC_URL and RYUDIT_SYNC_TOKEN are not set")
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
