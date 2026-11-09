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

// What an agent learns and does besides the pages' tools: help, the app's own description of itself; open_page and
// update_collection; every page it may read as a resource; and a few prompts ready to use.

// mcpHelp is what help answers: what the library is, what a page holds, and how to find pages.
const mcpHelp = `Ryudit, a notes app: what an agent should know.

LIBRARY. A library is collections; a collection is pages; a page can hold sub-pages (when its collection allows).
A page is named by its folder from the library's root ("books/dune/notes"): the id every tool takes. Pages and
collections the user hides from agents are not there at all; read-only ones refuse writes (Settings > MCP).

PAGES. A page has a title, properties (its collection's fields: text, number, date, choice, tags, a link to a
page, a file) and a body of blocks. You read and write the body as Markdown:
  # ## ###  headings          - item, 1. item   lists         - [ ] task, - [x] done   tasks
  - [!] important, - [/] in progress, - [?] question, - [*] star, - [e] error   kinds of tasks
  > quote     ` + "```lang" + ` code     ---  divider     [[Page title]]  a link to a page     [[2026-11-09]]  a day
  ` + "```query" + ` … ` + "```" + `  a live list of the pages a query finds
  <!--ryudit:block …-->  a block Markdown cannot say: leave the line as it is, it comes back as the block
Write what the user means, in their library's own ways (its collections, properties, templates, tags).

FINDING. search and query_pages take the query language: words a page must all have, "a phrase", a OR b,
-word or NOT word, (groups); başlık: (title), koleksiyon: (collection), #tag, @todo / @done (an open or a ticked
task), @today, created: and modified: with a day (2026-11-09), a range (2026-11-01..2026-11-30) or a side
(>2026-11-01), and field:value or field>value on a property, by its name.

TOOLS TO START WITH. list_collections, list_pages, read_page, search; get_daily_note for the journal;
insert_under_heading to add under a heading; update_page to change a page; list_tasks and set_task for tasks.
Every write is kept as a version first and the user can take it back in one click.`

type (
	openArgs struct {
		Page string `json:"page" jsonschema:"the page's id"`
	}
	collUpdateArgs struct {
		Collection string `json:"collection" jsonschema:"the collection's id or name"`
		Name       string `json:"name,omitempty" jsonschema:"its new name; its folder keeps its own"`
		PageNoun   string `json:"pageNoun,omitempty" jsonschema:"what a page of it is called, as in New Recipe"`
		SubPages   *bool  `json:"subPages,omitempty" jsonschema:"whether its pages may hold sub-pages"`
	}
)

func (a *app) addHelpTools(s *mcp.Server, ro *mcp.ToolAnnotations) {
	mcp.AddTool(s, &mcp.Tool{Name: "help", Description: "What the library is, what a page holds, the Markdown of its blocks and the query language: ask it first.", Annotations: ro},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: mcpHelp}}}, nil, nil
		})
	mcp.AddTool(s, &mcp.Tool{Name: "open_page", Description: "Opens a page in the app's window, for the user to see it.", Annotations: ro},
		func(ctx context.Context, _ *mcp.CallToolRequest, in openArgs) (*mcp.CallToolResult, any, error) {
			err := a.do(func() error {
				p, err := a.findPage(in.Page, mcpRead)
				if err != nil {
					return err
				}
				a.open(p)
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			return result(map[string]bool{"ok": true})
		})
	mcp.AddTool(s, &mcp.Tool{Name: "update_collection", Description: "Renames a collection, says what its pages are called, or whether they may hold sub-pages."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in collUpdateArgs) (*mcp.CallToolResult, any, error) {
			err := a.do(func() error {
				c, err := a.findColl(in.Collection, mcpWrite)
				if err != nil {
					return err
				}
				if strings.TrimSpace(in.Name) == "" && in.PageNoun == "" && in.SubPages == nil {
					return errors.New("nothing to change: give name, pageNoun or subPages")
				}
				if n := strings.TrimSpace(in.Name); n != "" {
					c.Name = n
				}
				if in.PageNoun != "" {
					c.PageNoun = strings.TrimSpace(in.PageNoun)
				}
				if in.SubPages != nil {
					c.SubPages = *in.SubPages
				}
				if err := c.Save(); err != nil {
					return err
				}
				PagesChanged()
				return nil
			})
			if err != nil {
				return nil, nil, err
			}
			return result(map[string]bool{"ok": true})
		})

	// every page an agent may read, as Markdown
	s.AddResourceTemplate(&mcp.ResourceTemplate{
		Name: "page", URITemplate: pageURIPrefix + "{+id}", MIMEType: "text/markdown",
		Description: "A page of the library as Markdown, by its id (list_pages names them).",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri := req.Params.URI
		id, ok := strings.CutPrefix(uri, pageURIPrefix)
		if !ok {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		var text string
		err := a.do(func() error {
			p, err := a.findPage(id, mcpRead)
			if err != nil {
				return err
			}
			if p == a.page {
				a.flush()
			}
			md, err := p.Text()
			text = "# " + p.Title + "\n\n" + md
			return err
		})
		if err != nil {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "text/markdown", Text: text}}}, nil
	})

	// prompts ready to use
	s.AddPrompt(&mcp.Prompt{
		Name: "summarize_page", Description: "Summarize a page of the library.",
		Arguments: []*mcp.PromptArgument{{Name: "page", Description: "the page's id", Required: true}},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		id := req.Params.Arguments["page"]
		var title, md string
		err := a.do(func() error {
			p, err := a.findPage(id, mcpRead)
			if err != nil {
				return err
			}
			title = p.Title
			md, err = p.Text()
			return err
		})
		if err != nil {
			return nil, err
		}
		return userPrompt("Summarize the page \""+title+"\"", fmt.Sprintf("Summarize this page of my notes in a few lines, keeping its own words for what matters. Then list what is still to do in it, if anything.\n\n# %s\n\n%s", title, md)), nil
	})
	s.AddPrompt(&mcp.Prompt{Name: "weekly_review", Description: "A review of the last seven days, from the journal."},
		func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			var days []string
			a.do(func() error {
				now := a.clock()
				for i := 6; i >= 0; i-- {
					days = append(days, now.AddDate(0, 0, -i).Format(time.DateOnly))
				}
				return nil
			})
			return userPrompt("A weekly review", "Write a review of my last seven days from my journal in Ryudit. Read each day's page with get_daily_note ("+
				strings.Join(days, ", ")+"; a day without a page is skipped), and the open tasks with list_tasks. Then say: what I did, what I left open, what keeps coming back, and three things to do next week. Do not change any page unless I ask."), nil
		})
}

// pageURIPrefix is the start of a page's resource URI; its id follows.
const pageURIPrefix = "ryudit://page/"

// userPrompt is a prompt of one message from the user.
func userPrompt(desc, text string) *mcp.GetPromptResult {
	return &mcp.GetPromptResult{Description: desc, Messages: []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: text}}}}
}
