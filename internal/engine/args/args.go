package args

import (
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/out"
)

// FlagValue is the value after a flag, and whether there is one.
func FlagValue(args []string, name string) (string, bool) {
	i := slices.Index(args, name)
	if i < 0 || i+1 >= len(args) {
		return "", false
	}
	return args[i+1], true
}

// Positional is the arguments that are neither flags nor the value of one of
// the valued flags.
func Positional(args []string, valued ...string) []string {
	var found []string
	for i, a := range args {
		if strings.HasPrefix(a, "-") || (i > 0 && slices.Contains(valued, args[i-1])) {
			continue
		}
		found = append(found, a)
	}
	return found
}

// First is a list's first element, and whether it has one.
func First(list []string) (string, bool) {
	if len(list) == 0 {
		return "", false
	}
	return list[0], true
}

// Split is a command's subcommand and the arguments after it.
func Split(args []string) (string, []string) {
	if len(args) == 0 {
		return "", nil
	}
	return args[0], args[1:]
}

// Sub reads a subcommand's arguments: its positional ones, the valued
// flags' values and the switches given. A flag it does not take, a valued
// flag with no value or an empty one, and a flag given twice are usage
// errors (out.Usage); after "--" every argument is positional.
func Sub(command, sub string, args []string, valued, switches []string) ([]string, map[string]string, map[string]bool, error) {
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
				return nil, nil, nil, out.Usage("%s takes one %s", name, flag)
			}
			set[flag] = true
		case slices.Contains(valued, flag):
			if !joined {
				if i+1 >= len(args) {
					return nil, nil, nil, out.Usage("%s %s needs a value", name, flag)
				}
				i++
				value = args[i]
			}
			if _, twice := values[flag]; twice || strings.TrimSpace(value) == "" {
				return nil, nil, nil, out.Usage("%s takes one %s with a value", name, flag)
			}
			values[flag] = value
		default:
			return nil, nil, nil, out.Usage("%s does not take %s", name, flag)
		}
	}
	return pos, values, set, nil
}
