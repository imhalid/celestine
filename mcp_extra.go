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

// The tools for what the app knows of the pages beyond their text: tags,
// tasks, and the versions it keeps.

type (
	tagArgs struct {
		Tag string `json:"tag,omitempty" jsonschema:"a tag, with or without the #: the pages that have it; all the tags, with their pages' counts, if not given"`
	}
	tasksArgs struct {
		State      string `json:"state,omitempty" jsonschema:"open (the default), done or all"`
		Due        string `json:"due,omitempty" jsonschema:"of the open ones: today, week (to Sunday), late, or any (the default)"`
		Collection string `json:"collection,omitempty" jsonschema:"only in this collection"`
	}
	setTaskArgs struct {
		Page string `json:"page" jsonschema:"the page's id"`
		Line int    `json:"line" jsonschema:"the task's line in the page's Markdown, from 0, as list_tasks gives it"`
		Done bool   `json:"done" jsonschema:"true to tick it off, false to put it back"`
	}
	versionArgs struct {
		Page    string `json:"page" jsonschema:"the page's id"`
		Version int64  `json:"version" jsonschema:"a version's id, as list_versions gives it"`
	}
)

func (a *app) addExtraTools(s *mcp.Server, ro *mcp.ToolAnnotations) {
	mcp.AddTool(s, &mcp.Tool{Name: "list_tags", Description: "Lists the #tags of the library with how many pages have each; given a tag, the pages that have it.", Annotations: ro},
		func(ctx context.Context, _ *mcp.CallToolRequest, in tagArgs) (*mcp.CallToolResult, any, error) {
			var out any
			err := a.do(func() error {
				a.linksStale()
				a.linksTo(nil, a.clock(), true)
				if strings.TrimSpace(in.Tag) == "" {
					type tag struct {
						Tag   string `json:"tag"`
						Pages int    `json:"pages"`
					}
					var tags []tag
					for _, ti := range a.allTags() {
						n := 0
						for _, p := range ti.pages {
							if a.mcpSees(p.Coll) {
								n++
							}
						}
						if n > 0 {
							tags = append(tags, tag{ti.name, n})
						}
					}
					out = tags
					return nil
				}
				type page struct {
					ID         string `json:"id"`
					Title      string `json:"title"`
					Collection string `json:"collection"`
				}
				var ps []page
				for _, p := range a.tagPages(tagKey(strings.TrimPrefix(strings.TrimSpace(in.Tag), "#"))) {
					if a.mcpSees(p.Coll) {
						ps = append(ps, page{a.idOf(p.Dir), p.Title, p.Coll.Name})
					}
				}
				out = ps
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			return result(out)
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_tasks", Description: "Lists the tasks (\"- [ ]\" lines) of the library: the page, the line, the text, whether it is done, and its day when it names one.", Annotations: ro},
		func(ctx context.Context, _ *mcp.CallToolRequest, in tasksArgs) (*mcp.CallToolResult, any, error) {
			type task struct {
				Page  string `json:"page"`
				Title string `json:"title"`
				Line  int    `json:"line"`
				Text  string `json:"text"`
				Done  bool   `json:"done"`
				Due   string `json:"due,omitempty"`
			}
			var out []task
			err := a.do(func() error {
				var want *Collection
				if in.Collection != "" {
					var err error
					if want, err = a.findColl(in.Collection, mcpRead); err != nil {
						return err
					}
				}
				group := map[string]int{"": 0, "any": 0, "today": 1, "week": 2, "late": 3}
				g, ok := group[strings.ToLower(in.Due)]
				if !ok {
					return fmt.Errorf("due is today, week, late or any, not %q", in.Due)
				}
				state := strings.ToLower(in.State)
				if state != "" && state != "open" && state != "done" && state != "all" {
					return fmt.Errorf("state is open, done or all, not %q", in.State)
				}
				a.linksStale()
				for _, k := range a.allTasks(true) {
					if want != nil && k.page.Coll != want || !a.mcpSees(k.page.Coll) {
						continue
					}
					if (state == "" || state == "open") && k.done || state == "done" && !k.done {
						continue
					}
					if !k.done && !dueMatches(k, g, a.clock()) {
						continue
					}
					t := task{a.idOf(k.page.Dir), k.page.Title, k.line, k.text, k.done, ""}
					if !k.due.IsZero() {
						t.Due = k.due.Format("2006-01-02")
					}
					out = append(out, t)
				}
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			return result(out)
		})

	mcp.AddTool(s, &mcp.Tool{Name: "set_task", Description: "Ticks a task off, or puts it back, by its page and line (list_tasks gives them)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in setTaskArgs) (*mcp.CallToolResult, any, error) {
			err := a.do(func() error {
				p, err := a.findPage(in.Page, mcpWrite)
				if err != nil {
					return err
				}
				a.linksStale()
				for _, k := range a.allTasks(true) {
					if k.page == p && k.line == in.Line {
						if k.done != in.Done {
							a.toggleTask(k)
							a.jiggle(40)
						}
						return nil
					}
				}
				return fmt.Errorf("no task at line %d of %s: list_tasks names them", in.Line, in.Page)
			})
			if err != nil {
				return nil, nil, err
			}
			return result(map[string]bool{"ok": true})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_versions", Description: "Lists the versions the app keeps of a page, the latest first: id, when it was written, its size, and the name the user gave it if any. A library kept as a folder keeps none.", Annotations: ro},
		func(ctx context.Context, _ *mcp.CallToolRequest, in pageArgs) (*mcp.CallToolResult, any, error) {
			type version struct {
				ID    int64     `json:"id"`
				At    time.Time `json:"at"`
				Size  int       `json:"size"`
				Label string    `json:"label,omitempty"`
			}
			var out []version
			err := a.do(func() error {
				p, err := a.findPage(in.Page, mcpRead)
				if err != nil {
					return err
				}
				h, ok := historyOf(p)
				if !ok {
					return errors.New("this library keeps no versions")
				}
				for _, v := range h.Versions(p.File()) {
					out = append(out, version{v.ID, v.At, v.Size, v.Label})
				}
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			return result(out)
		})

	mcp.AddTool(s, &mcp.Tool{Name: "read_version", Description: "Reads what a page's text was in a version, in Markdown.", Annotations: ro},
		func(ctx context.Context, _ *mcp.CallToolRequest, in versionArgs) (*mcp.CallToolResult, any, error) {
			var md string
			err := a.do(func() error {
				p, h, err := a.versionOf(in.Page, mcpRead)
				if err != nil {
					return err
				}
				data, ok := h.VersionData(p.File(), in.Version)
				if !ok {
					return fmt.Errorf("no version %d of %s", in.Version, in.Page)
				}
				_, body := SplitPage(data)
				md = bodyText(body)
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			return result(map[string]string{"markdown": md})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "restore_version", Description: "Puts a page's text back to what it was in a version; the text it has now is kept as a version first."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in versionArgs) (*mcp.CallToolResult, any, error) {
			err := a.do(func() error {
				p, h, err := a.versionOf(in.Page, mcpWrite)
				if err != nil {
					return err
				}
				if _, ok := h.VersionData(p.File(), in.Version); !ok {
					return fmt.Errorf("no version %d of %s", in.Version, in.Page)
				}
				a.flush()
				v := &a.vhist
				*v = histView{page: p, vers: h.Versions(p.File())}
				for i, ver := range v.vers {
					if ver.ID == in.Version {
						v.sel = i
					}
				}
				if err := a.putBackVersion(); err != nil {
					return err
				}
				a.jiggle(400)
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			return result(map[string]bool{"ok": true})
		})
}

// versionOf is the page of id and where its versions are kept.
func (a *app) versionOf(id, need string) (*Page, Historian, error) {
	p, err := a.findPage(id, need)
	if err != nil {
		return nil, nil, err
	}
	h, ok := historyOf(p)
	if !ok {
		return nil, nil, errors.New("this library keeps no versions")
	}
	return p, h, nil
}
