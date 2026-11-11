//go:build js

package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/imhalid/celestine/locales"
)

// In a browser the app cannot listen on a port: there is no MCP server (mcp_server.go), and the setting says so.

func (a *app) setMCP(on bool) {
	a.settings.MCP = false
	if on {
		a.mcpErr = locales.McpABrowserCannotRun()
	}
}

// mcpSetupRow is the row adding the app to the agents' apps (mcpsetup.go): a browser cannot write their files.
func (a *app) mcpSetupRow(*ui.Context) func() { return func() {} }

// runMCP is `celestine mcp`, which a browser has not.
func runMCP([]string) int { return 1 }
