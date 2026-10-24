package agentrules

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/vcs/port"
)

const (
	// AgentsFile is the file agents read their instructions from.
	AgentsFile = "AGENTS.md"
	// ClaudeFile is Claude Code's, which reads AGENTS.md through ClaudeImport.
	ClaudeFile   = "CLAUDE.md"
	ClaudeImport = "@AGENTS.md"
	// Begin and End are the lines the block is written between.
	Begin = "<!-- itos:begin -->"
	End   = "<!-- itos:end -->"
)

// Block is the block, its markers included, LF: the staging boundary
// under a heading, the same in every repository.
const Block = Begin + "\n\n" +
	"## itos\n\n" +
	"Stage only your own files, by explicit path: untracked files you did not make are someone else's work in progress.\n\n" +
	End + "\n"

// StaleBlock is whether a block found (nil for none) is other than the one
// this itos writes: an older itos's, say, or one edited by hand.
func StaleBlock(found *string) bool { return found != nil && *found != Block }

// StaleLine is what init's rerun says of a stale block, and itos status
// with it: AGENTS.md's, or under a stealth config that of the files init
// writes there.
func StaleLine(stealth bool) string {
	whose := AgentsFile + "'s itos rules are"
	if stealth {
		whose = "The itos rules for agents are"
	}
	return whose + " not the ones this itos writes: itos init --agent-rules rewrites them, only between the markers."
}

// Stale are the files holding a block of itos's rules for agents other
// than the one this itos writes, as init's rerun decides it without
// --agent-rules: AGENTS.md, or under a stealth config the copies init
// writes there, v locating them. It writes nothing and never fails: no
// block is none (the rules are offered, never required), and a file that
// cannot be read, or whose markers cannot be placed, is none too, left for
// init to report.
func Stale(ctx context.Context, v port.VCS, stealth bool) []string {
	if stealth {
		s, err := ReadStealth(ctx, v)
		if err != nil {
			return nil
		}
		return s.StaleBlocks()
	}
	text, _, err := ReadText(port.Within(ctx, v, AgentsFile))
	if err != nil {
		return nil
	}
	if _, found, _, err := Split(text); err != nil || !StaleBlock(found) {
		return nil
	}
	return []string{AgentsFile}
}

// Edit is a file whose rules block upgrade rewrites: its path as said, its
// path on disk, read from the folder itos runs in, its new text (LF) and
// whether it used CRLF.
type Edit struct {
	Path, Disk, Text string
	CRLF             bool
}

// Write writes the edit, keeping the file's line endings.
func (e Edit) Write() error { return WriteText(e.Disk, e.Text, e.CRLF) }

// Refreshed are the files Stale finds, each with its block rewritten in
// this itos's words, as itos init --agent-rules rewrites it, only between
// the markers: AGENTS.md, or under a stealth config the copies init writes
// there, v locating them. It writes nothing. A file with no block gains
// none, since the rules are offered, never required, and one that cannot
// be read, or whose markers cannot be placed, is left as it is, for init to
// report.
func Refreshed(ctx context.Context, v port.VCS, stealth bool) []Edit {
	if stealth {
		s, err := ReadStealth(ctx, v)
		if err != nil {
			return nil
		}
		return s.refreshed()
	}
	disk := port.Within(ctx, v, AgentsFile)
	text, crlf, err := ReadText(disk)
	if err != nil {
		return nil
	}
	before, found, after, err := Split(text)
	if err != nil || !StaleBlock(found) {
		return nil
	}
	return []Edit{{AgentsFile, disk, before + Block + after, crlf}}
}

// ReadText is a file's text with its line endings made LF, whether it used
// CRLF, and "" for a file that is not there.
func ReadText(p string) (string, bool, error) {
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	text := string(data)
	crlf := strings.Contains(text, "\r\n")
	return strings.ReplaceAll(text, "\r\n", "\n"), crlf, nil
}

// WriteText writes LF text, with CRLF line endings when crlf.
func WriteText(p, text string, crlf bool) error {
	if crlf {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	return os.WriteFile(p, []byte(text), 0o666)
}

// exists is whether a file is there.
func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Split is AGENTS.md's text (LF) cut around its block: the text before the
// begin marker's line, the block from that line through the end marker's
// (nil when the file has neither marker), and the text after it. A file
// with one marker and not the other, or more than one block, is an error:
// which text is itos's would be a guess.
func Split(text string) (string, *string, string, error) {
	return splitBlockOf(text, AgentsFile, Begin, End)
}

// splitBlockOf is Split for the file name's text and a block between the
// lines begin and end.
func splitBlockOf(text, name, begin, end string) (string, *string, string, error) {
	lines := strings.SplitAfter(text, "\n")
	at, err := blockSpan(lines, name, begin, end, noSpan)
	if err != nil || at == noSpan {
		return text, nil, "", err
	}
	block := strings.Join(lines[at[0]:at[1]+1], "")
	if !strings.HasSuffix(block, "\n") {
		block += "\n"
	}
	return strings.Join(lines[:at[0]], ""), &block, strings.Join(lines[at[1]+1:], ""), nil
}

// noSpan is the span of no lines.
var noSpan = [2]int{-1, -1}

// blockSpan is the indexes of the lines begin and end that mark a block in
// the file name's lines, its lines from skip[0] through skip[1] not looked
// at (another block's, say); noSpan when it has neither marker. One marker
// and not the other, or more than one block, is an error.
func blockSpan(lines []string, name, begin, end string, skip [2]int) ([2]int, error) {
	find := func(marker string, from int) int {
		for i := from; i < len(lines); i++ {
			if (i < skip[0] || i > skip[1]) && strings.TrimSpace(lines[i]) == marker {
				return i
			}
		}
		return -1
	}
	b, e := find(begin, 0), find(end, 0)
	switch {
	case b < 0 && e < 0:
		return noSpan, nil
	case e < 0:
		return noSpan, refusedMarkers("%s has a %s line with no %s line after it; put the markers right by hand",
			name, begin, end)
	case b < 0 || e < b:
		return noSpan, refusedMarkers("%s has a %s line with no %s line before it; put the markers right by hand",
			name, end, begin)
	}
	if find(begin, e+1) >= 0 || find(end, e+1) >= 0 {
		return noSpan, refusedMarkers("%s has more than one block between %s and %s; leave one", name, begin, end)
	}
	return [2]int{b, e}, nil
}

// refusedMarkers is a marked block itos cannot place in a file it does not
// own, which the person puts right: kind.Usage (exit 2).
func refusedMarkers(format string, a ...any) error {
	return kind.Wrap(kind.Usage, fmt.Errorf(format, a...))
}

// AppendBlock is text with the block at its end, after a blank line.
func AppendBlock(text, block string) string {
	if text == "" {
		return block
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if strings.TrimSpace(text[strings.LastIndex(strings.TrimSuffix(text, "\n"), "\n")+1:]) != "" {
		text += "\n"
	}
	return text + block
}
