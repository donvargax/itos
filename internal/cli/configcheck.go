package cli

import (
	"fmt"

	"github.com/donvargax/itos/internal/config"
	"github.com/donvargax/itos/internal/out"
	"github.com/donvargax/itos/internal/value"
)

// printDefaults is `config check --print-defaults`: the one table of
// defaults the loader applies, as YAML or JSON, as it applies to this
// config's ledger. It checks nothing, so a config that does not load gets the
// table's own values.
func printDefaults(o Out) (int, error) {
	var file *value.Map
	if cfg, err := config.Load(config.Path()); err == nil {
		file = cfg.File()
	}
	table := config.DefaultsFor(file)
	if o.JSON {
		return 0, out.Emit(o.Stdout, out.Field{Key: "defaults", Value: table})
	}
	_, err := fmt.Fprint(o.Stdout, value.YAML(table))
	return 0, err
}
