package guide

import (
	_ "embed"
	"slices"
	"strings"
)

//go:embed coordinate.md
var coordinate string

//go:embed work.md
var work string

// Coordinate is the name of the coordinator's guide, the one itos go prints.
const Coordinate = "coordinate"

var guides = map[string]string{
	Coordinate: coordinate,
	"work":     work,
}

// Text is the guide by its name, as written, and whether there is one. Its
// lines end in LF wherever the binary was built: go:embed takes a file's
// bytes as the checkout has them, and a checkout that writes text with CRLF
// (git's core.autocrlf, as on a Windows runner) would embed it so.
func Text(name string) (string, bool) {
	text, ok := guides[name]
	return strings.ReplaceAll(text, "\r\n", "\n"), ok
}

// Names are the guides' names, sorted.
func Names() []string {
	names := make([]string, 0, len(guides))
	for name := range guides {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
