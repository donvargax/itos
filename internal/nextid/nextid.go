// Package nextid is the next free ID of a series (slice 60): `itos tests
// next-id`'s tag and `itos task next-id`'s task ID, each one past the highest
// number the series already holds, as internal/ask's NextID gives a question
// its id, so that a writer never greps for it.
package nextid

import (
	"fmt"
	"strconv"
)

// Next is the ID after the highest of ids that is the prefix and a number,
// prefix and number alike: the others are another series' and count for
// nothing. Its number is zero-padded to width, or to the widest number the
// series writes when that is wider, so 03 is followed by 04 and T-084 by
// T-085; and, when fits is given, to the narrowest width from there up that
// it accepts, so a pattern asking for three digits gets them. A series with
// no number yet starts at 1.
func Next(prefix string, ids []string, width int, fits func(string) bool) string {
	highest := 0
	for _, id := range ids {
		if len(id) <= len(prefix) || id[:len(prefix)] != prefix || !digits(id[len(prefix):]) {
			continue
		}
		written := id[len(prefix):]
		n, err := strconv.Atoi(written)
		if err != nil {
			continue
		}
		highest = max(highest, n)
		width = max(width, len(written))
	}
	at := func(w int) string { return fmt.Sprintf("%s%0*d", prefix, w, highest+1) }
	if fits != nil {
		for w := width; w <= width+8; w++ {
			if fits(at(w)) {
				return at(w)
			}
		}
	}
	return at(width)
}

// digits is whether s is one or more ASCII digits and nothing else.
func digits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
