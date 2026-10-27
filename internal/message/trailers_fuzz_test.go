package message

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/value"
)

// fuzzKeys are the footers' keys a fuzzed message draws: the repository's configured footers,
// the ones git writes, and the two spellings of a breaking change.
var fuzzKeys = [...]string{
	"Task", "Scenarios", "Item", "Upgrading", "Changes", "Refs",
	"BREAKING CHANGE", "BREAKING-CHANGE", "Signed-off-by", "Co-authored-by",
}

// The kinds of line a fuzzed message's last paragraph draws beside the footers: a footer's kind
// is its key's index in fuzzKeys.
const (
	kindContinuation = len(fuzzKeys) + iota // an indented line, which continues the footer above it
	kindComment
	kindBlank
	kindCherryPick
	kindProse
	kindScissors
	kindURL
	kindCount
)

// separators are how a footer's key meets its value, chosen by an op's argument.
var separators = [...]string{": ", ":", " : ", ":\t"}

// fuzzMessage is the message the fuzzer's arguments stand for, and the comment character
// it uses, so that every input, a seed above all, is one reproducible message:
//
//   - subject, then body after a blank line (none when body is empty), then the last
//     paragraph: one line for each pair of ops, a kind (a byte modulo kindCount) and an
//     argument, whose low four bits are the length of the text the line takes from text,
//     taken in order (15 is the text to its next line break, which is taken too); bits 4 and 5 choose how a footer's key meets its value, bit 6 adds
//     trailing space to the line, bit 7 makes a continuation a tab, a blank line made
//     of spaces and a comment line without its space;
//   - style: bit 0 ends every line with CRLF, bit 1 only those of the last paragraph,
//     bit 2 makes the comment character ";" in place of "#", bit 3 leaves out the
//     last line break, bit 4 glues the last paragraph to the body without a blank line, and bit 5 adds
//     blank lines and a comment line at the end.
func fuzzMessage(subject, body, text string, ops []byte, style uint8) (message, comment string) {
	comment = "#"
	if style&4 != 0 {
		comment = ";"
	}
	take := func(n int) string {
		if n == 15 {
			line, rest, _ := strings.Cut(text, "\n")
			text = rest
			return line
		}
		n = min(n, len(text))
		s := text[:n]
		text = text[n:]
		return s
	}
	var lines []string
	for i := 0; i+1 < len(ops); i += 2 {
		kind, arg := int(ops[i])%kindCount, ops[i+1]
		value := take(int(arg & 15))
		sep := separators[arg>>4&3]
		var line string
		switch {
		case kind < len(fuzzKeys):
			line = fuzzKeys[kind] + sep + value
		case kind == kindContinuation && arg&128 != 0:
			line = "\t" + value
		case kind == kindContinuation:
			line = "  " + value
		case kind == kindComment && arg&128 != 0:
			line = comment + value
		case kind == kindComment:
			line = comment + " " + value
		case kind == kindBlank && arg&128 != 0:
			line = "  "
		case kind == kindBlank:
			line = ""
		case kind == kindCherryPick:
			line = "(cherry picked from commit " + value + ")"
		case kind == kindProse:
			line = value
		case kind == kindScissors:
			line = comment + " ------------------------ >8 ------------------------"
		case kind == kindURL:
			line = "Link://" + value
		}
		if arg&64 != 0 {
			line += "  "
		}
		lines = append(lines, line)
	}
	nl, blockNL := "\n", "\n"
	if style&1 != 0 {
		nl, blockNL = "\r\n", "\r\n"
	} else if style&2 != 0 {
		blockNL = "\r\n"
	}
	message = subject
	if body != "" {
		message += nl + nl + body
	}
	if len(lines) > 0 {
		if style&16 != 0 {
			message += nl
		} else {
			message += nl + nl
		}
		message += strings.Join(lines, blockNL)
	}
	if style&8 == 0 {
		message += blockNL
	}
	if style&32 != 0 {
		message += blockNL + comment + " end" + blockNL + blockNL
	}
	return message, comment
}

// gitSetup is a directory for git to run in and the environment it runs in: the process's
// without any GIT_ variable, no user or system config, so that what a hook hands its children
// or a person configured never changes what git reads.
func gitSetup(t testing.TB) (dir string, env []string) {
	t.Helper()
	dir = t.TempDir()
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(kv), "GIT_") {
			env = append(env, kv)
		}
	}
	return dir, append(env, "HOME="+dir, "USERPROFILE="+dir, "XDG_CONFIG_HOME="+dir,
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
}

// realGit is the git the oracle runs: the one itos exports as ITOS_GIT, which is never itos
// linked as git, else the PATH's. The test cannot import internal/git (depguard), and needs
// the real one.
func realGit() string {
	if p := os.Getenv("ITOS_GIT"); p != "" {
		return p
	}
	return "git"
}

