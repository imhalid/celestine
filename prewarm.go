package main

import (
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
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
	name   string
	mod    time.Time
	writes int
	src    []byte // its file, read with others (a library's file): nil to be read by itself
}

// prewarmChunk is how many page files of a library's file are read at once: the reading holds that many, not all.
// A test makes it smaller.
var prewarmChunk = 2000

type prewarmRead struct {
	j     prewarmJob
	idx   *pageIdx
	plain *plainIdx
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
		defer a.do(func() error { a.prewarming = false; return nil })
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
					src := j.src
					if src == nil {
						var err error
						if src, err = j.fs.ReadFile(j.name); err != nil {
							continue
						}
					}
					_, body := SplitPage(src)
					e, plain := readPageIdxAt(j.p, j.mod, j.writes, TextOfBody(body))
					out <- prewarmRead{j, e, &plainIdx{mod: j.mod, writes: j.writes, folded: fold(plain)}}
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
		for r := range out {
			if got = append(got, r); len(got) >= prewarmBatch {
				a.takePrewarm(got)
				got = nil
			}
		}
		a.takePrewarm(got)
	}()
}

// feedPrewarm gives the workers the jobs, the pages of a library's file with their files read in parts of
// prewarmChunk, in the order of their paths, a query a part: the files of a part are let go once its pages are
// handed on, where all of them were read and held at once (#196). The others read their own.
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
			part, last, err := st.ReadNamedFrom(PageFileName, after, prewarmChunk)
			done := err != nil || len(part) < prewarmChunk // the last part: what is left is read by itself
			for ; i < len(js) && (done || Clean(js[i].name) <= last); i++ {
				js[i].src = part[Clean(js[i].name)]
				in <- js[i]
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
