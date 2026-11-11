//go:build !js

package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/imhalid/celestine/locales"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The MCP server itself (mcp.go says what it is): not in the browser's build, which cannot listen on a port
// (mcp_js.go), so the SDK and its schema and JSON packages stay out of the wasm.

// setMCP starts or stops the server as the setting says. It runs in a
// frame, holding the lock: stopping does not wait for tools still running.
func (a *app) setMCP(on bool) {
	a.settings.MCP = on
	a.saveSettings()
	if !on {
		if srv := a.mcpServer; srv != nil {
			a.mcpServer = nil
			go srv.Close()
		}
		a.mcpErr = ""
		return
	}
	if a.mcpServer != nil {
		return
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", a.settings.Port))
	if err != nil {
		a.mcpErr = err.Error()
		a.settings.MCP = false
		return
	}
	a.mcpErr = ""
	a.settings.Port = ln.Addr().(*net.TCPAddr).Port // 0 asks for a free one
	srv := &http.Server{Handler: a.mcpHandler(), ReadHeaderTimeout: 10 * time.Second}
	a.mcpServer = srv
	go srv.Serve(ln)
}

// mcpHandler serves MCP to those who have the token, on a local address
// only (a page in a browser can't reach it under another name).
func (a *app) mcpHandler() http.Handler {
	server := a.newMCPServer()
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.Host)
		if host != "127.0.0.1" && host != "localhost" && host != "::1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var token string
		a.mu.Lock()
		token = a.settings.Token
		a.mu.Unlock()
		if token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// result is v as the tool's text, JSON.
func result(v any) (*mcp.CallToolResult, any, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}}, nil, nil
}

type (
	noArgs   struct{}
	collArgs struct {
		Collection string `json:"collection" jsonschema:"the collection: its id or its name"`
	}
	pageArgs struct {
		Page string `json:"page" jsonschema:"the page's id, as list_pages gives it"`
	}
	searchArgs struct {
		Query      string `json:"query" jsonschema:"words to find in titles and text, any case"`
		Collection string `json:"collection,omitempty" jsonschema:"only in this collection"`
	}
	propSpec struct {
		Name    string   `json:"name" jsonschema:"the property's name"`
		Type    string   `json:"type" jsonschema:"choice, text, number, date (2026-11-04), url, page (another page's folders from the library's root, as kitaplar/dune), tags (words with commas between them) or file (a file's name in the page's folder)"`
		Options []string `json:"options,omitempty" jsonschema:"a choice's options, in order: the columns of the board"`
	}
	newCollArgs struct {
		Name       string     `json:"name" jsonschema:"the collection's name"`
		Properties []propSpec `json:"properties,omitempty" jsonschema:"the fields its pages have; none if not given"`
	}
	addPropArgs struct {
		Collection string `json:"collection" jsonschema:"the collection: its id or its name"`
		propSpec
	}
	newPageArgs struct {
		Collection string            `json:"collection" jsonschema:"the collection: its id or its name"`
		Title      string            `json:"title" jsonschema:"the page's title"`
		Parent     string            `json:"parent,omitempty" jsonschema:"a page to make it a sub-page of"`
		Template   string            `json:"template,omitempty" jsonschema:"the name of one of the collection's templates to start from (list_collections names them); its default one if not given"`
		Markdown   string            `json:"markdown,omitempty" jsonschema:"its text, in Markdown"`
		Properties map[string]string `json:"properties,omitempty" jsonschema:"values of the collection's properties, by name"`
	}
	updateArgs struct {
		Page       string            `json:"page" jsonschema:"the page's id"`
		Title      *string           `json:"title,omitempty" jsonschema:"a new title"`
		Markdown   *string           `json:"markdown,omitempty" jsonschema:"new text in Markdown: replaces the page's text"`
		Append     string            `json:"append,omitempty" jsonschema:"Markdown to add at the end of the page's text"`
		Properties map[string]string `json:"properties,omitempty" jsonschema:"values of the collection's properties, by name; an empty one clears it"`
	}
	moveArgs struct {
		Page       string `json:"page" jsonschema:"the page's id"`
		Collection string `json:"collection,omitempty" jsonschema:"the collection to move it to; its own if not given"`
		Parent     string `json:"parent,omitempty" jsonschema:"a page to put it under; the top of the collection if not given"`
		Index      *int   `json:"index,omitempty" jsonschema:"its place among its new siblings, from 0; the end if not given"`
	}
)

