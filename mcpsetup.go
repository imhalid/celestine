//go:build !js

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// Adding the app to the agents' apps in one click (Settings ▸ MCP): Claude Desktop and Cursor keep their MCP
// servers in a JSON file, where the app writes itself in as "ryudit", this program with `mcp` over stdio (mcpbridge.go),
// keeping whatever else the file holds; Claude Code is told with its own command. Nothing is written over a file
// that is not JSON.

// mcpServerName is the name the app has among an agent app's servers.
const mcpServerName = "ryudit"

// agentApp is an app the server can be added to.
type agentApp struct {
	name string
	file func() (string, error) // its config file; nil for one that has a command of its own
}

var agentApps = []agentApp{
	{"Claude Desktop", func() (string, error) {
		dir, err := os.UserConfigDir() // ~/Library/Application Support, %AppData%, ~/.config
		return filepath.Join(dir, "Claude", "claude_desktop_config.json"), err
	}},
	{"Cursor", func() (string, error) {
		home, err := os.UserHomeDir()
		return filepath.Join(home, ".cursor", "mcp.json"), err
	}},
	{"Claude Code", nil},
}

// addMCPServer puts the app among the mcpServers of the JSON file at path, made when there is none: exe with
// `mcp`. It says whether the file changed; one that holds something not JSON is left as it is, with an error.
func addMCPServer(path, exe string) (bool, error) {
	conf := map[string]any{}
	old, err := os.ReadFile(path)
	switch {
	case err == nil && len(bytes.TrimSpace(old)) > 0:
		if err := json.Unmarshal(old, &conf); err != nil {
			return false, fmt.Errorf("%s is not JSON (%v): left as it is", path, err)
		}
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return false, err
	}
	servers, _ := conf["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	want := map[string]any{"command": exe, "args": []any{"mcp"}}
	if cur, ok := servers[mcpServerName]; ok {
		a, _ := json.Marshal(cur)
		b, _ := json.Marshal(want)
		if bytes.Equal(a, b) {
			return false, nil
		}
	}
	servers[mcpServerName] = want
	conf["mcpServers"] = servers
	out, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	// written whole or not at all: a half-written file would be the agent app's lost settings
	tmp := path + ".ryudit-tmp"
	if err := os.WriteFile(tmp, append(out, '\n'), 0o600); err != nil {
		return false, err
	}
	return true, os.Rename(tmp, path)
}

// addToAgentApp adds the app to ag, and says what was done.
func (a *app) addToAgentApp(ag agentApp) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if ag.file == nil { // Claude Code: its command, when it is there
		claude, err := exec.LookPath("claude")
		if err != nil {
			return "", errors.New(T("Claude Code is not installed here: copy the command above into a terminal instead."))
		}
		out, err := exec.Command(claude, "mcp", "add", "--scope", "user", mcpServerName, "--", exe, "mcp").CombinedOutput()
		if err != nil && strings.Contains(strings.ToLower(string(out)), "already exists") {
			return T("Claude Code already has it."), nil
		}
		if err != nil {
			return "", fmt.Errorf("claude mcp add: %s", strings.TrimSpace(string(out)))
		}
		return T("Added to Claude Code."), nil
	}
	path, err := ag.file()
	if err != nil {
		return "", err
	}
	changed, err := addMCPServer(path, exe)
	if err != nil {
		return "", err
	}
	if !changed {
		return Tf("%s already has it.", ag.name), nil
	}
	return Tf("Added to %s (%s). Restart it to connect.", ag.name, path), nil
}

// mcpSetupRow is the settings' row of the buttons that add the app to the agents' apps.
func (a *app) mcpSetupRow(c *ui.Context) func() {
	return func() {
		settingRow(c, "Add to an agent app", "Writes the app into the app's own list of servers (over stdio), keeping what else it has; then restart that app.", func() {
			ui.Row(c).Gap(6).Children(func() {
				for _, ag := range agentApps {
					if ryugo.Button(c.Key("mcp-add:"+ag.name), ag.name, ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						msg, err := a.addToAgentApp(ag)
						if err != nil {
							msg = err.Error()
						}
						a.notice = msg
					}
				}
			})
		})
	}
}
