// Package guide holds the guides a session starts from (features/guide.feature,
// slice 64), embedded in the binary so that one text, versioned with itos,
// serves every repository it manages: coordinate.md, the coordinator's, which
// itos go prints, and work.md, the implementer's, which itos guide work
// prints. They are generic: what one repository learns that holds for no
// other stays in that repository's own notes, which itos go appends
// (guide.orchestrating in the config, internal/cli/guide.go).
package guide

import (
	_ "embed"
	"slices"
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

// Text is the guide by its name, as written, and whether there is one.
func Text(name string) (string, bool) {
	text, ok := guides[name]
	return text, ok
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