func (a *app) newMCPServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "celestine", Title: "celestine", Version: "0.1.0"}, &mcp.ServerOptions{
		Instructions: "celestine is a notes app: a library of collections of pages, kept in one file (or, opened so, a folder of plain files). Pages are named by ids (their folders, as list_pages gives them) and have properties (list_collections says each collection's). A page's text is read and written as Markdown, which the app keeps as its own blocks: headings, lists, \"- [ ]\" tasks, #tags, [[links]] to pages, tables, code. The user sees what you write in their window at once. A library in one file keeps versions of each page (list_versions), and a page thrown away goes to the trash.",
	})
	ro := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(s, &mcp.Tool{Name: "list_collections", Description: "Lists the library's collections.", Annotations: ro},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
			var out []collInfo
			a.do(func() error {
				for _, c := range a.lib.Collections {
					if !a.mcpSees(c) { // one agents may not see is not listed
						continue
					}
					ci := collInfo{ID: a.idOf(c.Dir), Name: c.Name, Pages: len(c.All()), Properties: c.Properties, SubPages: c.SubPages, Query: c.Query}
					for _, t := range c.AllTemplates() {
						ci.Templates = append(ci.Templates, t.Name)
					}
					out = append(out, ci)
				}
				return nil
			})
			return result(out)
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_pages", Description: "Lists a collection's pages as a tree: id, title, status, sub-pages.", Annotations: ro},
		func(ctx context.Context, _ *mcp.CallToolRequest, in collArgs) (*mcp.CallToolResult, any, error) {
			var out []pageInfo
			err := a.do(func() error {
				c, err := a.findColl(in.Collection, mcpRead)
				if err != nil {
					return err
				}
				out = a.tree(c.Pages)
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			return result(out)
		})

	mcp.AddTool(s, &mcp.Tool{Name: "read_page", Description: "Reads a page: its title, status, collection and text in Markdown.", Annotations: ro},
		func(ctx context.Context, _ *mcp.CallToolRequest, in pageArgs) (*mcp.CallToolResult, any, error) {
			var out map[string]any
			err := a.do(func() error {
				p, err := a.findPage(in.Page, mcpRead)
				if err != nil {
					return err
				}
				if p == a.page {
					a.flush() // what is typed, too
				}
				body, err := p.Text()
				if err != nil {
					return err
				}
				out = map[string]any{"id": a.idOf(p.Dir), "title": p.Title, "properties": values(p),
					"collection": p.Coll.Name, "created": p.Created, "modified": p.Modified, "markdown": body}
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			return result(out)
		})

	mcp.AddTool(s, &mcp.Tool{Name: "search", Description: "Finds pages whose title or text holds every word, Turkish letters taken as their ASCII. Also: \"an exact phrase\", başlık:word (in the title), koleksiyon:name, #tag.", Annotations: ro},
		func(ctx context.Context, _ *mcp.CallToolRequest, in searchArgs) (*mcp.CallToolResult, any, error) {
			type hit struct {
				ID         string `json:"id"`
				Title      string `json:"title"`
				Collection string `json:"collection"`
				Excerpt    string `json:"excerpt"`
			}
			var out []hit
			err := a.do(func() error {
				if strings.TrimSpace(in.Query) == "" {
					return errors.New("no query")
				}
				var want *Collection
				if in.Collection != "" {
					var err error
					if want, err = a.findColl(in.Collection, mcpRead); err != nil {
						return err
					}
				}
				a.linksStale()
				for _, h := range a.searchPages(in.Query, 0, true) {
					if want != nil && h.page.Coll != want || !a.mcpSees(h.page.Coll) {
						continue
					}
					var sb strings.Builder
					for _, sg := range h.snip {
						sb.WriteString(sg.text)
					}
					out = append(out, hit{a.idOf(h.page.Dir), h.page.Title, h.page.Coll.Name, sb.String()})
				}
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			return result(out)
		})

	a.addExtraTools(s, ro)
	a.addMoreTools(s, ro)
	a.addHelpTools(s, ro)
	a.addCommandTools(s, ro)

	mcp.AddTool(s, &mcp.Tool{Name: "create_collection", Description: "Makes a collection."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in newCollArgs) (*mcp.CallToolResult, any, error) {
			var id string
			err := a.do(func() error {
				name := strings.TrimSpace(in.Name)
				if name == "" {
					return errors.New("no name")
				}
				if a.mcpAccess(nil) != mcpWrite { // new collections are what the others are
					return errors.New("agents may not make collections (Settings ▸ MCP)")
				}
				c, err := a.lib.NewCollection(name)
				if err != nil {
					return err
				}
				for _, ps := range in.Properties {
					if _, err := c.AddProperty(ps.Name, ps.Type, ps.Options); err != nil {
						return err
					}
				}
				id = a.idOf(c.Dir)
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			return result(map[string]string{"id": id})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "create_page", Description: "Makes a page in a collection, or under a page, with its text in Markdown."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in newPageArgs) (*mcp.CallToolResult, any, error) {
			var id string
			err := a.do(func() error {
				c, err := a.findColl(in.Collection, mcpWrite)
				if err != nil {
					return err
				}
				var parent *Page
				if in.Parent != "" {
					if parent, err = a.findPage(in.Parent, mcpWrite); err != nil {
						return err
					}
					if parent.Coll != c {
						return errors.New("the parent is in another collection")
					}
				}
				title := strings.TrimSpace(in.Title)
				if title == "" {
					title = locales.CommonUntitled()
				}
				p, err := c.NewPage(parent, title)
				if err != nil {
					return err
				}
				a.agentMade(p)
				if in.Markdown == "" && in.Template != "" {
					found := false
					for _, t := range c.AllTemplates() {
						if strings.EqualFold(t.Name, strings.TrimSpace(in.Template)) {
							in.Markdown, found = expandTemplate(t.Body, title, a.clock()), true
						}
					}
					if !found {
						return fmt.Errorf("no template %q in %s", in.Template, c.Name)
					}
				}
				if in.Markdown == "" && c.Template != "" {
					in.Markdown = expandTemplate(c.Template, title, a.clock()) // as the collection starts its pages
				}
				if in.Markdown != "" {
					if err := p.SaveText(bodyOf(in.Markdown)); err != nil {
						return err
					}
					a.jiggle(len(in.Markdown))
				}
				if err := a.setValues(p, in.Properties); err != nil {
					return err
				}
				id = a.idOf(p.Dir)
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			return result(map[string]string{"id": id})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "add_property", Description: "Adds a property (a field) to a collection's pages: a choice, text, number, date or url. A collection has none until one is added; its first choice gives it a board."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in addPropArgs) (*mcp.CallToolResult, any, error) {
			var pr *Property
			err := a.do(func() error {
				c, err := a.findColl(in.Collection, mcpWrite)
				if err != nil {
					return err
				}
				pr, err = c.AddProperty(in.Name, in.Type, in.Options)
				return err
			})
			if err != nil {
				return nil, nil, err
			}
			return result(pr)
		})

	mcp.AddTool(s, &mcp.Tool{Name: "update_page", Description: "Changes a page: its title, its text (replaced, or added to at the end) or its properties' values."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in updateArgs) (*mcp.CallToolResult, any, error) {
			var id string
			err := a.do(func() error {
				p, err := a.findPage(in.Page, mcpWrite)
				if err != nil {
					return err
				}
				if in.Title != nil {
					t := strings.TrimSpace(*in.Title)
					if t == "" {
						return errors.New("an empty title")
					}
					if p == a.page {
						a.flush()
						a.title = t
					}
					if err := p.SetTitle(t); err != nil {
						return err
					}
				}
				if in.Markdown != nil || in.Append != "" {
					if p == a.page {
						a.flush()
					}
					body, err := p.Text()
					if err != nil {
						return err
					}
					if in.Markdown != nil {
						body = bodyOf(*in.Markdown)
					}
					if in.Append != "" {
						body = strings.TrimRight(body, "\n")
						if body != "" {
							body += "\n\n"
						}
						body += bodyOf(in.Append)
					}
					if err := a.writeBody(p, body); err != nil {
						return err
					}
				}
				if err := a.setValues(p, in.Properties); err != nil {
					return err
				}
				id = a.idOf(p.Dir)
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			return result(map[string]string{"id": id})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "move_page", Description: "Moves a page, its sub-pages with it: under another page, to the top of a collection, or to another place among its siblings."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in moveArgs) (*mcp.CallToolResult, any, error) {
			var id string
			err := a.do(func() error {
				p, err := a.findPage(in.Page, mcpWrite)
				if err != nil {
					return err
				}
				c := p.Coll
				if in.Collection != "" {
					if c, err = a.findColl(in.Collection, mcpWrite); err != nil {
						return err
					}
				}
				var parent *Page
				if in.Parent != "" {
					if parent, err = a.findPage(in.Parent, mcpWrite); err != nil {
						return err
					}
					c = parent.Coll
				}
				siblings := c.Pages
				if parent != nil {
					siblings = parent.Children
				}
				index := len(siblings)
				if in.Index != nil {
					index = *in.Index
					// among the siblings but itself: past it, one further
					if at := indexOf(siblings, p); at >= 0 && at < index {
						index++
					}
				}
				if p == a.page {
					a.flush()
				}
				if err := a.lib.MovePage(p, c, parent, index); err != nil {
					return err
				}
				id = a.idOf(p.Dir)
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			return result(map[string]string{"id": id})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "trash_page", Description: "Moves a page, its sub-pages with it, to the trash; the user can put it back.", Annotations: &mcp.ToolAnnotations{DestructiveHint: ptr(true)}},
		func(ctx context.Context, _ *mcp.CallToolRequest, in pageArgs) (*mcp.CallToolResult, any, error) {
			err := a.do(func() error {
				p, err := a.findPage(in.Page, mcpWrite)
				if err != nil {
					return err
				}
				a.trashPage(p)
				if a.saveErr != "" {
					return errors.New(a.saveErr)
				}
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			return result(map[string]string{"trashed": in.Page})
		})
	return s
}

// ---- through MCP ----

type runCmdArgs struct {
	Command string `json:"command" jsonschema:"the command's title, as list_commands gives it"`
}

// addCommandTools gives an agent the palette's commands: to list, and to run, as the user would from ⌘P.
func (a *app) addCommandTools(s *mcp.Server, ro *mcp.ToolAnnotations) {
	mcp.AddTool(s, &mcp.Tool{Name: "list_commands", Description: "Lists the commands of the app's command palette (⌘P), for what is open now: each title, its key if it has one, and why it cannot run now if it cannot. The settings are commands too.", Annotations: ro},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
			type cmdInfo struct {
				Title string `json:"title"`
				Keys  string `json:"keys,omitempty"`
				Off   string `json:"off,omitempty"`
			}
			var out []cmdInfo
			a.do(func() error {
				for _, it := range a.commands() {
					if mcpMayRun(it) {
						out = append(out, cmdInfo{it.title, it.keys, it.off})
					}
				}
				return nil
			})
			return result(out)
		})
	mcp.AddTool(s, &mcp.Tool{Name: "run_command", Description: "Runs a command of the app's command palette by its title (list_commands gives them), as the user would from ⌘P; the user sees it happen in their window."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in runCmdArgs) (*mcp.CallToolResult, any, error) {
			ran := false
			err := a.do(func() error {
				if it := a.commandByTitle(in.Command); it != nil && it.run != nil && mcpMayRun(*it) { // what it found, not what was named: a near title is another command
					if err := a.mcpMayChange(*it); err != nil {
						return err
					}
					it.run()
					ran = true
				}
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			if !ran {
				return nil, nil, errString("no such command, or it cannot run now: " + in.Command)
			}
			return result(map[string]any{"ran": in.Command})
		})
}

// mcpCommandCats are the groups of commands an agent may run: moving about, the view, the text, a page,
// a collection, the panes. The groups left out change where the library is kept, what it is sent to, how
// the app updates, or ask for a file (Dosyalar ve senkron, Uygulama): they are the user's.
var mcpCommandCats = map[string]bool{"page": true, "collection": true, "panes": true, "common_go": true, "writing": true, "common_appearance": true}

// mcpCommandsAlso are commands of the groups left out that an agent may run too: they only do what their
// title says, and cannot be turned to anything else.
var mcpCommandsAlso = []string{"common_syncNow", "common_backUpNow", "shortcutsAndHelp", "whatsNewInThisVersion", "checkForUpdates"}

// mcpMayRun says whether an agent may run the command it. A page thrown away can be put back from the
// trash; a restore of a backup, the settings put back to their start, a file chosen in a dialog, the place
// sync goes to, are not for an agent.
func mcpMayRun(it palItem) bool {
	if mcpCommandCats[it.cat] {
		return true
	}
	for _, t := range mcpCommandsAlso {
		if it.id == t {
			return true
		}
	}
	return false
}

// mcpReadOnlyCommands are commands of a page that change nothing, by their Turkish title.
var mcpReadOnlyCommands = map[string]bool{"findInPage2": true}

// mcpMayChange says why an agent may not run command it, or nil: a command of a page or a collection changes
// the page or collection the window shows (a new page goes into it), and so needs what writing it needs
// (Settings ▸ MCP), as the tools do; a journal's commands, the journal's.
func (a *app) mcpMayChange(it palItem) error {
	name := it.name
	if name == "" {
		name = it.fam
	}
	if it.cat != "page" && it.cat != "collection" || mcpReadOnlyCommands[name] {
		return nil
	}
	if name == "newCollection" {
		if a.mcpAccess(nil) != mcpWrite {
			return errors.New("agents may not make collections (Settings ▸ MCP)")
		}
		return nil
	}
	var c *Collection
	switch {
	case name == "todaysPage" || name == "Yeni günlük":
		for _, x := range a.lib.Collections {
			if x.Journal {
				c = x
			}
		}
	case a.page != nil:
		c = a.page.Coll
	default:
		c = a.coll
	}
	if c == nil {
		return nil
	}
	return a.mcpMay(c, mcpWrite)
}
