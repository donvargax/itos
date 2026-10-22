package cli

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/slices/followup"
	"github.com/donvargax/itos/v8/internal/slices/followup/domain"
	"github.com/donvargax/itos/v8/internal/store/private/adapter"
)

// followCommand is `followup`, the followup slice (internal/slices/followup)
// over its ports bound to the real code; a doc path is read from where itos
// was started, before a run from a subfolder moved to the top. A threads'
// file the domain refuses is a data file itos refuses, exit 2.
func followCommand(args []string, o Out) (int, error) {
	c, err := followup.Parse(args, o.Origin, o.JSON, o.Quiet)
	if err != nil {
		return 0, usage("%s", err)
	}
	code, err := c.Run(context.Background(), o.Stdout, o.Stderr, followup.NewHandler(followupPorts{adapter.Store{OS: o.OS}}))
	var invalid *domain.Invalid
	if errors.As(err, &invalid) {
		err = kind.Wrap(kind.Usage, err)
	}
	return code, err
}

// followupPorts are the followup slice's ports over the real code: the
// private store's adapter and the clock.
type followupPorts struct{ adapter.Store }

// Now is time.Now.
func (followupPorts) Now(context.Context) time.Time { return time.Now() }

// subArgs reads a subcommand's arguments: its positional ones, the valued
// flags' values and the switches given. A flag it does not take, a valued
// flag with no value or an empty one, and a flag given twice are usage
// errors; after "--" every argument is positional.
func subArgs(command, sub string, args []string, valued, switches []string) ([]string, map[string]string, map[string]bool, error) {
	name := strings.TrimSpace(command + " " + sub)
	var pos []string
	values := map[string]string{}
	set := map[string]bool{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			pos = append(pos, arg)
			continue
		}
		flag, value, joined := strings.Cut(arg, "=")
		switch {
		case slices.Contains(switches, flag) && !joined:
			if set[flag] {
				return nil, nil, nil, usage("%s takes one %s", name, flag)
			}
			set[flag] = true
		case slices.Contains(valued, flag):
			if !joined {
				if i+1 >= len(args) {
					return nil, nil, nil, usage("%s %s needs a value", name, flag)
				}
				i++
				value = args[i]
			}
			if _, twice := values[flag]; twice || strings.TrimSpace(value) == "" {
				return nil, nil, nil, usage("%s takes one %s with a value", name, flag)
			}
			values[flag] = value
		default:
			return nil, nil, nil, usage("%s does not take %s", name, flag)
		}
	}
	return pos, values, set, nil
}
