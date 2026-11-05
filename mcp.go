package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The MCP server: turned on in the settings, the app serves its library to
// agents (Claude Code, or any MCP client) at http://127.0.0.1:<port>/mcp,
// with a token they send as "Authorization: Bearer <token>". An agent
// lists, reads, searches, makes, edits, moves and throws away collections
// and pages; what it does shows in the window at once, as if typed there.
//
// The library is the window's: a tool runs holding the app's lock, which a
// frame holds too, and asks for a frame after it.

const defaultPort = 7457

// Settings are the app's own, kept in the user's config folder.
type Settings struct {
	MCP   bool   `json:"mcp"`
	Port  int    `json:"port"`
	Token string `json:"token"`
}

// loadSettings reads the settings, making a token the first time.
func (a *app) loadSettings() {
	if a.settingsFile != "" {
		if data, err := os.ReadFile(a.settingsFile); err == nil {
			json.Unmarshal(data, &a.settings)
		}
	}
	if a.settings.Port == 0 {
		a.settings.Port = defaultPort
	}
	if a.settings.Token == "" {
		a.settings.Token = newToken()
		a.saveSettings()
	}
}

func (a *app) saveSettings() {
	if a.settingsFile == "" {
		return
	}
	data, _ := json.MarshalIndent(a.settings, "", "  ")
	os.MkdirAll(filepath.Dir(a.settingsFile), 0o755)
	if err := writeAtomic(a.settingsFile, data); err != nil {
		a.saveErr = err.Error()
	}
}

func newToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// mcpURL is where agents reach the server.
func (a *app) mcpURL() string { return fmt.Sprintf("http://127.0.0.1:%d/mcp", a.settings.Port) }

// mcpCommand is the line that adds the server to Claude Code.
func (a *app) mcpCommand() string {
	return fmt.Sprintf("claude mcp add --transport http ryudit %s --header \"Authorization: Bearer %s\"", a.mcpURL(), a.settings.Token)
}

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
		if r.Header.Get("Authorization") != "Bearer "+a.settings.Token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// do runs fn on the library, holding the app's lock, and asks for a frame.
func (a *app) do(fn func() error) error {
	a.mu.Lock()
	err := fn()
	a.mu.Unlock()
	if a.invalidate != nil {
		a.invalidate()
	}
	return err
}

// ---- what an agent sees ----

// A page or collection is named by its folder, from the library's root, as
// "kitaplar/dune/notlar".

type pageInfo struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	Properties map[string]string `json:"properties,omitempty"` // by property name
	Children   []pageInfo        `json:"children,omitempty"`
}

type collInfo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Pages      int        `json:"pages"`
	Properties []Property `json:"properties"`
	SubPages   bool       `json:"subPages"`
}

func (a *app) idOf(dir string) string {
	rel, _ := filepath.Rel(a.lib.Root, dir)
	return filepath.ToSlash(rel)
}

func (a *app) findColl(id string) (*Collection, error) {
	id = strings.Trim(id, "/")
	for _, c := range a.lib.Collections {
		if a.idOf(c.Dir) == id || strings.EqualFold(c.Name, id) {
			return c, nil
		}
	}
	return nil, fmt.Errorf("no collection %q: list_collections names them", id)
}

func (a *app) findPage(id string) (*Page, error) {
	id = strings.Trim(id, "/")
	for _, c := range a.lib.Collections {
		for _, p := range c.All() {
			if a.idOf(p.Dir) == id {
				return p, nil
			}
		}
	}
	return nil, fmt.Errorf("no page %q: list_pages names them", id)
}

func (a *app) tree(ps []*Page) []pageInfo {
	var out []pageInfo
	for _, p := range ps {
		out = append(out, pageInfo{ID: a.idOf(p.Dir), Title: p.Title, Properties: values(p), Children: a.tree(p.Children)})
	}
	return out
}

// values is page p's values, by property name.
func values(p *Page) map[string]string {
	out := map[string]string{}
	for i := range p.Coll.Properties {
		pr := &p.Coll.Properties[i]
		if v := p.Value(pr); v != "" {
			out[pr.Name] = v
		}
	}
	return out
}

// setValues gives page p the values vs, by property name or id.
func (a *app) setValues(p *Page, vs map[string]string) error {
	if p == a.page {
		a.flush()
	}
	for name, v := range vs {
		pr := p.Coll.PropNamed(name)
		if pr == nil {
			var names []string
			for _, x := range p.Coll.Properties {
				names = append(names, x.Name)
			}
			return fmt.Errorf("%s has no property %q (it has: %s; add_property adds one)", p.Coll.Name, name, strings.Join(names, ", "))
		}
		if err := p.SetValue(pr.ID, v); err != nil {
			return err
		}
	}
	return nil
}

// result is v as the tool's text, JSON.
func result(v any) (*mcp.CallToolResult, any, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}}, nil, nil
}

// bodyOf is what an agent gives as a page's text: Markdown, without front matter.
func bodyOf(md string) string {
	if _, body := splitFrontMatter(md); strings.HasPrefix(md, "---\n") {
		md = body
	}
	md = strings.TrimLeft(md, "\n")
	if md != "" && !strings.HasSuffix(md, "\n") {
		md += "\n"
	}
	return md
}

