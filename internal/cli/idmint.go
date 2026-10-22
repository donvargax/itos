package cli

import (
	"context"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/ids"
)

// newItemID is the item id a command that mints one writes: the id a draft
// reserved for it (Out's reserved), else one minted now.
func newItemID(cfg *config.Loaded, category string, registryIDs []string, reserved, goos string) (string, error) {
	if reserved != "" {
		return reserved, nil
	}
	return mintItemID(cfg, category, registryIDs, goos)
}

// mintItemID is ids.Mint, through the tracker of the config (tracker.go).
func mintItemID(cfg *config.Loaded, category string, registryIDs []string, goos string) (string, error) {
	return ids.Mint(context.Background(), tracker(cfg, goos), cfg, category, registryIDs)
}
