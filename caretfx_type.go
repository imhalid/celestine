package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// clock keeps a time of its own for a caret.
type clock struct{ base time.Time }

func (c *clock) at(p *ui.Painter) float64 {
	now := p.Now()
	if c.base.IsZero() {
		c.base = now
	}
	return ms(now.Sub(c.base))
}
