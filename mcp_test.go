package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
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
	if err != nil || len(tools.Tools) != 18 {
		t.Fatalf("tools: %v %d", err, len(tools.Tools))
	}
	var made struct{ ID string }
	call(t, cs, "create_collection", map[string]any{"name": "Okumalar", "properties": []map[string]any{{"name": "Durum", "type": "choice", "options": []string{"Okunacak", "Okunuyor", "Okundu"}}}}, &made)
	if made.ID != "okumalar" {
		t.Fatalf("collection id: %q", made.ID)
	}
	var page, sub struct{ ID string }
	call(t, cs, "create_page", map[string]any{"collection": "Okumalar", "title": "Dune", "markdown": "# Notlar\n\n- çöl\n", "properties": map[string]string{"durum": "okunuyor"}}, &page)
	call(t, cs, "create_page", map[string]any{"collection": "okumalar", "title": "Bölüm 1", "parent": page.ID, "markdown": "Paul."}, &sub)
	if page.ID != "okumalar/dune" || sub.ID != "okumalar/dune/bolum-1" {
		t.Fatalf("ids: %s %s", page.ID, sub.ID)
	}
	var list []pageInfo
	call(t, cs, "list_pages", map[string]any{"collection": "okumalar"}, &list)
	if len(list) != 1 || list[0].Properties["Durum"] != "Okunuyor" || len(list[0].Children) != 1 || list[0].Children[0].Title != "Bölüm 1" {
		t.Fatalf("pages: %+v", list)
	}
	var read struct {
		Title, Markdown string
		Properties      map[string]string
	}
	call(t, cs, "read_page", map[string]any{"page": page.ID}, &read)
	if read.Markdown != "# Notlar\n\n- çöl\n" || read.Properties["Durum"] != "Okunuyor" {
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
	// a property added, and a value of it
	call(t, cs, "add_property", map[string]any{"collection": "okumalar", "name": "Yazar", "type": "text"}, nil)
	call(t, cs, "update_page", map[string]any{"page": page.ID, "properties": map[string]string{"Yazar": "Frank Herbert"}}, nil)
	res, _ := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "update_page", Arguments: map[string]any{"page": page.ID, "properties": map[string]string{"Yok": "x"}}})
	if !res.IsError {
		t.Fatal("a value of a property that is not there")
	}
	call(t, cs, "read_page", map[string]any{"page": page.ID}, &read)
	if read.Properties["Yazar"] != "Frank Herbert" || read.Title != "Dune (1965)" || read.Markdown != "# Notlar\n\n- çöl\n- baharat\n" {
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
	res, _ = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "read_page", Arguments: map[string]any{"page": "yok"}})
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
	if body, _ := a.page.Text(); body != "Ajan yazdı.\n\n---\n\nSon.\n" {
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
	a.settings.Port = 0                         // not a port in use: any free one
	if err := tt.Click("Ayarlar"); err != nil { // the gear: a short menu, the dialog at its end
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click("Tüm ayarlar…"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(500 * time.Millisecond)
	a.settingsTab = tabMCP
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
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

// TestMCPInStatusBar: the server on shows in the status bar, which opens the settings.
func TestMCPInStatusBar(t *testing.T) {
	a, tt := newTestApp(t, "")
	if tt.HasText("MCP") {
		t.Fatal("shown while off")
	}
	a.settings.Port = 0
	a.setMCP(true)
	defer a.setMCP(false)
	tt.Frame()
	if err := tt.Click("MCP açık: ayarlar"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.settingsOpen {
		t.Fatal("the settings did not open")
	}
}

// TestMCPTagsTasksVersions: what an agent can know of tags, tasks and the
// versions of a page, and do with them.
func TestMCPTagsTasksVersions(t *testing.T) {
	st, now := storeAt(t)
	lib, _ := OpenLibraryIn(st, storeRoot)
	seed(lib)
	a := newApp(lib)
	a.clock = func() time.Time { return *now }
	tt := ui.NewTester(a.view, 1000, 700)
	tt.Advance(50 * time.Millisecond)
	cs := agent(t, a)
	c := lib.Collections[0]
	c.Templates = []NamedTemplate{{"Toplantı", "# {{başlık}}\n\n- [ ] karar [[2026-11-04]]\n"}}
	c.save()
	var made struct{ ID string }
	call(t, cs, "create_page", map[string]any{"collection": c.Name, "title": "Haftalık", "template": "toplantı"}, &made)
	call(t, cs, "update_page", map[string]any{"page": made.ID, "append": "Bir #fikir ve bir görev:\n\n- [ ] başka iş"}, nil)
	res, _ := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "create_page", Arguments: map[string]any{"collection": c.Name, "title": "x", "template": "yok"}})
	if !res.IsError {
		t.Fatal("a template that is not there")
	}

	var tags []struct {
		Tag   string
		Pages int
	}
	call(t, cs, "list_tags", nil, &tags)
	if len(tags) != 1 || tags[0].Tag != "fikir" || tags[0].Pages != 1 {
		t.Fatalf("tags: %+v", tags)
	}
	var withTag []struct{ ID, Title string }
	call(t, cs, "list_tags", map[string]any{"tag": "#Fikir"}, &withTag)
	if len(withTag) != 1 || withTag[0].ID != made.ID {
		t.Fatalf("pages of the tag: %+v", withTag)
	}

	type task struct {
		Page, Text, Due string
		Line            int
		Done            bool
	}
	var open []task
	call(t, cs, "list_tasks", nil, &open)
	if len(open) < 2 {
		t.Fatalf("tasks: %+v", open)
	}
	var late []task
	call(t, cs, "list_tasks", map[string]any{"due": "late"}, &late)
	if len(late) != 1 || late[0].Text != "karar" || late[0].Due != "2026-11-04" {
		t.Fatalf("late: %+v", late)
	}
	call(t, cs, "set_task", map[string]any{"page": late[0].Page, "line": late[0].Line, "done": true}, nil)
	var done []task
	call(t, cs, "list_tasks", map[string]any{"state": "done"}, &done)
	found := false
	for _, k := range done {
		found = found || k.Text == "karar"
	}
	if !found {
		t.Fatalf("done: %+v", done)
	}

	// versions: written over after the quiet, then read and put back
	p, _ := a.findPage(made.ID)
	*now = now.Add(10 * time.Minute)
	p.SaveText("# Haftalık\n\nbaşka bir metin\n")
	var vers []struct {
		ID    int64
		Label string
	}
	call(t, cs, "list_versions", map[string]any{"page": made.ID}, &vers)
	if len(vers) == 0 {
		t.Fatal("no versions")
	}
	var rv struct{ Markdown string }
	call(t, cs, "read_version", map[string]any{"page": made.ID, "version": vers[0].ID}, &rv)
	if !strings.Contains(rv.Markdown, "karar") {
		t.Fatalf("version text: %q", rv.Markdown)
	}
	call(t, cs, "restore_version", map[string]any{"page": made.ID, "version": vers[0].ID}, nil)
	if body, _ := p.Text(); !strings.Contains(body, "karar") {
		t.Fatalf("restored: %q", body)
	}
	// search takes the filters
	var hits []struct{ ID string }
	call(t, cs, "search", map[string]any{"query": "#fikir"}, &hits)
	call(t, cs, "search", map[string]any{"query": "başlık:haftalik"}, &hits)
	if len(hits) != 1 {
		t.Fatalf("search: %+v", hits)
	}
}
