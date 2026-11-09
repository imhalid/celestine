//go:build !js

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Adding the app to an agent app's file makes the file when there is none, keeps what else it holds, changes
// nothing the second time, and leaves a file that is not JSON as it is (#67).
func TestAddMCPServer(t *testing.T) {
	dir := t.TempDir()
	fresh := filepath.Join(dir, "Claude", "claude_desktop_config.json")
	if changed, err := addMCPServer(fresh, "/opt/ryudit"); err != nil || !changed {
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
	if s := conf.MCPServers["ryudit"]; s.Command != "/opt/ryudit" || len(s.Args) != 1 || s.Args[0] != "mcp" {
		t.Fatalf("written: %s", b)
	}
	if changed, err := addMCPServer(fresh, "/opt/ryudit"); err != nil || changed {
		t.Fatalf("the second time: %v %v", changed, err)
	}
	// a file of the agent app's own, with another server and a setting of its own
	own := filepath.Join(dir, "mcp.json")
	os.WriteFile(own, []byte(`{"theme":"dark","mcpServers":{"other":{"command":"x"}}}`), 0o600)
	if _, err := addMCPServer(own, "/opt/ryudit"); err != nil {
		t.Fatal(err)
	}
	var kept map[string]any
	b, _ = os.ReadFile(own)
	json.Unmarshal(b, &kept)
	servers, _ := kept["mcpServers"].(map[string]any)
	if kept["theme"] != "dark" || servers["other"] == nil || servers["ryudit"] == nil {
		t.Fatalf("what was there is not kept: %s", b)
	}
	// not JSON: left alone
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte("{ not json"), 0o600)
	if _, err := addMCPServer(bad, "/opt/ryudit"); err == nil {
		t.Fatal("a file that is not JSON was written over")
	}
	if b, _ := os.ReadFile(bad); string(b) != "{ not json" {
		t.Fatalf("changed: %q", b)
	}
}
