//go:build !js

package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The bridge of `celestine mcp` carries an agent's messages to the app's server and back: an agent on one side lists
// the app's tools and calls one, through it.
func TestMCPBridge(t *testing.T) {
	a, _ := newTestApp(t, "")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: a.mcpHandler()}
	go srv.Serve(ln)
	defer srv.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	agentSide, bridgeSide := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	go mcpRelay(ctx, bridgeSide, mcpClientTransport(port, a.settings.Token))
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "agent", Version: "0"}, nil).Connect(ctx, agentSide, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(tools.Tools, func(tl *mcp.Tool) bool { return tl.Name == "list_collections" }) {
		t.Fatalf("the app's tools did not come through: %d", len(tools.Tools))
	}
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_collections"})
	if err != nil || res.IsError {
		t.Fatalf("a call through the bridge: %v %+v", err, res)
	}
	b, _ := json.Marshal(res.StructuredContent)
	if len(b) < 3 {
		t.Fatalf("no answer: %s", b)
	}
}

// The bridge reads the port and the token as the app reads its settings, from the library's file; and --mcp is taken
// off the command line, saying it was there.
func TestMCPSettingsAndFlag(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "k.db")
	st, err := OpenStore(file)
	if err != nil {
		t.Fatal(err)
	}
	a := &app{settingsStore: st}
	a.loadSettings()
	a.settings.Port = 7999
	a.saveSettings()
	token := a.settings.Token
	st.Close()
	port, tok, err := mcpSettings([]string{file})
	if err != nil || port != 7999 || tok != token || tok == "" {
		t.Fatalf("settings read: %d %q %v (want %q)", port, tok, err, token)
	}
	if _, _, err := mcpSettings([]string{filepath.Join(dir, "none.db")}); err == nil {
		t.Fatal("a library that is not there was read")
	}
	args, on := withoutMCPFlag([]string{"celestine", mcpFlag, "k.db"})
	if !on || !slices.Equal(args, []string{"celestine", "k.db"}) {
		t.Fatalf("flag: %v %v", args, on)
	}
	if _, on := withoutMCPFlag([]string{"celestine"}); on {
		t.Fatal("a flag that was not there")
	}
}

func TestMCPSettingsReadOnlyNoTokenWrite(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "k.db")
	st, err := OpenStore(file)
	if err != nil {
		t.Fatal(err)
	}
	wantRaw := []byte(`{"port":7999}`)
	if err := st.SetMeta(SettingsKey, wantRaw); err != nil {
		t.Fatal(err)
	}
	st.Close()

	prev := keychain
	spy := &spyKeychain{}
	keychain = spy
	defer func() { keychain = prev }()

	port, tok, err := mcpSettings([]string{file})
	if err != nil {
		t.Fatal(err)
	}
	if port != 7999 || tok != "" {
		t.Fatalf("got port=%d tok=%q, want 7999 and empty", port, tok)
	}
	st, err = OpenStoreReadOnly(file)
	if err != nil {
		t.Fatal(err)
	}
	gotRaw := st.Meta(SettingsKey)
	st.Close()
	if string(gotRaw) != string(wantRaw) {
		t.Fatalf("read-only mcpSettings modified DB meta: got %s, want %s", gotRaw, wantRaw)
	}

	// Legacy token in settings must not be moved into keychain or stripped from DB during read-only read.
	st, err = OpenStore(file)
	if err != nil {
		t.Fatal(err)
	}
	legacyRaw := []byte(`{"port":7999,"token":"legacytok"}`)
	if err := st.SetMeta(SettingsKey, legacyRaw); err != nil {
		t.Fatal(err)
	}
	st.Close()

	port, tok, err = mcpSettings([]string{file})
	if err != nil || port != 7999 || tok != "legacytok" {
		t.Fatalf("legacy read: port=%d tok=%q err=%v", port, tok, err)
	}
	if spy.sets != 0 {
		t.Fatalf("read-only mcpSettings wrote to keychain %d times", spy.sets)
	}
	st, err = OpenStoreReadOnly(file)
	if err != nil {
		t.Fatal(err)
	}
	gotLegacy := st.Meta(SettingsKey)
	st.Close()
	if string(gotLegacy) != string(legacyRaw) {
		t.Fatalf("read-only mcpSettings modified legacy DB meta: got %s", gotLegacy)
	}
}

func TestMCPRelayReturnsErrorOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	aClient, bridgeSide := mcp.NewInMemoryTransports()
	ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel1()
	tr := &mcp.StreamableClientTransport{
		Endpoint:   srv.URL,
		HTTPClient: &http.Client{Transport: bearer{"bad"}},
	}
	errCh1 := make(chan error, 1)
	go func() {
		errCh1 <- mcpRelay(ctx1, bridgeSide, tr)
	}()
	// Trigger an initialize request through the bridge so StreamableClientTransport hits the 401 server.
	go func() {
		_, _ = mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx1, aClient, nil)
	}()
	if err := <-errCh1; err == nil {
		t.Fatal("expected error from mcpRelay on 401 server")
	}

	// Server closing unexpectedly before the agent disconnects must return a non-nil error (not nil).
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	aClient2, aBridge2 := mcp.NewInMemoryTransports()
	sBridge2, sServer2 := mcp.NewInMemoryTransports()
	errCh2 := make(chan error, 1)
	go func() {
		errCh2 <- mcpRelay(ctx2, aBridge2, sBridge2)
	}()
	cConn2, err := aClient2.Connect(ctx2)
	if err != nil {
		t.Fatal(err)
	}
	defer cConn2.Close()
	sConn2, err := sServer2.Connect(ctx2)
	if err != nil {
		t.Fatal(err)
	}
	sConn2.Close()
	if err := <-errCh2; err == nil || (!errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF)) {
		t.Fatalf("expected server EOF error, got %v", err)
	}
}

// TestMCPBridgeAppImageEnv: the bridge does not pass on APPIMAGE and APPDIR that another AppImage (the agent) gave
// it to the app it starts, unless it runs from an AppImage itself (#527).
func TestMCPBridgeAppImageEnv(t *testing.T) {
	env := []string{"HOME=/home/u", "APPIMAGE=/opt/Cursor.AppImage", "APPDIR=/tmp/.mount_Cursor", "APPIMAGE_EXTRA=1"}
	if got := mcpAppEnv(env, false); !slices.Equal(got, []string{"HOME=/home/u", "APPIMAGE_EXTRA=1"}) {
		t.Errorf("mcpAppEnv = %v", got)
	}
	if got := mcpAppEnv(env, true); !slices.Equal(got, env) {
		t.Errorf("mcpAppEnv, its own AppImage = %v", got)
	}
}

// TestMCPBridgeRefusesImpostor: a process that took the port and does not hold the token is never sent it. Whether
// it does not know the handshake, answers it with nonsense, or answers it under another token, the bridge sends it
// its nonce and nothing more, and ends (#170).
func TestMCPBridgeRefusesImpostor(t *testing.T) {
	const token = "the-real-token-0123456789abcdef"
	answers := map[string]func(w http.ResponseWriter, body []byte){
		"no handshake": func(w http.ResponseWriter, _ []byte) { http.Error(w, "not found", http.StatusNotFound) },
		"nonsense":     func(w http.ResponseWriter, _ []byte) { w.Write([]byte(`{"nonce":"zz","proof":"00"}`)) },
		"another token": func(w http.ResponseWriter, body []byte) {
			var in mcpHello
			json.Unmarshal(body, &in)
			cn, _ := hex.DecodeString(in.Nonce)
			sn := mcpNonce()
			json.NewEncoder(w).Encode(mcpHello{Nonce: hex.EncodeToString(sn), Proof: hex.EncodeToString(mcpProof("another", "server", cn, sn))})
		},
	}
	for name, answer := range answers {
		t.Run(name, func(t *testing.T) {
			var mu sync.Mutex
			var seen [][]byte
			var paths []string
			fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewReader(body))
				dump, _ := httputil.DumpRequest(r, true)
				mu.Lock()
				seen, paths = append(seen, dump), append(paths, r.URL.Path)
				mu.Unlock()
				answer(w, body)
			}))
			defer fake.Close()
			port := fake.Listener.Addr().(*net.TCPAddr).Port

			agentSide, bridgeSide := mcp.NewInMemoryTransports()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- mcpRelay(ctx, bridgeSide, mcpClientTransport(port, token)) }()
			go mcp.NewClient(&mcp.Implementation{Name: "agent", Version: "0"}, nil).Connect(ctx, agentSide, nil)
			if err := <-done; err == nil {
				t.Fatal("the bridge went on with a server that did not prove it holds the token")
			}
			mu.Lock()
			defer mu.Unlock()
			if len(seen) == 0 {
				t.Fatal("no handshake was tried")
			}
			for i, dump := range seen {
				if bytes.Contains(dump, []byte(token)) || bytes.Contains(bytes.ToLower(dump), []byte("authorization")) {
					t.Fatalf("the impostor was sent the token or a proof:\n%s", dump)
				}
				if paths[i] != mcpHelloPath {
					t.Fatalf("the impostor was sent more than the handshake: %s", paths[i])
				}
			}
		})
	}
}

