package work

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/donvargax/itos/v2/internal/value"
)

// PrintList writes every item as `work list` prints it (slice 43), one line
// each in the registry's order: its id, kind, status and title, the first
// three in columns as wide as their widest. An item that gives no kind or no
// status shows "-" there, so the columns hold, and one with no title
// nothing after them.
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
