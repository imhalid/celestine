//go:build !js

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
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
		HTTPClient: &http.Client{Transport: bearerToken("bad")},
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
