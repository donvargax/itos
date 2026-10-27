package ci

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
)

// maxAnnotations is how many error annotations GitHub keeps for a step.
const maxAnnotations = 10

// maxLine is how much of one output line the run reads for a path and line.
const maxLine = 8192

// annotator prints the run's failures as GitHub Actions workflow commands
// (slice 198), at most maxAnnotations a run, and reads the output of the
// step or task check being run for the file and line it names.
type annotator struct {
	cfg      *config.Loaded
	w        io.Writer
	printed  int
	out, err finder
}

func newAnnotator(cfg *config.Loaded, w io.Writer) *annotator {
	folder := cfg.Folder()
	return &annotator{cfg: cfg, w: w, out: finder{folder: folder}, err: finder{folder: folder}}
}

// watch resets what was read and gives the streams a command writes to: the
// given ones, each also read for a file and line.
func (a *annotator) watch(stdout, stderr io.Writer) (io.Writer, io.Writer) {
	a.out.reset()
	a.err.reset()
	return io.MultiWriter(stdout, &a.out), io.MultiWriter(stderr, &a.err)
}

// run prints the failure of the command last watched: at the first line of
// its stdout that names a file and line, else of its stderr, with the rest of
// that line as the reason; else plain, with the reason given.
func (a *annotator) run(what, reason string) {
	a.out.flush()
	a.err.flush()
	h := a.out.hit
	if h == nil {
		h = a.err.hit
	}
	if h != nil {
		if file, ok := a.rooted(h.path); ok {
			a.emit(file, h.line, what+" failed: "+h.reason)
			return
		}
	}
	a.emit("", 0, what+" failed: "+reason)
}

// emit prints one workflow command, with a file and line when file is given.
func (a *annotator) emit(file string, line int, message string) {
	if a.printed == maxAnnotations {
		return
	}
	a.printed++
	if file == "" {
		fmt.Fprintf(a.w, "::error::%s\n", escapeMessage(message))
		return
	}
	fmt.Fprintf(a.w, "::error file=%s,line=%d::%s\n", escapeProperty(file), line, escapeMessage(message))
}

// rooted is the file found in the folder the run runs its commands in, as
// GitHub resolves file=: relative to the repository's top level, with
// forward slashes; not ok when it is outside the repository.
func (a *annotator) rooted(found string) (string, bool) {
	location, _ := a.cfg.VCS.Locate(context.Background(), "")
	abs, _ := filepath.Abs(filepath.Join(a.cfg.Folder(), found))
	return within(location.Top, abs)
}

// within is the path file relative to the folder top, symbolic links
// resolved on both, with forward slashes; not ok when it is not inside.
func within(top, file string) (string, bool) {
	rel, err := filepath.Rel(resolved(top), resolved(file))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func resolved(path string) string {
	if r, err := filepath.EvalSymlinks(path); err == nil {
		return r
	}
	return path
}

// escapeMessage is text as the data of a workflow command.
var escapeMessage = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace

// escapeProperty is text as a property of a workflow command.
var escapeProperty = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C").Replace

// hit is a line of output that names a file and line: the path as written
// (forward slashes), the line and the rest of the line.
type hit struct {
	path   string
	line   int
	reason string
}

// at matches <path>:<line>: with an optional <column>: after it.
var at = regexp.MustCompile(`^(\S+?):([0-9]+):(?:[0-9]+:)?(.*)$`)

// finder reads the lines written to it for the first that begins with a
// relative path of a file in folder and a line number: a writer that keeps
// the first hit and nothing else of what it is given.
type finder struct {
	folder string
	partly []byte
	hit    *hit
}

func (f *finder) reset() { f.partly, f.hit = nil, nil }

func (f *finder) Write(p []byte) (int, error) {
	for _, b := range p {
		if b != '\n' {
			if len(f.partly) < maxLine {
				f.partly = append(f.partly, b)
			}
			continue
		}
		f.flush()
	}
	return len(p), nil
}

// flush reads the line written so far, whole or not.
func (f *finder) flush() {
	line := string(f.partly)
	f.partly = f.partly[:0]
	if f.hit == nil {
		f.hit = f.read(line)
	}
}

func (f *finder) read(line string) *hit {
	m := at.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	n, err := strconv.Atoi(m[2])
	path := filepath.ToSlash(m[1])
	if err != nil || n < 1 || filepath.IsAbs(path) || strings.HasPrefix(path, "/") {
		return nil
	}
	if info, err := os.Stat(filepath.Join(f.folder, filepath.FromSlash(path))); err != nil || info.IsDir() {
		return nil
	}
	return &hit{path: path, line: n, reason: strings.TrimSpace(m[3])}
}
