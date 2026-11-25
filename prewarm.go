package main

import (
	"encoding/json"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/imhalid/celestine/internal/library"
)

// As the app opens, every page's text is read once, away from the window: its links, tags, tasks and
// words (the links' index), and its plain text for the search. Read on the window's thread, a few
// milliseconds a frame, a library of thousands of pages took seconds of CPU (its text made from the
// page's form, the marks taken out of it) and slowed every frame until it was done. Here the pages
// come out of the library file in one pass, are read on the cores the window does not need, and are
// handed to the indexes in batches; the indexes, built from what was read, are made in a moment.

// prewarmBatch is how many pages are read before they are handed to the window.
const prewarmBatch = 300

type prewarmJob struct {
	p      *Page
	fs     FS
	store  *Store
	name   string
	mod    time.Time
	writes int
	mtime  int64
	size   int64
	cached []byte // cached derived index JSON read from Store
	src    []byte // raw page file read with others (Store), nil to read individually
}

// prewarmChunk is how many page files of a library's file are read at once: the reading holds that many, not all.
// A test makes it smaller.
var prewarmChunk = 2000

type prewarmRead struct {
	j      prewarmJob
	idx    *pageIdx
	plain  *plainIdx
	save   *library.PageIndex
	cached bool
}

// prewarmWait is how long the reading waits for the window's first frame, which it would otherwise take the cores
// from (#581): a window that never draws one still gets its indexes.
const prewarmWait = 2 * time.Second

// prewarm starts the reading, once the window's first frame is drawn. While it goes on, the indexes do not read pages
// on the window's thread.
func (a *app) prewarm() {
	var jobs []prewarmJob
	for _, c := range a.lib.Collections {
		for _, p := range c.All() {
			jobs = append(jobs, prewarmJob{p: p, fs: p.FS(), name: p.File(), mod: p.Modified, writes: p.Writes})
		}
	}
	if len(jobs) == 0 {
		return
	}
	a.prewarming = true
	first := a.firstFrame
	go func() {
		defer a.guard()
		hits, misses := 0, 0
		defer func() {
			h, m := hits, misses
			a.do(func() error {
				a.links.warmHits = h
				a.links.warmMisses = m
				a.prewarming = false
				return nil
			})
		}()
		select {
		case <-first:
		case <-time.After(prewarmWait):
		}
		in := make(chan prewarmJob)
		out := make(chan prewarmRead, 64)
		var wg sync.WaitGroup
		for range max(1, runtime.NumCPU()/2) { // half the cores: the window and its GPU have the others
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer a.guard()
				for j := range in {
					if len(j.cached) > 0 {
						var pi library.PageIndex
						if json.Unmarshal(j.cached, &pi) == nil {
							out <- prewarmRead{
								j:      j,
								idx:    idxFromPersisted(j.p, j.mod, j.writes, &pi),
								plain:  &plainIdx{mod: j.mod, writes: j.writes, folded: pi.Folded},
								cached: true,
							}
							continue
						}
					}
					src := j.src
					if src == nil {
						var err error
						if src, err = j.fs.ReadFile(j.name); err != nil {
							continue
						}
					}
					_, body := SplitPage(src)
					e, plain := readPageIdxAt(j.p, j.mod, j.writes, TextOfBody(body))
					folded := foldText(plain)
					var save *library.PageIndex
					if j.store != nil {
						save = persistablePageIndex(j.name, j.mtime, j.size, e, folded)
					}
					out <- prewarmRead{j: j, idx: e, plain: &plainIdx{mod: j.mod, writes: j.writes, folded: folded}, save: save}
				}
			}()
		}
		go func() {
			defer a.guard()
			feedPrewarm(jobs, in)
			close(in)
			wg.Wait()
			close(out)
		}()
		var got []prewarmRead
		var toSave map[*Store][]library.PageIndex
		for r := range out {
			if r.cached {
				hits++
			} else {
				misses++
			}
			if r.save != nil && r.j.store != nil {
				if toSave == nil {
					toSave = map[*Store][]library.PageIndex{}
				}
				toSave[r.j.store] = append(toSave[r.j.store], *r.save)
			}
			if got = append(got, r); len(got) >= prewarmBatch {
				a.takePrewarm(got)
				got = nil
			}
		}
		a.takePrewarm(got)
		for st, entries := range toSave {
			_ = st.WritePageIndexes(entries)
		}
	}()
}

