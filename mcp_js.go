//go:build js

package main

// In a browser the app cannot listen on a port: there is no MCP server (mcp_server.go), and the setting says so.

func (a *app) setMCP(on bool) {
	a.settings.MCP = false
	if on {
		a.mcpErr = T("Tarayıcıda MCP sunucusu açılamaz")
	}
}

// runMCP is `ryudit mcp`, which a browser has not.
func runMCP([]string) int { return 1 }
