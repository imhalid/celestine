package library

import (
	"slices"
	"strconv"
	"strings"
)

// A Filter is a condition a view's pages meet: property Prop ("title" for the title) compared by Op with Value.
// Op is "is", "isNot", "contains", "empty", "notEmpty", or for a number or a day "lt" and "gt" (less, greater; a
// day before, after).
type Filter struct {
	Prop  string `json:"prop"`
	Op    string `json:"op"`
	Value string `json:"value,omitempty"`
}

// FilterOps are the comparisons a filter on a property of type typ ("title" for the title) can make.
func FilterOps(typ string) []string {
	switch typ {
	case PropNumber, PropDate:
		return []string{"is", "isNot", "lt", "gt", "empty", "notEmpty"}
	case PropChoice, PropPage, PropFile:
		return []string{"is", "isNot", "empty", "notEmpty"}
	case PropTags, PropAliases:
		return []string{"contains", "empty", "notEmpty"}
	case "title":
		return []string{"contains", "is", "isNot"}
	}
	return []string{"contains", "is", "isNot", "empty", "notEmpty"}
}

// Match reports whether page p of collection c meets every filter of view v; a filter on a property c no longer has
// lets every page through.
func (v *View) Match(c *Collection, p *Page) bool {
	for _, f := range v.Filters {
		if !f.match(c, p) {
			return false
		}
	}
	return true
}

func (f Filter) match(c *Collection, p *Page) bool {
	val, pr := p.Title, (*Property)(nil)
	if f.Prop != "title" {
		if pr = c.Prop(f.Prop); pr == nil {
			return true
		}
		val = p.Value(pr)
	}
	want := strings.TrimSpace(f.Value)
	switch f.Op {
	case "empty":
		return val == ""
	case "notEmpty":
		return val != ""
	case "contains":
		if pr != nil && IsList(pr.Type) { // one of its tags, whole
			return slices.ContainsFunc(SplitTags(val), func(t string) bool { return strings.EqualFold(t, want) })
		}
		return strings.Contains(strings.ToLower(val), strings.ToLower(want))
	case "is":
		return strings.EqualFold(val, want)
	case "isNot":
		return !strings.EqualFold(val, want)
	case "lt", "gt":
		if val == "" || want == "" || pr == nil {
			return false
		}
		cmp := CompareValues(pr, val, want)
		return f.Op == "lt" && cmp < 0 || f.Op == "gt" && cmp > 0
	}
	return true
}

// CompareValues orders two values of property pr: an empty one last, a choice by its options' order, a number as a
// number, the rest as text, ignoring case (a day, 2006-01-02, sorts as text).
func CompareValues(pr *Property, a, b string) int {
	switch {
	case a == b:
		return 0
	case a == "":
		return 1
	case b == "":
		return -1
	}
	switch pr.Type {
	case PropChoice:
		return slices.Index(pr.Options, a) - slices.Index(pr.Options, b)
	case PropNumber:
		x, _ := strconv.ParseFloat(strings.ReplaceAll(a, ",", "."), 64)
		y, _ := strconv.ParseFloat(strings.ReplaceAll(b, ",", "."), 64)
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}