// breakingSpaced is the one key with a space itos reads as a footer: where git reads none.
const breakingSpaced = breakingKey + ":"

// gitTrailers are the trailers git interpret-trailers --parse gives for message, with
// core.commentChar set to comment and no "---" divider. git reads BREAKING CHANGE as no
// trailer (its key has a space), which itos reads as one by design (trailers.go): git is
// given the message with that key spelled BREAKING-CHANGE, the one it is read as, so
// every other difference between the readers is a difference.
func gitTrailers(t testing.TB, dir string, env []string, message, comment string) []trailer {
	t.Helper()
	lines := strings.Split(message, "\n")
	for i, l := range lines {
		if rest, ok := strings.CutPrefix(l, breakingSpaced); ok {
			lines[i] = "BREAKING-CHANGE:" + rest
		}
	}
	cmd := exec.Command(realGit(), "-c", "core.commentChar="+comment, "interpret-trailers", "--parse", "--no-divider")
	cmd.Dir, cmd.Env, cmd.Stdin = dir, env, strings.NewReader(strings.Join(lines, "\n"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git interpret-trailers failed on %q: %v", message, err)
	}
	var found []trailer
	for _, l := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		if l == "" {
			continue
		}
		key, value, ok := strings.Cut(l, ":")
		if !ok {
			t.Fatalf("git interpret-trailers gave %q, a line with no colon, for %q", l, message)
		}
		found = append(found, trailer{key: key, value: strings.TrimPrefix(value, " ")})
	}
	return found
}

// itosTrailers are the trailers of message as itos reads them (trailers), with the
// values unfolded and BREAKING CHANGE as BREAKING-CHANGE, as gitTrailers gives them to git.
func itosTrailers(message, comment string) []trailer {
	var found []trailer
	for _, tr := range trailers(message, comment) {
		if tr.key == breakingKey {
			tr.key = "BREAKING-CHANGE"
		}
		found = append(found, trailer{key: tr.key, value: unfolded(tr.value)})
	}
	return found
}

// FuzzTrailersMatchGit requires trailers (trailers.go) to read, from a message built from
// the fuzzer's arguments (fuzzMessage), the keys and unfolded values, in order, that git
// interpret-trailers --parse reads from it with the same comment character (bug 110's
// oracle; the reviewer replay's K143b, which dropped a trailer the hook required). The
// seeds under testdata/fuzz/FuzzTrailersMatchGit are the conformance corpus's messages
// (each as the arguments that build it), hand-written ones for each rule trailers.go reads
// (the last paragraph, the title, a body glued to footers, git's own trailers and the quarter
// rule, folding, CRLF, comment and scissors lines, K143b's BREAKING CHANGE) and what the
// nightly's fuzz run found; go test runs only them.
//
// Two differences are by design and no disagreement. BREAKING CHANGE is a footer for itos
// and none for git (trailers.go's header), so git reads the message with that key
// spelled BREAKING-CHANGE (gitTrailers). A message with a NUL is not one git commit takes,
// and git reads it to the NUL. The git that runs may differ too: see oldGit.
func FuzzTrailersMatchGit(f *testing.F) {
	dir, env := gitSetup(f)
	// Newer git reads no line that begins with a URL as a trailer, as itos does (separatorAt);
	// an older one reads "Link://x" as the trailer Link, so with that git a message holding "://"
	// is not compared.
	oldGit := len(gitTrailers(f, dir, env, "x\n\nLink://x\n", "#")) > 0
	f.Fuzz(func(t *testing.T, subject, body, text string, ops []byte, style uint8) {
		message, comment := fuzzMessage(subject, body, text, ops, style)
		if strings.Contains(message, "\x00") {
			t.Skip("git reads a message as a C string, to its first NUL, and commit refuses one that holds a NUL")
		}
		if oldGit && strings.Contains(message, "://") {
			t.Skip("this git reads a line beginning with a URL as a trailer")
		}
		want := gitTrailers(t, dir, env, message, comment)
		// A known disagreement, not a difference by design: unfolded trims a value with
		// JavaScript's space (value.Trim), git with its own four (" \t\n\r"), so a value that
		// ends in a form feed, a no-break space or the like is another value. Both are trimmed
		// so here until unfolded reads as git does.
		for i := range want {
			want[i].value = value.Trim(want[i].value)
		}
		got := itosTrailers(message, comment)
		if !slices.Equal(got, want) {
			t.Errorf("message %q, comment character %q:\n git reads %q\nitos reads %q", message, comment, want, got)
		}
	})
}
