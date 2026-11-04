package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// agent connects an MCP client to the app's server, as an agent does.
func agent(t *testing.T, a *app) *mcp.ClientSession {
	t.Helper()
	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := a.newMCPServer().Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

// call calls tool with args and decodes its JSON answer into out.
func call(t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any, out any) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatal(tool, err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if res.IsError {
		t.Fatalf("%s: %s", tool, text)
	}
	if out != nil {
		if err := json.Unmarshal([]byte(text), out); err != nil {
			t.Fatalf("%s: %v: %s", tool, err, text)
		}
	}
}

// TestMCPTools: an agent makes a collection and pages, reads, finds,
// changes, moves and throws them away, and it is all on disk.
func TestMCPTools(t *testing.T) {
	a, tt := newTestApp(t, "ilk metin")
	cs := agent(t, a)

	var tools *mcp.ListToolsResult
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) != 9 {
		t.Fatalf("tools: %v %d", err, len(tools.Tools))
	}
	var made struct{ ID string }
	call(t, cs, "create_collection", map[string]any{"name": "Okumalar", "statuses": []string{"Okunacak", "Okunuyor", "Okundu"}}, &made)
	if made.ID != "okumalar" {
		t.Fatalf("collection id: %q", made.ID)
	}
	var page, sub struct{ ID string }
	call(t, cs, "create_page", map[string]any{"collection": "Okumalar", "title": "Dune", "markdown": "# Notlar\n\n- çöl\n", "status": "okunuyor"}, &page)
	call(t, cs, "create_page", map[string]any{"collection": "okumalar", "title": "Bölüm 1", "parent": page.ID, "markdown": "Paul."}, &sub)
	if page.ID != "okumalar/dune" || sub.ID != "okumalar/dune/bolum-1" {
		t.Fatalf("ids: %s %s", page.ID, sub.ID)
	}
	var list []pageInfo
	call(t, cs, "list_pages", map[string]any{"collection": "okumalar"}, &list)
	if len(list) != 1 || list[0].Status != "Okunuyor" || len(list[0].Children) != 1 || list[0].Children[0].Title != "Bölüm 1" {
		t.Fatalf("pages: %+v", list)
	}
	var read struct{ Title, Status, Markdown string }
	call(t, cs, "read_page", map[string]any{"page": page.ID}, &read)
	if read.Markdown != "# Notlar\n\n- çöl\n" || read.Status != "Okunuyor" {
		t.Fatalf("read: %+v", read)
	}
	var hits []struct {
		ID string `json:"id"`
	}
	call(t, cs, "search", map[string]any{"query": "PAUL"}, &hits)
	if len(hits) != 1 || hits[0].ID != sub.ID {
		t.Fatalf("search: %+v", hits)
	}
	call(t, cs, "update_page", map[string]any{"page": page.ID, "title": "Dune (1965)", "append": "- baharat"}, nil)
	call(t, cs, "read_page", map[string]any{"page": page.ID}, &read)
	if read.Title != "Dune (1965)" || read.Markdown != "# Notlar\n\n- çöl\n\n- baharat\n" {
		t.Fatalf("updated: %+v", read)
	}
	// a sub-page to the top of the collection, first
	call(t, cs, "move_page", map[string]any{"page": sub.ID, "index": 0}, &sub)
	var colls []collInfo
	call(t, cs, "list_collections", nil, &colls)
	again, _ := OpenLibrary(a.lib.Root)
	for _, c := range again.Collections {
		if c.Name == "Okumalar" && !equal(titles(c.Pages), []string{"Bölüm 1", "Dune (1965)"}) {
			t.Fatalf("on disk: %v", titles(c.Pages))
		}
	}
	if len(colls) != 2 {
		t.Fatalf("collections: %+v", colls)
	}
	call(t, cs, "trash_page", map[string]any{"page": sub.ID}, nil)
	if len(a.lib.Trash()) != 1 {
		t.Fatal("not in the trash")
	}
	// a wrong name is an error the agent reads
	res, _ := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "read_page", Arguments: map[string]any{"page": "yok"}})
	if !res.IsError {
		t.Fatal("no error for a page that is not there")
	}
	tt.Frame()
}

// TestMCPEditsOpenPage: an agent changing the page open in the window
// changes what the window shows, after what was typed there is saved.
func TestMCPEditsOpenPage(t *testing.T) {
	a, tt := newTestApp(t, "ilk")
	cs := agent(t, a)
	tt.Type("x")
	tt.Frame()
	id := a.idOf(a.page.Dir)
	var read struct{ Markdown string }
	call(t, cs, "read_page", map[string]any{"page": id}, &read)
	if read.Markdown != "ilkx\n" {
		t.Fatalf("what was typed: %q", read.Markdown)
	}
	call(t, cs, "update_page", map[string]any{"page": id, "markdown": "Ajan yazdı.\n\n---\n\nSon."}, nil)
	tt.Frame()
	if got := a.doc.Text().String(); got != "Ajan yazdı.\n\nSon." {
		t.Fatalf("the window shows %q", got)
	}
	tt.Advance(2 * time.Second) // no save of the old text over it
	if body, _ := a.page.Body(); body != "Ajan yazdı.\n\n---\n\nSon.\n" {
		t.Fatalf("on disk: %q", body)
	}
}

// TestMCPHTTP: the server answers only with the token, and only on a local name.
func TestMCPHTTP(t *testing.T) {
	a, _ := newTestApp(t, "")
	h := a.mcpHandler()
	init := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`
	try := func(host, auth string) int {
		r := httptest.NewRequest("POST", "http://"+host+"/mcp", strings.NewReader(init))
		r.Host = host
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		if auth != "" {
			r.Header.Set("Authorization", auth)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if c := try("127.0.0.1:7457", ""); c != http.StatusUnauthorized {
		t.Fatalf("no token: %d", c)
	}
	if c := try("127.0.0.1:7457", "Bearer yanlis"); c != http.StatusUnauthorized {
		t.Fatalf("a wrong token: %d", c)
	}
	if c := try("evil.example:7457", "Bearer "+a.settings.Token); c != http.StatusForbidden {
		t.Fatalf("another name: %d", c)
	}
	if c := try("127.0.0.1:7457", "Bearer "+a.settings.Token); c != http.StatusOK {
		t.Fatalf("with the token: %d", c)
	}
}

// TestSettingsDialog turns the server on from the settings and off again.
func TestSettingsDialog(t *testing.T) {
	a, tt := newTestApp(t, "")
	a.settings.Port = 0 // not a port in use: any free one
	if err := tt.Click("Ayarlar"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(500 * time.Millisecond)
	if err := tt.Click("MCP sunucusunu aç"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(300 * time.Millisecond)
	if a.mcpServer == nil || !a.settings.MCP || a.mcpErr != "" {
		t.Fatalf("not on: %q", a.mcpErr)
	}
	if !tt.HasText(a.mcpCommand()) {
		t.Fatal("the line for Claude Code is not shown")
	}
	// an agent over HTTP, with the token
	client := &http.Client{Transport: bearer{a.settings.Token}}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "t", Version: "0"}, nil).
		Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: a.mcpURL(), HTTPClient: client}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var colls []collInfo
	call(t, cs, "list_collections", nil, &colls)
	if len(colls) != 1 || colls[0].Name != "Test" {
		t.Fatalf("over HTTP: %+v", colls)
	}
	cs.Close()
	tt.Click("MCP sunucusunu aç")
	tt.Advance(300 * time.Millisecond)
	if a.mcpServer != nil || a.settings.MCP {
		t.Fatal("not off")
	}
}

type bearer struct{ token string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}
