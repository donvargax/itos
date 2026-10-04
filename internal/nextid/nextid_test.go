package nextid

import (
	"regexp"
	"testing"
)

// One past the highest of the series, at the width it writes or the one
// asked for; other series' IDs count for nothing; a pattern widens it.
func TestNext(t *testing.T) {
	three := regexp.MustCompile(`^T-\d{3}$`).MatchString
	for _, c := range []struct {
		prefix string
		ids    []string
		width  int
		fits   func(string) bool
		want   string
	}{
		{"ID-A-", []string{"ID-A-01", "ID-A-03", "ID-AB-09", "ID-A-x"}, 2, nil, "ID-A-04"},
		{"ID-NEW-", []string{"ID-A-01"}, 2, nil, "ID-NEW-01"},
		{"slice-", []string{"slice-7", "slice-9", "slice-"}, 1, nil, "slice-10"},
		{"ID-A-", []string{"ID-A-99"}, 2, nil, "ID-A-100"},
		{"T-", []string{"T-001", "T-002"}, 1, nil, "T-003"},
		{"T-", nil, 1, three, "T-001"},
		{"T-", []string{"T-999"}, 1, three, "T-1000"},
	} {
		if got := Next(c.prefix, c.ids, c.width, c.fits); got != c.want {
			t.Errorf("Next(%q, %q, %d): %q, want %q", c.prefix, c.ids, c.width, got, c.want)
		}
	}
}
