//go:build !js

package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// More tools for agents: text put under a heading of a page, pages found by their properties' values, the
// journal's page of a day, and the trash looked into and a page brought back from it.

type (
	headingArgs struct {
		Page     string `json:"page" jsonschema:"the page's id"`
		Heading  string `json:"heading" jsonschema:"the heading's text, without its #s, any case"`
		Markdown string `json:"markdown" jsonschema:"what to put at the end of the heading's section, before the next heading of its level or above"`
	}
	queryCond struct {
		Property string `json:"property" jsonschema:"a property's name, or title"`
		Op       string `json:"op" jsonschema:"is, isNot, contains, empty, notEmpty, lt or gt (less, before; more, after)"`
		Value    string `json:"value,omitempty"`
	}
	queryArgs struct {
		Collection string      `json:"collection,omitempty" jsonschema:"only in this collection"`
		Where      []queryCond `json:"where" jsonschema:"the conditions a page meets, all of them"`
	}
	dayArgs struct {
		Date string `json:"date,omitempty" jsonschema:"the day as 2026-11-04; today when left out"`
	}
	restoreArgs struct {
		Item string `json:"item" jsonschema:"the trash item's id, as list_trash gives it"`
	}
	trashInfo struct {
		ID      string    `json:"id"`
		Kind    string    `json:"kind"`
		Title   string    `json:"title"`
		Where   string    `json:"where,omitempty"`
		Trashed time.Time `json:"trashed"`
	}
)

