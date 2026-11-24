//go:build !js

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Adding the app to an agent app's file makes the file when there is none, keeps what else it holds, changes
// nothing the second time, and leaves a file that is not JSON as it is (#67).
func TestAddMCPServer(t *testing.T) {
	dir := t.TempDir()
	fresh := filepath.Join(dir, "Claude", "claude_desktop_config.json")
	if changed, err := addMCPServer(fresh, "/opt/celestine"); err != nil || !changed {
		t.Fatalf("a new file: %v %v", changed, err)
	}
	var conf struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	b, _ := os.ReadFile(fresh)
	json.Unmarshal(b, &conf)
	if s := conf.MCPServers["celestine"]; s.Command != "/opt/celestine" || len(s.Args) != 1 || s.Args[0] != "mcp" {
		t.Fatalf("written: %s", b)
	}
	if changed, err := addMCPServer(fresh, "/opt/celestine"); err != nil || changed {
		t.Fatalf("the second time: %v %v", changed, err)
	}
	// a file of the agent app's own, with another server and a setting of its own
	own := filepath.Join(dir, "mcp.json")
	os.WriteFile(own, []byte(`{"theme":"dark","mcpServers":{"other":{"command":"x"}}}`), 0o600)
	if _, err := addMCPServer(own, "/opt/celestine"); err != nil {
		t.Fatal(err)
	}
	var kept map[string]any
	b, _ = os.ReadFile(own)
	json.Unmarshal(b, &kept)
	servers, _ := kept["mcpServers"].(map[string]any)
	if kept["theme"] != "dark" || servers["other"] == nil || servers["celestine"] == nil {
		t.Fatalf("what was there is not kept: %s", b)
	}
	// not JSON: left alone
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte("{ not json"), 0o600)
	if _, err := addMCPServer(bad, "/opt/celestine"); err == nil {
		t.Fatal("a file that is not JSON was written over")
	}
	if b, _ := os.ReadFile(bad); string(b) != "{ not json" {
		t.Fatalf("changed: %q", b)
	}
	// its order and its numbers as written; through a link, the file it points at, in its mode
	ordered := filepath.Join(dir, "real.json")
	os.WriteFile(ordered, []byte(`{"zeta":1,"big":12345678901234567890,"mcpServers":{"b":{"command":"y"},"a":{"command":"x"}},"alpha":true}`), 0o640)
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(ordered, link); err != nil {
		t.Skip("no symbolic links here")
	}
	if _, err := addMCPServer(link, "/opt/celestine"); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the link was written over")
	}
	b, _ = os.ReadFile(ordered)
	keys := []string{`"zeta"`, `"big": 12345678901234567890`, `"mcpServers"`, `"b"`, `"a"`, `"celestine"`, `"alpha"`}
	at := 0
	for _, k := range keys {
		i := strings.Index(string(b[at:]), k)
		if i < 0 {
			t.Fatalf("%s not after what comes before it: %s", k, b)
		}
		at += i
	}
	if fi, _ := os.Stat(ordered); fi.Mode().Perm() != 0o640 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".celestine-*")); len(left) > 0 {
		t.Fatalf("left behind: %v", left)
	}
	// mcpServers null: as none
	null := filepath.Join(dir, "null.json")
	os.WriteFile(null, []byte(`{"mcpServers":null}`), 0o600)
	if changed, err := addMCPServer(null, "/opt/celestine"); err != nil || !changed {
		t.Fatalf("null servers: %v %v", changed, err)
	}
}
