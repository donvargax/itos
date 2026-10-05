package work

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/donvargax/itos/v4/internal/value"
)

// Open is the items work list prints by default (slice 79): those not
// closed, in the order given. Closed is done or dropped, the two statuses
// itos knows to end an item (bug 25), so blocked, any status a project adds
// to work.statuses and an item with no status are open; done and dropped
// are left to work list --all.
func Open(items []*value.Map) []*value.Map {
	open := []*value.Map{}
	for _, item := range items {
		if status := item.At("status"); status != "done" && status != "dropped" {
			open = append(open, item)
		}
	}
	return open
}

// PrintList writes the items as `work list` prints them (slice 43), one line
// each in the order given: its id, kind, status and title, the first three
// in columns as wide as their widest. An item that gives no kind or no
// status shows "-" there, so the columns hold, and one with no title
// nothing after them; a deferred one (deferred: <reason>) has (deferred)
// after its title (slice 79), since its status alone says todo.
func PrintList(w io.Writer, items []*value.Map) {
	cell := func(v any) string {
		if absent(v) {
			return "-"
		}
		return value.String(v)
	}
	rows := make([][4]string, len(items))
	var widths [3]int
	for i, item := range items {
		title := ""
		if t := item.At("title"); !absent(t) {
			title = value.String(t)
		}
		if !absent(item.At("deferred")) {
			title = strings.TrimLeft(title+"  (deferred)", " ")
		}
		rows[i] = [4]string{cell(item.At("id")), cell(item.At("kind")), cell(item.At("status")), title}
		for c := range widths {
			widths[c] = max(widths[c], utf8.RuneCountInString(rows[i][c]))
		}
	}
	pad := func(s string, width int) string {
		return s + strings.Repeat(" ", width-utf8.RuneCountInString(s))
	}
	for _, row := range rows {
		fmt.Fprintln(w, strings.TrimRight(pad(row[0], widths[0])+"  "+pad(row[1], widths[1])+"  "+pad(row[2], widths[2])+"  "+row[3], " "))
	}
}
