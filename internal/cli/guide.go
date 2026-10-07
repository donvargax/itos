package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/git"
	"github.com/donvargax/itos/v7/internal/guide"
	"github.com/donvargax/itos/v7/internal/out"
	"github.com/donvargax/itos/v7/internal/value"
)

// itos go and itos guide (features/guide.feature, slice 64) print the guides
// embedded in the binary (internal/guide) as written. itos go, the same as
// itos guide coordinate, prints the coordinator's guide and then, after a
// separating line, the repository's own notes: the file guide.orchestrating
// names (docs/ORCHESTRATING.md by default; under the git folder for a stealth
// config, as its other data), read where the config's paths are read, or from
// the repository's top when there is no config. itos go alone then prints
// this clone's own notes (slice 68): notes.md in itos's folder of the git
// common dir, which git never commits and every linked worktree shares, for
// what holds on this machine only. They need no config and write nothing;
// outside a repository the guide prints alone.

// separator stands between the guide and the repository's own notes.
const separator = "\n---\n\n"

// localNotesName is this clone's own notes' file in itos's folder of the git
// common dir.
const localNotesName = "notes.md"

// localNotesHeading heads this clone's own notes where itos go prints them.
const localNotesHeading = "# This clone's own notes"

// goCommand is itos go: the coordinator's guide, then where things stand
// (status.go, slice 67), so a session starts from one command.
func goCommand(args []string, o Out) (int, error) {
	if len(args) > 0 {
		return 0, usage("go takes no arguments: %s", strings.Join(args, " "))
	}
	return printGuideThen(guide.Coordinate, true, goStatus(o), o)
}

// goStatus is where things stand for itos go, nil when it cannot be
// computed at all (no config, a registry that is not there or not sound),
// which is one line on stderr, never a failure of the guide.
func goStatus(o Out) *standing {
	skip := func(why string) *standing {
		fmt.Fprintf(o.Stderr, "itos: where things stand is not printed: %s\n", why)
		return nil
	}
	file := config.Path()
	if _, err := os.Stat(file); err != nil {
		return skip("there is no itos config here (itos init makes one)")
	}
	cfg, err := config.Load(file)
	if err != nil {
		return skip(strings.ReplaceAll(strings.TrimSpace(err.Error()), "\n", "; "))
	}
	st, found, _, err := statusOf(cfg, "", o)
	switch {
	case err != nil:
		return skip(err.Error())
	case len(found) > 0:
		return skip(found[0].Message + " (itos work check)")
	}
	return st
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
func printGuide(name string, o Out) (int, error) { return printGuideThen(name, false, nil, o) }

// printGuideThen is printGuide followed, when local is set, by this clone's
// own notes when it keeps them, and then, after a line of ---, by where
// things stand when st is not nil, under --json its object as "status".
func printGuideThen(name string, local bool, st *standing, o Out) (int, error) {
	text, _ := guide.Text(name)
	notesFile, localFile := "", ""
	if name == guide.Coordinate {
		if notes, file := orchestratingNotes(o); file != "" {
			text = strings.TrimRight(text, "\n") + "\n" + separator + notes
			notesFile = file
		}
	}
	if local {
		if notes, file := localNotes(notesFile, o); file != "" {
			text = strings.TrimRight(text, "\n") + "\n" + separator + localNotesHeading + "\n\n" +
				"These are uncommitted and hold what is true on this machine only (" +
				filepath.ToSlash(file) + ").\n\n" + notes
			localFile = file
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
		if localFile != "" {
			fields = append(fields, out.Field{Key: "local_notes", Value: filepath.ToSlash(localFile)})
		}
		if st != nil {
			fields = append(fields, out.Field{Key: "status", Value: st})
		}
		return 0, out.Emit(o.Stdout, fields...)
	}
	if _, err := fmt.Fprint(o.Stdout, text); err != nil || st == nil {
		return 0, err
	}
	fmt.Fprint(o.Stdout, separator)
	st.print(o.Stdout)
	return 0, nil
}

// orchestratingNotes is the repository's own notes and the path they were
// read from, both "" when it keeps none. A file that is there but cannot be
// read is a warning, and the guide prints without it.
func orchestratingNotes(o Out) (text, path string) { return readNotes(notesPath(o), o) }

// localNotes is this clone's own notes and the path they were read from,
// both "" when it keeps none, outside a repository, or when they are the
// file the repository's notes were read from (a stealth config's
// guide.orchestrating naming it), which is printed once, as those.
func localNotes(printed string, o Out) (text, path string) {
	common, err := git.Output("rev-parse", "--git-common-dir")
	if common = strings.TrimSpace(common); err != nil || common == "" {
		return "", ""
	}
	path = filepath.Join(common, config.StealthFolder, localNotesName)
	if printed != "" {
		a, errA := os.Stat(printed)
		b, errB := os.Stat(path)
		if errA == nil && errB == nil && os.SameFile(a, b) {
			return "", ""
		}
	}
	return readNotes(path, o)
}

// readNotes is the notes the file at path holds and the path, both "" when
// path is "", the file is not there or holds only blanks. A file that is
// there but cannot be read is a warning, and the guide prints without it.
func readNotes(path string, o Out) (string, string) {
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
	text := string(data)
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