// writeBody gives page p the text body, on disk and in its document if it
// is open: the window shows it at once. What was typed in it is saved first.
func (a *app) writeBody(p *Page, body string) error {
	if p == a.page {
		a.flush()
	}
	if err := p.Save(body); err != nil {
		return err
	}
	if doc := a.docs[p]; doc != nil {
		doc.SetText(a.parse(body))
		if p == a.page {
			_, caret := doc.Selection()
			a.version, a.length, a.caretAt = doc.Version(), doc.Text().Len(), caret
			a.lastSave, a.dirty = bodyMarkdown(doc.Text()), false
		}
	}
	return nil
}

// ---- the tools ----

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
		Type    string   `json:"type" jsonschema:"choice, text, number, date (2026-11-04) or url"`
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
	s := mcp.NewServer(&mcp.Implementation{Name: "ryudit", Title: "ryudit", Version: "0.1.0"}, &mcp.ServerOptions{
		Instructions: "ryudit is a Markdown editor of collections of pages, kept as plain files. Pages are named by ids (their folders, as list_pages gives them). Text is Markdown; the user sees what you write in their window at once.",
	})
	ro := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(s, &mcp.Tool{Name: "list_collections", Description: "Lists the library's collections.", Annotations: ro},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
			var out []collInfo
			a.do(func() error {
				for _, c := range a.lib.Collections {
					out = append(out, collInfo{ID: a.idOf(c.Dir), Name: c.Name, Pages: len(c.All()), Properties: c.Properties, SubPages: c.SubPages})
				}
				return nil
			})
			return result(out)
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_pages", Description: "Lists a collection's pages as a tree: id, title, status, sub-pages.", Annotations: ro},
		func(ctx context.Context, _ *mcp.CallToolRequest, in collArgs) (*mcp.CallToolResult, any, error) {
			var out []pageInfo
			err := a.do(func() error {
				c, err := a.findColl(in.Collection)
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
				p, err := a.findPage(in.Page)
				if err != nil {
					return err
				}
				if p == a.page {
					a.flush() // what is typed, too
				}
				body, err := p.Body()
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

	mcp.AddTool(s, &mcp.Tool{Name: "search", Description: "Finds pages whose title or text holds the words.", Annotations: ro},
		func(ctx context.Context, _ *mcp.CallToolRequest, in searchArgs) (*mcp.CallToolResult, any, error) {
			type hit struct {
				ID         string `json:"id"`
				Title      string `json:"title"`
				Collection string `json:"collection"`
				Excerpt    string `json:"excerpt"`
			}
			var out []hit
			err := a.do(func() error {
				q := strings.ToLower(strings.TrimSpace(in.Query))
				if q == "" {
					return errors.New("no query")
				}
				for _, c := range a.lib.Collections {
					if in.Collection != "" {
						if want, err := a.findColl(in.Collection); err != nil {
							return err
						} else if want != c {
							continue
						}
					}
					for _, p := range c.All() {
						body, _ := p.Body()
						if strings.Contains(strings.ToLower(p.Title), q) || strings.Contains(strings.ToLower(body), q) {
							out = append(out, hit{a.idOf(p.Dir), p.Title, c.Name, p.Excerpt()})
						}
					}
				}
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			return result(out)
		})

	mcp.AddTool(s, &mcp.Tool{Name: "create_collection", Description: "Makes a collection."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in newCollArgs) (*mcp.CallToolResult, any, error) {
			var id string
			err := a.do(func() error {
				name := strings.TrimSpace(in.Name)
				if name == "" {
					return errors.New("no name")
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
				c, err := a.findColl(in.Collection)
				if err != nil {
					return err
				}
				var parent *Page
				if in.Parent != "" {
					if parent, err = a.findPage(in.Parent); err != nil {
						return err
					}
					if parent.Coll != c {
						return errors.New("the parent is in another collection")
					}
				}
				title := strings.TrimSpace(in.Title)
				if title == "" {
					title = "Adsız"
				}
				p, err := c.NewPage(parent, title)
				if err != nil {
					return err
				}
				if in.Markdown == "" && c.Template != "" {
					in.Markdown = c.Template // as the collection starts its pages
				}
				if in.Markdown != "" {
					if err := p.Save(bodyOf(in.Markdown)); err != nil {
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

	mcp.AddTool(s, &mcp.Tool{Name: "add_property", Description: "Adds a property (a field) to a collection's pages: a choice, text, number, date or url. A collection has none until one is added; its first choice gives it a board."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in addPropArgs) (*mcp.CallToolResult, any, error) {
			var pr *Property
			err := a.do(func() error {
				c, err := a.findColl(in.Collection)
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
				p, err := a.findPage(in.Page)
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
					body, err := p.Body()
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
				p, err := a.findPage(in.Page)
				if err != nil {
					return err
				}
				c := p.Coll
				if in.Collection != "" {
					if c, err = a.findColl(in.Collection); err != nil {
						return err
					}
				}
				var parent *Page
				if in.Parent != "" {
					if parent, err = a.findPage(in.Parent); err != nil {
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
				p, err := a.findPage(in.Page)
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

func indexOf(ps []*Page, p *Page) int {
	for i, q := range ps {
		if q == p {
			return i
		}
	}
	return -1
}

func ptr[T any](v T) *T { return &v }
