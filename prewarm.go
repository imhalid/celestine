package main

import (
	"runtime"
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
}

type prewarmRead struct {
	j     prewarmJob
	idx   *pageIdx
	plain *plainIdx
}

// prewarm starts the reading. While it goes on, the indexes do not read pages on the window's thread.
func (a *app) prewarm() {
	var jobs []prewarmJob
	for _, c := range a.lib.Collections {
		for _, p := range c.All() {
			jobs = append(jobs, prewarmJob{p, p.fs(), p.file(), p.Modified, p.writes})
		}
	}
	if len(jobs) == 0 {
		return
	}
	a.prewarming = true
	go func() {
		defer a.guard()
		defer a.do(func() error { a.prewarming = false; return nil })
		// a library in one file gives all its pages in one pass
		bulk := map[FS]map[string][]byte{}
		for _, j := range jobs {
			if st, ok := j.fs.(*Store); ok && bulk[j.fs] == nil {
				if m, err := st.ReadNamed(pageFileName); err == nil {
					bulk[j.fs] = m
				}
			}
		}
		in := make(chan prewarmJob)
		out := make(chan prewarmRead, 64)
		var wg sync.WaitGroup
		for range max(1, runtime.NumCPU()-1) { // a core left for the window
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer a.guard()
				for j := range in {
					src, ok := bulk[j.fs][clean(j.name)]
					if !ok {
						var err error
						if src, err = j.fs.ReadFile(j.name); err != nil {
							continue
						}
					}
					_, body := splitPage(src)
					e, plain := readPageIdxAt(j.p, j.mod, j.writes, textOfBody(body))
					out <- prewarmRead{j, e, &plainIdx{mod: j.mod, writes: j.writes, plain: plain, folded: fold(plain)}}
				}
			}()
		}
		go func() {
			for _, j := range jobs {
				in <- j
			}
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
			if !r.j.p.Modified.Equal(r.j.mod) || r.j.p.writes != r.j.writes {
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
