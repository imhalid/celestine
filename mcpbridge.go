//go:build !js

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// `celestine mcp [library]` is the app's MCP server for an agent that speaks over stdio (as Claude Desktop and most
// agents start a server): it carries the agent's messages to the app's own server (mcp_server.go) and back. The app
// is the only one writing the library, and the user sees what the agent writes at once. When the app is not
// running it is started; when its server is off, it is turned on (the app is started, or reached, with --mcp).

// mcpWait is how long the bridge waits for the app's server to answer.
const mcpWait = 30 * time.Second

// runMCP is `celestine mcp`: it returns the process's exit code.
func runMCP(args []string) int {
	ctx := context.Background()
	if !mcpReachable(args) {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, "celestine mcp:", err)
			return 1
		}
		// the app, or the running app it hands its command line to, turns its server on
		cmd := exec.Command(exe, append([]string{mcpFlag}, args...)...)
		if err := cmd.Start(); err != nil {
			fmt.Fprintln(os.Stderr, "celestine mcp: the app could not be started:", err)
			return 1
		}
		go cmd.Wait()
		deadline := time.Now().Add(mcpWait)
		for !mcpReachable(args) {
			if time.Now().After(deadline) {
				fmt.Fprintln(os.Stderr, "celestine mcp: the app's MCP server did not answer; turn it on in Settings ▸ MCP")
				return 1
			}
			time.Sleep(300 * time.Millisecond)
		}
	}
	port, token, err := mcpSettings(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "celestine mcp:", err)
		return 1
	}
	if token == "" {
		fmt.Fprintln(os.Stderr, "celestine mcp: no MCP token found (unlock keychain or open Settings > MCP)")
		return 1
	}
	if err := mcpRelay(ctx, &mcp.StdioTransport{}, mcpClientTransport(port, token)); err != nil {
		fmt.Fprintln(os.Stderr, "celestine mcp:", err)
		return 1
	}
	return 0
}

// mcpClientTransport is the way to the app's server on port, with its token.
func mcpClientTransport(port int, token string) *mcp.StreamableClientTransport {
	return &mcp.StreamableClientTransport{
		Endpoint:   "http://127.0.0.1:" + strconv.Itoa(port) + "/mcp",
		HTTPClient: &http.Client{Transport: bearerToken(token)},
	}
}

// mcpRelay carries messages between an agent (from) and the app's server (to), each way, until either side ends.
func mcpRelay(ctx context.Context, from, to mcp.Transport) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	agent, err := from.Connect(ctx)
	if err != nil {
		return err
	}
	defer agent.Close()
	server, err := to.Connect(ctx)
	if err != nil {
		return err
	}
	defer server.Close()
	type relayEnd struct {
		side string
		err  error
	}
	done := make(chan relayEnd, 2)
	pump := func(side string, src, dst mcp.Connection) {
		for {
			msg, err := src.Read(ctx)
			if err != nil {
				done <- relayEnd{side, err}
				return
			}
			if err := dst.Write(ctx, msg); err != nil {
				done <- relayEnd{side, err}
				return
			}
		}
	}
	go pump("agent", agent, server)
	go pump("server", server, agent)
	end := <-done // one side ended (the agent went, or the app): the other is closed with it
	if end.side == "agent" && (end.err == nil || errors.Is(end.err, io.EOF)) {
		return nil
	}
	if end.err != nil {
		return end.err
	}
	return errors.New("server closed connection")
}

// mcpReachable is whether the app's server listens, for the library args names.
func mcpReachable(args []string) bool {
	port, _, err := mcpSettings(args)
	if err != nil {
		return false
	}
	c, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// mcpSettings are the port and the token of the app's MCP server, read as the app reads its settings: from the
// library's file, or the settings file of a folder library.
func mcpSettings(args []string) (int, string, error) {
	a := &app{}
	if dir, err := os.UserConfigDir(); err == nil {
		a.settingsFile = filepath.Join(dir, "celestine", "settings.json")
	}
	file := LibraryFile()
	if len(args) > 0 {
		if info, err := os.Stat(args[0]); err == nil && info.IsDir() {
			file = ""
		} else {
			file, _ = filepath.Abs(args[0])
		}
	}
	if file != "" {
		if _, err := os.Stat(file); err != nil {
			return 0, "", fmt.Errorf("no library at %s", file)
		}
		st, err := OpenStoreReadOnly(file)
		if err != nil {
			return 0, "", err
		}
		defer st.Close()
		a.settingsStore = st
	}
	a.loadSettingsReadOnly()
	return a.settings.Port, a.settings.Token, nil
}

// bearerToken sends the token with each request.
type bearerToken string

func (b bearerToken) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+string(b))
	return http.DefaultTransport.RoundTrip(r)
}
