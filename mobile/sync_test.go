package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/internal/account"
	"github.com/imhalid/celestine/internal/library"
)

// memSecrets is a secret store of a map.
type memSecrets map[string][]byte

func (m memSecrets) Get(name string) ([]byte, error) {
	if v, ok := m[name]; ok {
		return v, nil
	}
	return nil, errors.New("none")
}
func (m memSecrets) Set(name string, v []byte) error { m[name] = v; return nil }
func (m memSecrets) Delete(name string) error        { delete(m, name); return nil }

// waitFor waits for cond, which a goroutine of the app makes true, at most a minute.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for end := time.Now().Add(time.Minute); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("waited for %s", what)
}

// The phone signs in to an account a computer syncs with and the two libraries become one: a page the computer wrote
// comes to the phone, end-to-end encrypted on the way, the phone's journal and a note it writes go to the computer,
// and nothing conflicts (#634). It needs the sync Worker on this machine (scripts/worker-test.sh).
func TestSyncLive(t *testing.T) {
	base, admin := os.Getenv("CELESTINE_SYNC_URL"), os.Getenv("CELESTINE_ADMIN_TOKEN")
	if base == "" || admin == "" {
		t.Skip("CELESTINE_SYNC_URL and CELESTINE_ADMIN_TOKEN are not set")
	}
	library.AllowHTTP = strings.HasPrefix(base, "http://")
	defer func() { library.AllowHTTP = false }()
	old := secrets
	secrets = memSecrets{}
	defer func() { secrets = old }()

	email, pass := fmt.Sprintf("telefon-%d@example.com", time.Now().UnixNano()), "telefon-parola-1"
	token, key := register(t, base, admin, email, pass)

	// the computer: a library of its own with a note, synced up
	desk := t.TempDir()
	dlib, err := library.OpenLibraryIn(library.OSFS{}, desk)
	if err != nil {
		t.Fatal(err)
	}
	dc, _ := dlib.NewCollection("Masaüstü")
	dp, _ := dc.NewPage(nil, "Bilgisayardan")
	dp.SaveText("Bilgisayarda yazıldı")
	dstate := &library.SyncState{Bases: library.DirBases{Root: filepath.Join(t.TempDir(), "bases")}}
	dsync := func() library.SyncResult {
		r, err := library.NewEncRemote(library.NewHTTPRemote(base, token), key)
		if err != nil {
			t.Fatal(err)
		}
		res, err := library.SyncCounted(library.OSFS{}, desk, r, dstate, map[string]bool{".git": true}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	if res := dsync(); res.Pushed == 0 {
		t.Fatalf("the computer sent nothing: %+v", res)
	}

	// the phone signs in and syncs
	a, _ := newTestApp(t)
	a.settings.SyncURL = base
	a.sy.email, a.sy.pass = email, pass
	a.signIn()
	waitFor(t, "the sign-in", func() bool { return !a.sy.signing })
	if !a.signedIn() || a.sy.signErr != "" {
		t.Fatalf("not signed in: %q", a.sy.signErr)
	}
	waitFor(t, "the first pass", func() bool { return !a.sy.busy })
	if a.sy.err != "" {
		t.Fatalf("pass: %s", a.sy.err)
	}
	var got *library.Page
	for _, p := range a.allPages() {
		if p.ID == dp.ID {
			got = p
		}
	}
	if got == nil || !strings.Contains(plainOf(got), "Bilgisayarda yazıldı") {
		t.Fatalf("the computer's page did not come: %v", a.allPages())
	}

	// the phone writes; the computer gets it
	mp, err := a.makeToday()
	if err != nil {
		t.Fatal(err)
	}
	mp.SaveText("Telefonda yazıldı")
	a.syncNow()
	waitFor(t, "the second pass", func() bool { return !a.sy.busy })
	if res := dsync(); res.Pulled == 0 || len(res.Conflicts) > 0 {
		t.Fatalf("the computer's pass: %+v", res)
	}
	dlib, _ = library.OpenLibraryIn(library.OSFS{}, desk)
	if p := dlib.PageByID(mp.ID); p == nil {
		t.Fatal("the phone's page did not come to the computer")
	} else if md, _ := p.Text(); !strings.Contains(md, "Telefonda yazıldı") {
		t.Fatalf("the phone's page came as %q", md)
	}

	a.signOut()
	if a.signedIn() || len(secrets.(memSecrets)) != 0 {
		t.Fatal("signed out, the secrets stayed")
	}
}

// register opens an account on the Worker at base, as its administrator approves one, as the computer opens it.
func register(t *testing.T, base, admin, email, pass string) (token string, key []byte) {
	t.Helper()
	call := func(method, path string, in, out any) {
		var body io.Reader
		if in != nil {
			b, _ := json.Marshal(in)
			body = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, base+path, body)
		req.Header.Set("Authorization", "Bearer "+admin)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode/100 != 2 {
			t.Fatalf("%s %s: %d %s", method, path, resp.StatusCode, raw)
		}
		if out != nil {
			json.Unmarshal(raw, out)
		}
	}
	var added struct {
		ID int `json:"id"`
	}
	call("POST", "/api/admin/waitlist", map[string]string{"email": email}, &added)
	var approved struct {
		Code string `json:"code"`
	}
	call("POST", fmt.Sprintf("/api/admin/waitlist/%d/approve", added.ID), struct{}{}, &approved)
	token, key, err := account.New(base, nil, func() string { return "Masaüstü" }).Register(email, pass, approved.Code)
	if err != nil {
		t.Fatal(err)
	}
	return token, key
}
