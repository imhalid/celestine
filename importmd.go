package main

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// Importing a folder of Markdown - an Obsidian vault, a notes folder: its
// subfolders become collections (the files at its top, one named after it),
// each file a page. A page's title is its front matter's, else its first "# "
// line, else the file's name; its front matter's tags become #tags at the end
// of its text; ![[picture.png]] and ![](picture.png) bring the picture along,
// into the page's folder; [[links]] stay as they are. The folder is only
// read.

var (
	embedWiki = regexp.MustCompile(`!\[\[([^\]|]+?)(?:\|[^\]]*)?\]\]`)
	embedMD   = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	h1Line    = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
)

// importStats is what an import made.
type importStats struct {
	Collections, Pages, Pictures int
}

func (s importStats) String() string {
	return fmt.Sprintf("%d koleksiyon, %d sayfa, %d resim içe alındı", s.Collections, s.Pages, s.Pictures)
}

// mdFiles are the .md files under dir, hidden folders left out.
func mdFiles(dir string) []string {
	var out []string
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != dir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".md") && !strings.HasPrefix(d.Name(), ".") {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// importMarkdown brings the Markdown under root into the library.
func (a *app) importMarkdown(root string) (importStats, error) {
	var st importStats
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return st, fmt.Errorf("klasör değil: %s", root)
	}
	groups := map[string][]string{} // a collection's name, to its files
	var order []string
	add := func(name, file string) {
		if _, ok := groups[name]; !ok {
			order = append(order, name)
		}
		groups[name] = append(groups[name], file)
	}
	rootName := filepath.Base(root)
	for _, f := range mdFiles(root) {
		rel, _ := filepath.Rel(root, f)
		if top, _, nested := strings.Cut(filepath.ToSlash(rel), "/"); nested {
			add(top, f)
		} else {
			add(rootName, f)
		}
	}
	if len(order) == 0 {
		return st, fmt.Errorf("klasörde .md dosyası yok: %s", root)
	}
	for _, name := range order {
		c, err := a.lib.NewCollection(name)
		if err != nil {
			return st, err
		}
		st.Collections++
		for _, f := range groups[name] {
			if _, err := a.importFile(c, root, f, &st); err != nil {
				return st, err
			}
		}
	}
	a.linksStale()
	return st, nil
}

// importFile makes a page of the file f in c.
func (a *app) importFile(c *Collection, root, f string, st *importStats) (*Page, error) {
	data, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}
	meta, body := splitFrontMatter(string(data))
	title := strings.TrimSpace(meta["title"])
	if title == "" {
		if m := h1Line.FindStringSubmatch(body); m != nil {
			title = strings.TrimSpace(m[1])
		}
	}
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
	}
	p, err := c.NewPage(nil, title)
	if err != nil {
		return nil, err
	}
	body = a.importPictures(p, root, filepath.Dir(f), body, st)
	if tags := frontTags(meta); len(tags) > 0 {
		body = strings.TrimRight(body, "\n") + "\n\n" + strings.Join(tags, " ") + "\n"
	}
	if t := firstTime(meta["created"], meta["date"]); !t.IsZero() {
		p.Created = t
	} else if fi, err := os.Stat(f); err == nil {
		p.Created = fi.ModTime().Truncate(time.Second)
	}
	body, _ = splitTitle(body, title) // the title is the page's own
	if err := p.SaveText(body); err != nil {
		return nil, err
	}
	st.Pages++
	return p, nil
}

// frontTags are the tags of a page's front matter, written as #tags.
func frontTags(meta map[string]string) []string {
	var out []string
	seen := map[string]bool{}
	for _, key := range []string{"tags", "tag", "etiketler"} {
		for _, w := range strings.FieldsFunc(meta[key], func(r rune) bool { return r == ',' || r == ' ' || r == '\n' }) {
			w = strings.TrimPrefix(strings.TrimSpace(w), "#")
			w = strings.ReplaceAll(w, " ", "-")
			if w != "" && tagWord.MatchString(" #"+w) && !seen[tagKey(w)] {
				seen[tagKey(w)] = true
				out = append(out, "#"+w)
			}
		}
	}
	return out
}

// firstTime is the first of the days written that reads.
func firstTime(vals ...string) time.Time {
	for _, v := range vals {
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02"} {
			if t, err := time.Parse(layout, strings.TrimSpace(v)); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

// importPictures copies the pictures body names into page p's folder and
// makes the lines name them there: ![[a.png]] and ![](a.png) are looked for
// beside the file and anywhere under root.
func (a *app) importPictures(p *Page, root, dir, body string, st *importStats) string {
	copied := map[string]string{} // the source, to the name in the folder
	bring := func(ref string) string {
		if ref == "" || strings.Contains(ref, "://") || strings.HasPrefix(ref, "data:") {
			return ""
		}
		if u, err := url.PathUnescape(ref); err == nil {
			ref = u
		}
		if name, ok := copied[ref]; ok {
			return name
		}
		src := ""
		for _, cand := range []string{filepath.Join(dir, ref), filepath.Join(root, ref)} {
			if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
				src = cand
				break
			}
		}
		if src == "" { // an Obsidian embed names the file only: it is anywhere in the vault
			base := filepath.Base(ref)
			filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err == nil && src == "" && !d.IsDir() && d.Name() == base {
					src = path
				}
				return nil
			})
		}
		if src == "" {
			return ""
		}
		data, err := os.ReadFile(src)
		if err != nil || imageExt(data) == "" {
			return ""
		}
		name := fmt.Sprintf("resim-%s-%d%s", a.clock().Format("20060102-150405"), len(copied)+1, imageExt(data))
		if err := p.fs().WriteFile(filepath.Join(p.Dir, name), data); err != nil {
			return ""
		}
		copied[ref] = name
		st.Pictures++
		return name
	}
	body = embedWiki.ReplaceAllStringFunc(body, func(m string) string {
		ref := embedWiki.FindStringSubmatch(m)[1]
		if name := bring(strings.TrimSpace(ref)); name != "" {
			return "![" + strings.TrimSuffix(filepath.Base(ref), filepath.Ext(ref)) + "](" + name + ")"
		}
		return "*(resim yok: " + ref + ")*"
	})
	body = embedMD.ReplaceAllStringFunc(body, func(m string) string {
		sm := embedMD.FindStringSubmatch(m)
		if name := bring(sm[2]); name != "" {
			return "![" + sm[1] + "](" + name + ")"
		}
		return m
	})
	return body
}

// pickImportFolder asks for a folder in a dialog and imports it.
func (a *app) pickImportFolder() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Title: "İçe aktarılacak Markdown klasörü", Directory: true})
		if err != nil || len(paths) == 0 {
			return
		}
		a.do(func() error {
			a.flush()
			st, err := a.importMarkdown(paths[0])
			if err != nil {
				a.notice = "Olmadı: " + err.Error()
				return nil
			}
			a.notice = st.String()
			a.importNote = st.String()
			return nil
		})
	}()
}

// settingsImport is the settings' row to import a folder.
func (a *app) settingsImport(c *ui.Context) {
	note := a.importNote
	if note == "" {
		note = "Obsidian kasası ya da .md dosyaları olan bir klasör: alt klasörleri koleksiyon, dosyaları sayfa olur. Klasöre dokunulmaz."
	}
	settingRow(c, "Markdown klasörü içe aktar", note, func() {
		if ryugo.Button(c.Key("import-md"), "Klasör seç…", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
			a.pickImportFolder()
		}
	})
}