func (a *app) addMoreTools(s *mcp.Server, ro *mcp.ToolAnnotations) {
	mcp.AddTool(s, &mcp.Tool{Name: "insert_under_heading", Description: "Puts Markdown at the end of a heading's section of a page, leaving the rest of the page as it is."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in headingArgs) (*mcp.CallToolResult, any, error) {
			err := a.do(func() error {
				p, err := a.findPage(in.Page, mcpWrite)
				if err != nil {
					return err
				}
				if p == a.page {
					a.flush()
				}
				body, err := p.Text()
				if err != nil {
					return err
				}
				out, err := underHeading(body, in.Heading, bodyOf(in.Markdown))
				if err != nil {
					return err
				}
				return a.writeBody(p, out)
			})
			if err != nil {
				return nil, nil, err
			}
			return result(map[string]bool{"ok": true})
		})
	mcp.AddTool(s, &mcp.Tool{Name: "query_pages", Description: "Lists the pages whose properties meet every condition given, as a view's filters do.", Annotations: ro},
		func(ctx context.Context, _ *mcp.CallToolRequest, in queryArgs) (*mcp.CallToolResult, any, error) {
			type found struct {
				ID         string            `json:"id"`
				Title      string            `json:"title"`
				Collection string            `json:"collection"`
				Properties map[string]string `json:"properties,omitempty"`
			}
			var out []found
			err := a.do(func() error {
				colls := a.lib.Collections
				if in.Collection != "" {
					c, err := a.findColl(in.Collection, mcpRead)
					if err != nil {
						return err
					}
					colls = []*Collection{c}
				}
				for _, c := range colls {
					if !a.mcpSees(c) {
						continue
					}
					v, ok := viewOf(c, in.Where)
					if !ok { // a property this collection does not have: none of its pages meet it
						continue
					}
					for _, p := range c.All() {
						if v.Match(c, p) {
							out = append(out, found{a.idOf(p.Dir), p.Title, c.Name, values(p)})
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
	mcp.AddTool(s, &mcp.Tool{Name: "get_daily_note", Description: "Reads the journal's page of a day, made first when the day is today and it has none."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in dayArgs) (*mcp.CallToolResult, any, error) {
			var out map[string]string
			err := a.do(func() error {
				c, err := a.journal()
				if err != nil {
					return err
				}
				day := a.clock()
				if in.Date != "" {
					if day, err = time.ParseInLocation("2006-01-02", strings.TrimSpace(in.Date), time.Local); err != nil {
						return fmt.Errorf("a day is written 2026-11-04, not %q", in.Date)
					}
				}
				p := a.journalPage(c, day)
				if p == nil {
					if !sameDay(day, a.clock()) {
						return fmt.Errorf("the journal has no page for %s", day.Format("2006-01-02"))
					}
					if err := a.mcpMay(c, mcpWrite); err != nil {
						return err
					}
					if p, err = a.makeDay(c, day); err != nil {
						return err
					}
				} else if err := a.mcpMay(c, mcpRead); err != nil {
					return err
				}
				text, err := p.Text()
				if err != nil {
					return err
				}
				out = map[string]string{"id": a.idOf(p.Dir), "title": p.Title, "markdown": text}
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			return result(out)
		})
	mcp.AddTool(s, &mcp.Tool{Name: "list_trash", Description: "Lists what is in the trash: pages and collections thrown away, newest first.", Annotations: ro},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
			var out []trashInfo
			a.do(func() error {
				for _, it := range a.lib.Trash() {
					if !a.mcpSeesTrash(it) {
						continue
					}
					out = append(out, trashInfo{trashID(it), it.Kind, it.Title, it.Where, it.Trashed})
				}
				return nil
			})
			return result(out)
		})
	mcp.AddTool(s, &mcp.Tool{Name: "restore_page", Description: "Brings a page or collection back from the trash to where it was."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in restoreArgs) (*mcp.CallToolResult, any, error) {
			var id string
			err := a.do(func() error {
				for _, it := range a.lib.Trash() {
					if trashID(it) != in.Item || !a.mcpSeesTrash(it) {
						continue
					}
					if it.Kind == "collection" && a.mcpAccess(nil) != mcpWrite || it.Kind != "collection" && a.mcpMay(a.collOfTrash(it), mcpWrite) != nil {
						return errors.New("agents may not bring this back (Settings ▸ MCP)")
					}
					c, p, err := a.lib.Restore(it)
					if err != nil {
						return err
					}
					a.loadTrash()
					a.linksStale()
					switch {
					case p != nil:
						id = a.idOf(p.Dir)
					case c != nil:
						id = a.idOf(c.Dir)
					}
					return nil
				}
				return fmt.Errorf("no %q in the trash: list_trash names what is there", in.Item)
			})
			if err != nil {
				return nil, nil, err
			}
			return result(map[string]string{"id": id})
		})
}

// trashID names a trash item for an agent: where it was, and when it was thrown away.
func trashID(it *TrashItem) string { return it.From + "@" + it.Trashed.UTC().Format(time.RFC3339Nano) }

// collOfTrash is the collection a page in the trash was in: the first folder of where it was.
func (a *app) collOfTrash(it *TrashItem) *Collection {
	first := strings.SplitN(strings.Trim(it.From, "/"), "/", 2)[0]
	for _, c := range a.lib.Collections {
		if a.idOf(c.Dir) == first {
			return c
		}
	}
	return nil
}

// mcpSeesTrash is whether an agent may see a trash item: a page of a collection it may see, or a collection when it
// may see the others.
func (a *app) mcpSeesTrash(it *TrashItem) bool {
	if it.Kind == "collection" {
		return a.mcpAccess(nil) != mcpOff
	}
	c := a.collOfTrash(it)
	return c == nil && a.mcpAccess(nil) != mcpOff || c != nil && a.mcpSees(c)
}

// viewOf is a view with the conditions as its filters, the properties named as collection c has them; ok is false
// when c has no property of a name.
func viewOf(c *Collection, where []queryCond) (*View, bool) {
	v := &View{}
	for _, w := range where {
		f := Filter{Prop: "title", Op: strings.TrimSpace(w.Op), Value: w.Value}
		if name := strings.TrimSpace(w.Property); !strings.EqualFold(name, "title") {
			pr := c.PropNamed(name)
			if pr == nil {
				return nil, false
			}
			f.Prop = pr.ID
		}
		v.Filters = append(v.Filters, f)
	}
	return v, true
}

// underHeading is body with md put at the end of the section of the heading whose text is heading (any case): before
// the next heading of its level or above, or at the end.
func underHeading(body, heading, md string) (string, error) {
	want := strings.ToLower(strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(heading), "#")))
	lines := strings.Split(body, "\n")
	level, at := 0, -1
	inCode := false
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			inCode = !inCode
		}
		if inCode {
			continue
		}
		n := headingLevel(l)
		if n == 0 {
			continue
		}
		if level == 0 && strings.ToLower(strings.TrimSpace(l[n:])) == want {
			level = n
			continue
		}
		if level > 0 && n <= level {
			at = i
			break
		}
	}
	if level == 0 {
		return "", fmt.Errorf("no heading %q in the page", heading)
	}
	if at < 0 {
		at = len(lines)
	}
	// the section's blank lines at its end stay after what is put in
	end := at
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	ins := []string{"", strings.TrimRight(md, "\n")}
	if at < len(lines) {
		ins = append(ins, "")
	}
	out := append(append(append([]string{}, lines[:end]...), ins...), lines[at:]...)
	return strings.Join(out, "\n"), nil
}

// headingLevel is how many #s start line as a Markdown heading, or 0.
func headingLevel(line string) int {
	n := 0
	for n < len(line) && n < 6 && line[n] == '#' {
		n++
	}
	if n == 0 || n >= len(line) || line[n] != ' ' {
		return 0
	}
	return n
}
