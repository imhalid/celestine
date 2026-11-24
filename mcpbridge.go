//go:build !js

package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
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
		cmd.Env = mcpAppEnv(os.Environ(), appImagePath() != "")
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

// mcpAppEnv is the environment the bridge starts the app with: its own, less APPIMAGE and APPDIR when the bridge
// does not run from an AppImage (ownAppImage), as those were then passed on by another AppImage that started the
// agent, and name that one's file, not the app's (#527).
func mcpAppEnv(env []string, ownAppImage bool) []string {
	if ownAppImage {
		return env
	}
	return withoutMount(env)
}

// mcpClientTransport is the way to the app's server on port: the token never goes to it, the bridge proves it
// holds it once the server has proved the same (mcp_auth.go).
func mcpClientTransport(port int, token string) *mcp.StreamableClientTransport {
	base := "http://127.0.0.1:" + strconv.Itoa(port)
	return &mcp.StreamableClientTransport{
		Endpoint:   base + "/mcp",
		HTTPClient: &http.Client{Transport: &mcpProver{hello: base + mcpHelloPath, token: token}},
	}
}

// mcpProver makes the handshake with the server before its first request and signs each request with the
// bridge's proof. A server that does not prove it holds the token is not sent anything more, and there is no way
// back to sending the token itself: that would be the hole again. When the handshake fails, every request fails
// with it, and the bridge ends; one whose server went (the app restarted) ends too, and the agent starts it again.
type mcpProver struct {
	hello string
	token string
	mu    sync.Mutex
	auth  string // the Authorization of each request, once the server proved itself
	err   error  // why the handshake failed
}

// RoundTrip sends r with the bridge's proof, making the handshake first when there has been none.
func (p *mcpProver) RoundTrip(r *http.Request) (*http.Response, error) {
	p.mu.Lock()
	if p.auth == "" && p.err == nil {
		p.auth, p.err = mcpHandshake(r.Context(), p.hello, p.token)
	}
	auth, err := p.auth, p.err
	p.mu.Unlock()
	if err != nil {
		if r.Body != nil {
			r.Body.Close()
		}
		return nil, err
	}
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", auth)
	return http.DefaultTransport.RoundTrip(r)
}

// errMCPOlderApp is the handshake's error when the server knows no handshake: the running app is older than the
// bridge, and the bridge does not fall back to sending it the token.
var errMCPOlderApp = errors.New("the running Celestine is older than this bridge: quit and reopen Celestine to finish its update")

// mcpHandshake sends a nonce to the server at url, checks that its answer was made with token, and returns the
// Authorization that proves the bridge holds it too.
func mcpHandshake(ctx context.Context, url, token string) (string, error) {
	if token == "" {
		return "", errors.New("no MCP token")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cn := mcpNonce()
	body, _ := json.Marshal(mcpHello{Nonce: hex.EncodeToString(cn)})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	// an app from before the handshake knows no /mcp/hello: its server answers 401 (no token sent) or 404, as one
	// left running while the updater or install.sh replaced its binary does
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound {
		return "", errMCPOlderApp
	}
	refused := errors.New("the server on the MCP port did not prove it holds the token: it may not be Celestine, so the token was not sent")
	if resp.StatusCode != http.StatusOK {
		return "", refused
	}
	var in mcpHello
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&in); err != nil {
		return "", refused
	}
	sn, err := hex.DecodeString(in.Nonce)
	if err != nil || len(sn) != mcpNonceSize {
		return "", refused
	}
	proof, err := hex.DecodeString(in.Proof)
	if err != nil || !hmac.Equal(proof, mcpProof(token, "server", cn, sn)) {
		return "", refused
	}
	return mcpProofScheme + " " + in.Nonce + " " + hex.EncodeToString(mcpProof(token, "client", cn, sn)), nil
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