func persistablePageIndex(path string, mtime, size int64, e *pageIdx, folded string) *library.PageIndex {
	var tasks []library.PageIndexTask
	if len(e.tasks) > 0 {
		tasks = make([]library.PageIndexTask, len(e.tasks))
		for i, t := range e.tasks {
			var due int64
			if !t.due.IsZero() {
				due = t.due.UnixNano()
			}
			tasks[i] = library.PageIndexTask{
				Line: t.line, Done: t.done, Text: t.text, Due: due,
				Every: t.every, Kind: t.kind, Inl: t.inl, Col: t.col, Folded: t.folded,
			}
		}
	}
	return &library.PageIndex{
		Path: path, MTime: mtime, Size: size,
		Tasks: tasks, Tags: e.tags, Links: e.links,
		Words: e.words, Empty: e.empty, Folded: folded,
	}
}

func idxFromPersisted(p *Page, mod time.Time, writes int, pi *library.PageIndex) *pageIdx {
	var tasks []taskItem
	if len(pi.Tasks) > 0 {
		tasks = make([]taskItem, len(pi.Tasks))
		for i, t := range pi.Tasks {
			var due time.Time
			if t.Due != 0 {
				due = time.Unix(0, t.Due).UTC()
			}
			tasks[i] = taskItem{
				page: p, line: t.Line, done: t.Done, text: t.Text, due: due,
				every: t.Every, kind: t.Kind, inl: t.Inl, col: t.Col, folded: t.Folded,
			}
		}
	}
	return &pageIdx{
		mod: mod, writes: writes, tasks: tasks, tags: pi.Tags,
		links: pi.Links, words: pi.Words, empty: pi.Empty,
	}
}

// feedPrewarm gives the workers the jobs, the pages of a library's file with their files read in parts of
// prewarmChunk, in the order of their paths, a query a part: the files of a part are let go once its pages are
// handed on, where all of them were read and held at once (#196). Up-to-date cached derived indexes skip raw page bytes.
func feedPrewarm(jobs []prewarmJob, in chan<- prewarmJob) {
	stores := map[*Store][]prewarmJob{}
	for _, j := range jobs {
		if st, ok := j.fs.(*Store); ok {
			stores[st] = append(stores[st], j)
			continue
		}
		in <- j
	}
	for st, js := range stores {
		slices.SortFunc(js, func(x, y prewarmJob) int { return strings.Compare(Clean(x.name), Clean(y.name)) })
		after := ""
		for i := 0; i < len(js); {
			part, last, err := st.ReadPrewarmFrom(PageFileName, after, prewarmChunk)
			done := err != nil || len(part) < prewarmChunk // the last part: what is left is read by itself
			for ; i < len(js) && (done || Clean(js[i].name) <= last); i++ {
				if row, ok := part[Clean(js[i].name)]; ok {
					js[i].store = st
					js[i].mtime = row.MTime
					js[i].size = row.Size
					js[i].cached = row.Index
					js[i].src = row.Src
				}
				in <- js[i]
				js[i].cached = nil
				js[i].src = nil
			}
			after = last
		}
	}
}

// takePrewarm hands pages read to the indexes, on the window's side: a page written since it was read
// is left for them to read again.
func (a *app) takePrewarm(got []prewarmRead) {
	if len(got) == 0 {
		return
	}
	a.do(func() error {
		l := &a.links
		if l.pages == nil {
			l.pages = map[*Page]*pageIdx{}
		}
		if l.plain == nil {
			l.plain = map[*Page]*plainIdx{}
		}
		for _, r := range got {
			if !r.j.p.Modified.Equal(r.j.mod) || r.j.p.Writes != r.j.writes {
				continue
			}
			if l.pages[r.j.p] == nil {
				l.pages[r.j.p] = r.idx
			}
			if l.plain[r.j.p] == nil {
				l.plain[r.j.p] = r.plain
			}
		}
		return nil
	})
}
