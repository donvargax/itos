package guides

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/agentrules"
	"github.com/donvargax/itos/v8/internal/guide"
	"github.com/donvargax/itos/v8/internal/value"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Separator stands between the guide and the repository's own notes.
const Separator = "\n---\n\n"

// localNotesName is this clone's own notes' file in itos's folder of the git
// common dir.
const localNotesName = "notes.md"

// LocalNotesHeading heads this clone's own notes where itos go prints them.
const LocalNotesHeading = "# This clone's own notes"

// Config is the config the guides read, as the command found it: File, its
// path; Loaded, the config, nil when it is not there or cannot be read;
// Err, why it cannot be read, nil when it is not there.
type Config struct {
	File   string
	Loaded *config.Loaded
	Err    error
}

// there is whether the config is there, read or not.
func (c Config) there() bool { return c.Loaded != nil || c.Err != nil }

// Guide is a guide as printed: its Text; Notes, the file of the
// repository's own notes appended after the coordinator's guide, and Local,
// this clone's own notes' file, each "" when none was.
type Guide struct {
	Text, Notes, Local string
}

// Compose is the guide by its name, the coordinator's followed by the
// repository's own notes when it keeps them (c is read for those alone),
// then, when local is set, by this clone's own notes when it keeps them;
// the text ends with a newline.
func Compose(ctx context.Context, v vcsport.VCS, c Config, name string, local bool, stderr io.Writer) Guide {
	text, _ := guide.Text(name)
	var g Guide
	if name == guide.Coordinate {
		if notes, file := Notes(ctx, v, c, stderr); file != "" {
			text = strings.TrimRight(text, "\n") + "\n" + Separator + notes
			g.Notes = file
		}
	}
	if local {
		if notes, file := LocalNotes(ctx, v, g.Notes, stderr); file != "" {
			text = strings.TrimRight(text, "\n") + "\n" + Separator + LocalNotesHeading + "\n\n" +
				"These are uncommitted and hold what is true on this machine only (" +
				filepath.ToSlash(file) + ").\n\n" + notes
			g.Local = file
		}
	}
	g.Text = withNewline(text)
	return g
}

// withNewline is text ending with a newline, one added when it has none.
func withNewline(text string) string {
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return text
}

// CommitPolicy is the commit types, footers and paths the config sets
// (agentrules.CommitPolicy), under a heading of their own; "" with no
// config. A config that cannot be read is a warning, and the procedure
// prints without them.
func CommitPolicy(c Config, stderr io.Writer) string {
	if c.Err != nil {
		fmt.Fprintf(stderr, "itos: %s cannot be read, so the commit rules it sets are not printed: %s\n",
			c.File, strings.ReplaceAll(strings.TrimSpace(c.Err.Error()), "\n", "; "))
		return ""
	}
	if c.Loaded == nil {
		return ""
	}
	paras := append([]string{
		"## The rules the config sets",
		"As " + agentrules.Code(filepath.ToSlash(c.File)) + " sets them; the hooks and CI hold every commit to them.",
	}, agentrules.CommitPolicy(c.Loaded, agentrules.Code)...)
	return strings.Join(paras, "\n\n") + "\n"
}

// Notes is the repository's own notes and the path they were read from,
// both "" when it keeps none. A file that is there but cannot be read is a
// warning, and the guide prints without it.
func Notes(ctx context.Context, v vcsport.VCS, c Config, stderr io.Writer) (text, path string) {
	return readNotes(v.Folder(ctx), notesPath(ctx, v, c, stderr), stderr)
}

// LocalNotes is this clone's own notes and the path they were read from,
// both "" when it keeps none, outside a repository, or when they are the
// file the repository's notes were read from (a stealth config's
// guide.orchestrating naming it), which is printed once, as those.
func LocalNotes(ctx context.Context, v vcsport.VCS, printed string, stderr io.Writer) (text, path string) {
	l, err := v.Locate(ctx, "")
	if err != nil || l.Common == "" {
		return "", ""
	}
	path = filepath.Join(l.Common, config.StealthFolder, localNotesName)
	if printed != "" {
		a, errA := os.Stat(vcsport.Within(ctx, v, printed))
		b, errB := os.Stat(vcsport.Within(ctx, v, path))
		if errA == nil && errB == nil && os.SameFile(a, b) {
			return "", ""
		}
	}
	return readNotes(v.Folder(ctx), path, stderr)
}

// readNotes is the notes the file at path holds, read from the folder dir
// when relative, and the path, both "" when path is "", the file is not
// there or holds only blanks. A file that is there but cannot be read is a
// warning, and the guide prints without it.
func readNotes(dir, path string, stderr io.Writer) (string, string) {
	if path == "" {
		return "", ""
	}
	data, err := os.ReadFile(vcsport.In(dir, path))
	if failed := (*fs.PathError)(nil); errors.As(err, &failed) {
		failed.Path = path
	}
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(stderr, "itos: %s cannot be read, so the guide prints without it: %v\n", path, err)
		}
		return "", ""
	}
	if strings.TrimSpace(string(data)) == "" {
		return "", ""
	}
	return withNewline(string(data)), path
}

// notesPath is the file itos go appends: guide.orchestrating as the config
// gives it, relative to where itos runs, or, with no config, its default
// from the repository's top; "" outside a repository with no config. A
// config that cannot be read is a warning, and the default is read instead.
func notesPath(ctx context.Context, v vcsport.VCS, c Config, stderr io.Writer) string {
	if c.there() {
		if c.Err == nil {
			p, _ := c.Loaded.Get("guide.orchestrating").(string)
			return p
		}
		fmt.Fprintf(stderr, "itos: %s cannot be read, so guide.orchestrating is its default: %v\n", c.File, c.Err)
	}
	// The way up to the top, "" at the top itself: a relative path, as the
	// config's paths are, whatever links the folders' names go through.
	up, ok := upToTop(ctx, v)
	if !ok {
		return ""
	}
	p, _ := value.Prop(value.Prop(config.DefaultsFor(value.NewMap(), false), "guide"), "orchestrating").(string)
	if p != "" && !filepath.IsAbs(p) {
		p = filepath.Join(up, filepath.FromSlash(p))
	}
	return p
}

// upToTop is the way from the folder itos runs in (v's Folder) up to the work tree's top
// as git rev-parse --show-cdup gives it, a ../ for each folder between, the
// folder's links resolved as the top's are, none in the git folder; not ok
// outside a repository.
func upToTop(ctx context.Context, v vcsport.VCS) (string, bool) {
	l, err := v.Locate(ctx, "")
	if err != nil || l.Top == "" {
		return "", err == nil
	}
	cwd := v.Folder(ctx)
	if real, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = real
	}
	rel, err := filepath.Rel(l.Top, cwd)
	if err != nil || rel == "." {
		return "", err == nil
	}
	return strings.Repeat("../", len(strings.Split(filepath.ToSlash(rel), "/"))), true
}
