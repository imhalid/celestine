//go:build !js

package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
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
