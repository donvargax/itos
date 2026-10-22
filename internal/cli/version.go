package cli

import (
	"context"

	versionslice "github.com/donvargax/itos/v8/internal/slices/version"
	"github.com/donvargax/itos/v8/internal/version"
)

// runVersion is the version slice (internal/slices/version) over its
// ports bound to the real code.
func runVersion(args []string, o Out) (int, error) {
	h := versionslice.NewHandler(versionPorts{})
	return versionslice.Parse(args, o.JSON).Run(context.Background(), o.Stdout, o.Stderr, h)
}

// versionPorts are the version slice's ports over the real code.
type versionPorts struct{}

// BuildVersion is version.Version.
func (versionPorts) BuildVersion(context.Context) string {
	return version.Version()
}

// Requires is the requires of the config config.Path names.
func (versionPorts) Requires(context.Context) (*string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	return cfg.Requires, nil
}

// Satisfies is version.Satisfies.
func (versionPorts) Satisfies(_ context.Context, v, r string) bool {
	return version.Satisfies(v, r)
}
