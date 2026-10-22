package ledger

import (
	"slices"

	"github.com/donvargax/itos/v8/internal/config"
)

// GroupNames are the flags that name a group whatever the config's label.
var GroupNames = []string{"--group", "--phase"}

// GroupFlags are the flags that name a group: GroupNames and the label's
// own (ledger.group.label), all the same.
func GroupFlags(cfg *config.Loaded) []string {
	flags := slices.Clone(GroupNames)
	if own := "--" + cfg.Ledger.Group.Label; !slices.Contains(flags, own) {
		flags = append(flags, own)
	}
	return flags
}
