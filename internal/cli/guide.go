package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/git"
	"github.com/donvargax/itos/v2/internal/guide"
	"github.com/donvargax/itos/v2/internal/out"
	"github.com/donvargax/itos/v2/internal/value"
)

// itos go and itos guide (features/guide.feature, slice 64) print the guides
// embedded in the binary (internal/guide) as written. itos go, the same as
// itos guide coordinate, prints the coordinator's guide and then, after a
// separating line, the repository's own notes: the file guide.orchestrating
// names (docs/ORCHESTRATING.md by default; under the git folder for a stealth
// config, as its other data), read where the config's paths are read, or from
// the repository's top when there is no config. They need no config and
// write nothing; outside a repository the guide prints alone.

// separator stands between the guide and the repository's own notes.
const separator = "\n---\n\n"

// goCommand is itos go: the coordinator's guide.
func goCommand(args []string, o Out) (int, error) {
	if len(args) > 0 {
		return 0, usage("go takes no arguments: %s", strings.Join(args, " "))
	}
	return printGuide(guide.Coordinate, o)
}

// guideCommand is itos guide <name>.
func guideCommand(args []string, o Out) (int, error) {
	names := strings.Join(guide.Names(), ", ")
	name, rest := split(args)
	switch {
	case name == "":
		return 0, usage("guide needs a name: %s", names)
	case len(rest) > 0:
		return 0, usage("guide takes one name: %s", strings.Join(args, " "))
	}
	if _, ok := guide.Text(name); !ok {
		return 0, usage("no guide %s: the guides are %s", name, names)
	}
	return printGuide(name, o)
}

// printGuide prints the guide by its name, the coordinator's followed by
// the repository's own notes when it keeps them.
func printGuide(name string, o Out) (int, error) {
	text, _ := guide.Text(name)
	notesFile := ""
	if name == guide.Coordinate {
		if notes, file := orchestratingNotes(o); file != "" {
			text = strings.TrimRight(text, "\n") + "\n" + separator + notes
			notesFile = file
		}
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if o.JSON {
		fields := []out.Field{{Key: "guide", Value: name}, {Key: "text", Value: text}}
		if notesFile != "" {
			fields = append(fields, out.Field{Key: "notes", Value: filepath.ToSlash(notesFile)})
		}
		return 0, out.Emit(o.Stdout, fields...)
	}
	_, err := fmt.Fprint(o.Stdout, text)
	return 0, err
}

// orchestratingNotes is the repository's own notes and the path they were
// read from, both "" when it keeps none. A file that is there but cannot be
// read is a warning, and the guide prints without it.
func orchestratingNotes(o Out) (text, path string) {
	path = notesPath(o)
	if path == "" {
		return "", ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(o.Stderr, "itos: %s cannot be read, so the guide prints without it: %v\n", path, err)
		}
		return "", ""
	}
	if strings.TrimSpace(string(data)) == "" {
		return "", ""
	}
	text = string(data)
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return text, path
}

// notesPath is the file itos go appends: guide.orchestrating as the config
// gives it, relative to where itos runs, or, with no config, its default
// from the repository's top; "" outside a repository with no config. A
// config that cannot be read is a warning, and the default is read instead.
func notesPath(o Out) string {
	file := config.Path()
	if _, err := os.Stat(file); err == nil {
		loaded, err := config.Load(file)
		if err == nil {
			p, _ := loaded.Get("guide.orchestrating").(string)
			return p
		}
		fmt.Fprintf(o.Stderr, "itos: %s cannot be read, so guide.orchestrating is its default: %v\n", file, err)
	}
	// The way up to the top, "" at the top itself: a relative path, as the
	// config's paths are, whatever links the folders' names go through.
	up, err := git.Output("rev-parse", "--show-cdup")
	if err != nil {
		return ""
	}
	p, _ := value.Prop(value.Prop(config.DefaultsFor(value.NewMap(), false), "guide"), "orchestrating").(string)
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(strings.TrimSpace(up), filepath.FromSlash(p))
}