// TestMCPBridgeProofs: the server takes a request whose proof was made with its token on a nonce it gave, and refuses
// one made with another token, on a nonce it did not give, or on one it gave too long ago (#170).
func TestMCPBridgeProofs(t *testing.T) {
	a, _ := newTestApp(t, "")
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	a.clock = func() time.Time { return now }
	h := a.mcpHandler()
	serve := func(path, auth, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://127.0.0.1:7457"+path, strings.NewReader(body))
		r.Host = "127.0.0.1:7457"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		if auth != "" {
			r.Header.Set("Authorization", auth)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	hello := func() (cn, sn []byte) {
		cn = mcpNonce()
		w := serve(mcpHelloPath, "", `{"nonce":"`+hex.EncodeToString(cn)+`"}`)
		var in mcpHello
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &in) != nil {
			t.Fatalf("hello: %d %s", w.Code, w.Body)
		}
		sn, _ = hex.DecodeString(in.Nonce)
		if proof, _ := hex.DecodeString(in.Proof); !hmac.Equal(proof, mcpProof(a.settings.Token, "server", cn, sn)) {
			t.Fatal("the server's proof is not made with its token")
		}
		return cn, sn
	}
	auth := func(token string, cn, sn []byte) string {
		return mcpProofScheme + " " + hex.EncodeToString(sn) + " " + hex.EncodeToString(mcpProof(token, "client", cn, sn))
	}
	init := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`

	cn, sn := hello()
	if c := serve("/mcp", auth("wrong", cn, sn), init).Code; c != http.StatusUnauthorized {
		t.Fatalf("a proof with the wrong token: %d", c)
	}
	if c := serve("/mcp", auth(a.settings.Token, cn, mcpNonce()), init).Code; c != http.StatusUnauthorized {
		t.Fatalf("a proof on a nonce the server did not give: %d", c)
	}
	if c := serve("/mcp", auth(a.settings.Token, cn, sn), init).Code; c != http.StatusOK {
		t.Fatalf("the right proof: %d", c)
	}
	now = now.Add(2 * mcpChallengeLife) // a nonce in use does not run out
	if c := serve("/mcp", auth(a.settings.Token, cn, sn), init).Code; c != http.StatusOK {
		t.Fatalf("the right proof, later: %d", c)
	}
	cn, sn = hello()
	now = now.Add(2 * mcpChallengeLife)
	if c := serve("/mcp", auth(a.settings.Token, cn, sn), init).Code; c != http.StatusUnauthorized {
		t.Fatalf("a proof on a nonce that ran out: %d", c)
	}
	if c := serve(mcpHelloPath, "", `{"nonce":"abcd"}`).Code; c != http.StatusBadRequest {
		t.Fatalf("a short nonce: %d", c)
	}
	a.settings.Token = ""
	if c := serve(mcpHelloPath, "", `{"nonce":"`+hex.EncodeToString(mcpNonce())+`"}`).Code; c != http.StatusUnauthorized {
		t.Fatalf("a handshake with no token: %d", c)
	}
}

// TestMCPBridgeChallengesBounded: asking for nonces again and again keeps no more than mcpChallengeMax, and one a
// proof has used is kept over the new ones.
func TestMCPBridgeChallengesBounded(t *testing.T) {
	var cs mcpChallenges
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	cn, sn := mcpNonce(), mcpNonce()
	cs.Add(hex.EncodeToString(sn), cn, now)
	if !cs.Check(hex.EncodeToString(sn), hex.EncodeToString(mcpProof("t", "client", cn, sn)), "t", now) {
		t.Fatal("the right proof was refused")
	}
	for i := range 3 * mcpChallengeMax {
		cs.Add(hex.EncodeToString(mcpNonce()), mcpNonce(), now.Add(time.Duration(i)*time.Millisecond))
	}
	if cs.Len() > mcpChallengeMax {
		t.Fatalf("%d nonces kept", cs.Len())
	}
	if !cs.Check(hex.EncodeToString(sn), hex.EncodeToString(mcpProof("t", "client", cn, sn)), "t", now) {
		t.Fatal("a nonce in use was dropped for new ones")
	}

	// every nonce used, as after many bridges in one run of the app: a new one still comes, in place of the one
	// used longest ago
	cs = mcpChallenges{}
	type pair struct{ cn, sn []byte }
	var pairs []pair
	for i := range mcpChallengeMax {
		p := pair{mcpNonce(), mcpNonce()}
		at := now.Add(time.Duration(i) * time.Second)
		cs.Add(hex.EncodeToString(p.sn), p.cn, at)
		if !cs.Check(hex.EncodeToString(p.sn), hex.EncodeToString(mcpProof("t", "client", p.cn, p.sn)), "t", at) {
			t.Fatal("the right proof was refused")
		}
		pairs = append(pairs, p)
	}
	later := now.Add(time.Hour)
	cn, sn = mcpNonce(), mcpNonce()
	cs.Add(hex.EncodeToString(sn), cn, later)
	if cs.Len() != mcpChallengeMax || !cs.Has(hex.EncodeToString(sn)) {
		t.Fatalf("no room for a new nonce: %d", cs.Len())
	}
	if cs.Has(hex.EncodeToString(pairs[0].sn)) {
		t.Fatal("the nonce used longest ago was kept")
	}
	last := pairs[len(pairs)-1]
	if !cs.Check(hex.EncodeToString(last.sn), hex.EncodeToString(mcpProof("t", "client", last.cn, last.sn)), "t", later) {
		t.Fatal("the nonce used last was dropped")
	}
}

// TestMCPBridgeOlderApp: an app from before the handshake, left running while its binary was replaced, answers the
// hello with 401 or 404; the bridge says the app must be reopened, not that it may be someone else, and a wrong
// proof still reads as an impostor.
func TestMCPBridgeOlderApp(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusNotFound} {
		old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, http.StatusText(code), code)
		}))
		_, err := mcpHandshake(context.Background(), old.URL+mcpHelloPath, "tok")
		old.Close()
		if !errors.Is(err, errMCPOlderApp) || !strings.Contains(err.Error(), "older than this bridge") {
			t.Fatalf("%d: %v", code, err)
		}
	}
	impostor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sn := mcpNonce()
		json.NewEncoder(w).Encode(mcpHello{Nonce: hex.EncodeToString(sn), Proof: hex.EncodeToString(mcpProof("another", "server", sn, sn))})
	}))
	defer impostor.Close()
	_, err := mcpHandshake(context.Background(), impostor.URL+mcpHelloPath, "tok")
	if err == nil || errors.Is(err, errMCPOlderApp) || !strings.Contains(err.Error(), "did not prove") {
		t.Fatalf("a wrong proof: %v", err)
	}
}
