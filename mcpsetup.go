//go:build !js

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Adding the app to the agents' apps in one click (Settings ▸ MCP): Claude Desktop and Cursor keep their MCP
// servers in a JSON file, where the app writes itself in as "celestine", this program with `mcp` over stdio (mcpbridge.go),
// keeping whatever else the file holds; Claude Code is told with its own command. Nothing is written over a file
// that is not JSON.

// mcpServerName is the name the app has among an agent app's servers.
const mcpServerName = "celestine"

// agentCmdWait is how long an agent app's own command is waited for.
const agentCmdWait = 30 * time.Second

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
//
// The file is the agent app's, and is changed only where the app's server goes (#183): its keys stay in their
// order and its values as they were written, numbers too (a round trip through a map sorted the keys and made
// integers past 2^53 floats); a symbolic link is followed and the file it points at written, in its own mode.
func addMCPServer(path, exe string) (bool, error) {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	var top []jsonMember
	mode := os.FileMode(0o600)
	old, err := os.ReadFile(path)
	switch {
	case err == nil && len(bytes.TrimSpace(old)) > 0:
		if top, err = jsonObject(old); err != nil {
			return false, fmt.Errorf("%s is not JSON (%v): left as it is", path, err)
		}
		if fi, err := os.Stat(path); err == nil {
			mode = fi.Mode().Perm()
		}
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return false, err
	}
	want, _ := json.Marshal(map[string]any{"command": exe, "args": []any{"mcp"}})
	var servers []jsonMember
	at := slices.IndexFunc(top, func(m jsonMember) bool { return m.key == "mcpServers" })
	if at >= 0 && string(bytes.TrimSpace(top[at].value)) != "null" {
		if servers, err = jsonObject(top[at].value); err != nil {
			return false, fmt.Errorf("%s: mcpServers is not an object (%v): left as it is", path, err)
		}
	}
	if i := slices.IndexFunc(servers, func(m jsonMember) bool { return m.key == mcpServerName }); i >= 0 {
		var cur any
		if json.Unmarshal(servers[i].value, &cur) == nil {
			if b, _ := json.Marshal(cur); bytes.Equal(b, want) {
				return false, nil
			}
		}
		servers[i].value = want
	} else {
		servers = append(servers, jsonMember{mcpServerName, want})
	}
	if at >= 0 {
		top[at].value = jsonEncode(servers)
	} else {
		top = append(top, jsonMember{"mcpServers", jsonEncode(servers)})
	}
	var out bytes.Buffer
	if err := json.Indent(&out, jsonEncode(top), "", "  "); err != nil {
		return false, err
	}
	out.WriteByte('\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	// written whole or not at all: a half-written file would be the agent app's lost settings
	f, err := os.CreateTemp(dir, ".celestine-*.tmp")
	if err != nil {
		return false, err
	}
	defer os.Remove(f.Name()) // gone once renamed; left by a failure, taken away
	if _, err := f.Write(out.Bytes()); err != nil {
		f.Close()
		return false, err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return false, err
	}
	if err := f.Close(); err != nil {
		return false, err
	}
	return true, os.Rename(f.Name(), path)
}

// jsonMember is a member of a JSON object: its key, and its value as it was written.
type jsonMember struct {
	key   string
	value json.RawMessage
}

// jsonObject is the members of the JSON object b, in their order, each value as it was written.
func jsonObject(b []byte) ([]jsonMember, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, errors.New("not an object")
	}
	var out []jsonMember
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := tok.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		out = append(out, jsonMember{key, v})
	}
	if tok, err := dec.Token(); err != nil || tok != json.Delim('}') {
		return nil, errors.New("not an object")
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("more after the object")
	}
	return out, nil
}

// jsonEncode is the object of members ms, compact.
func jsonEncode(ms []jsonMember) []byte {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, m := range ms {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(m.key)
		b.Write(k)
		b.WriteByte(':')
		b.Write(m.value)
	}
	b.WriteByte('}')
	return b.Bytes()
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
			return "", errors.New(locales.McpClaudeCodeNotInstalled())
		}
		ctx, cancel := context.WithTimeout(context.Background(), agentCmdWait)
		defer cancel()
		out, err := exec.CommandContext(ctx, claude, "mcp", "add", "--scope", "user", mcpServerName, "--", exe, "mcp").CombinedOutput()
		if ctx.Err() != nil {
			return "", fmt.Errorf("claude mcp add: %w", ctx.Err())
		}
		if err != nil && strings.Contains(strings.ToLower(string(out)), "already exists") {
			return locales.McpClaudeCodeAlreadyHas(), nil
		}
		if err != nil {
			return "", fmt.Errorf("claude mcp add: %s", strings.TrimSpace(string(out)))
		}
		return locales.McpAddedClaudeCode(), nil
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
		return locales.McpAlreadyThere(ag.name), nil
	}
	return locales.McpAdded(ag.name, path), nil
}

// mcpSetupRow is the settings' row of the buttons that add the app to the agents' apps.
func (a *app) mcpSetupRow(c *ui.Context) func() {
	return func() {
		settingRow(c, locales.McpAddAgentApp(), locales.McpWritesAppIntoApps(), func() {
			ui.Row(c).Gap(6).Children(func() {
				for _, ag := range agentApps {
					if ryugo.Button(c.Key("mcp-add:"+ag.name), ag.name, ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
						go func() { // away from the frame: Claude Code's command may take its time (#183)
							defer a.guard()
							msg, err := a.addToAgentApp(ag)
							if err != nil {
								msg = err.Error()
							}
							a.do(func() error { a.sayDone(msg, err == nil); return nil })
						}()
					}
				}
			})
		})
	}
}
