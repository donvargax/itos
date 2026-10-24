package draft

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
)

// mintedKind is the kind the line's command would make, as that command
// reads its arguments: none for a line that asks for help, has no command,
// mints nothing or is refused.
func TestMintedKind(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		name string
		set  func(*fake)
		want string
	}{
		{"help", func(f *fake) { f.line = Line{Help: true, Rest: []string{"work", "add"}} }, ""},
		{"one word", func(f *fake) { f.line.Rest = []string{"work"} }, ""},
		{"no minting line", func(f *fake) { f.line.Rest = []string{"work", "queue"} }, ""},
		{"a line the table refuses", func(f *fake) { f.readErr = errBroken }, ""},
		{"work add, exactly", func(*fake) {}, "slice"},
		{"an idea", func(f *fake) { f.addKind = "idea" }, ""},
		{"work add refused", func(f *fake) { f.addErr = errBroken }, ""},
		{"work promote", func(f *fake) { f.line.Rest = []string{"work", "promote"}; f.promoteKind = "task" }, "task"},
		{"work promote refused", func(f *fake) { f.line.Rest = []string{"work", "promote"}; f.promoteErr = errBroken }, ""},
		{"task add", func(f *fake) { f.line.Rest = []string{"task", "add"} }, "task"},
		{"task add refused", func(f *fake) { f.line.Rest = []string{"task", "add"}; f.taskErr = errBroken }, ""},
		{"task add with no config", func(f *fake) { f.line.Rest = []string{"task", "add"}; f.cfgErr = errBroken }, ""},
	} {
		f := &fake{line: Line{Rest: []string{"work", "add"}}, read: []string{"work", "add"}, addKind: "slice"}
		c.set(f)
		if got := mintedKind(ctx, f, []string{"the", "line"}); got != c.want {
			t.Errorf("%s: mintedKind = %q, want %q", c.name, got, c.want)
		}
	}
}

// mint mints the id under the line's own global flags, through the Minting
// of the folder they name, putting back what they changed in the
// environment; a line they refuse, or one that mints nothing, mints none.
func TestMint(t *testing.T) {
	ctx := context.Background()
	t.Setenv("ITOS_CONFIG", "kept")
	t.Setenv("NO_COLOR", "")
	os.Unsetenv("NO_COLOR")
	released := 0
	f := &fake{line: Line{Rest: []string{"work", "add"}}, read: []string{"work", "add"}, addKind: "slice", minted: "slice-5",
		sound: Sound{Cfg: &config.Loaded{}, Release: func() { released++ }}}
	h := NewHandler(f)
	minted, code, err := h.mint(ctx, nil)
	if minted != "slice-5" || code != 0 || err != nil || released != 1 || strings.Join(f.mintedFor, " ") != "slice" {
		t.Errorf("mint = %q, %d, %v; released %d, minted for %q", minted, code, err, released, f.mintedFor)
	}
	// The line's flags name another folder and set the variables; mint
	// mints there and puts the variables back.
	there := &fake{line: f.line, read: f.read, addKind: "slice", minted: "slice-6", sound: f.sound}
	moved := &moving{fake: f, there: there}
	if minted, _, err := NewHandler(moved).mint(ctx, nil); minted != "slice-6" || err != nil {
		t.Errorf("mint after moving = %q, %v", minted, err)
	}
	if os.Getenv("ITOS_CONFIG") != "kept" {
		t.Errorf("mint left ITOS_CONFIG %q", os.Getenv("ITOS_CONFIG"))
	}
	if _, set := os.LookupEnv("NO_COLOR"); set {
		t.Error("mint left NO_COLOR set")
	}
	f.mintErr = errBroken
	if _, _, err := h.mint(ctx, nil); !errors.Is(err, errBroken) {
		t.Errorf("a mint that fails: %v", err)
	}
	f.sound = Sound{Code: refused}
	if minted, code, err := h.mint(ctx, nil); minted != "" || code != refused || err != nil {
		t.Errorf("an unsound registry: %q, %d, %v", minted, code, err)
	}
	f.addKind = "idea"
	if minted, code, err := h.mint(ctx, nil); minted != "" || code != 0 || err != nil {
		t.Errorf("an idea: %q, %d, %v", minted, code, err)
	}
	f.globalsErr = errBroken
	if minted, code, err := h.mint(ctx, nil); minted != "" || code != 0 || err != nil {
		t.Errorf("flags refused: %q, %d, %v", minted, code, err)
	}
}

// moving is the fake whose ApplyGlobals names the folder there runs in and
// sets the variables, as --root, --config and --no-color do.
type moving struct {
	*fake
	there *fake
}

func (m *moving) ApplyGlobals(context.Context, []string) (Minting, error) {
	os.Setenv("ITOS_CONFIG", "other")
	os.Setenv("NO_COLOR", "1")
	return m.there, nil
}

// runReserved runs the line in this process with the id reserved, and
// gives its exit code.
func TestRunReserved(t *testing.T) {
	f := &fake{runCode: 4}
	var out strings.Builder
	if code := NewHandler(f).runReserved(context.Background(), makeIt, &out); code != 4 || out.String() != "ran here\n" ||
		strings.Join(f.reserved, "") != "slice-5" {
		t.Errorf("runReserved: exit %d, output %q, reserved %q", code, out.String(), f.reserved)
	}
}

// inPlace gives f's error, the environment put back.
func TestInPlaceGivesFsError(t *testing.T) {
	t.Setenv("NO_COLOR", "kept")
	if err := inPlace(func() error { os.Setenv("NO_COLOR", "1"); return os.ErrInvalid }); err != os.ErrInvalid {
		t.Errorf("inPlace gave %v, not f's error", err)
	}
	if os.Getenv("NO_COLOR") != "kept" {
		t.Errorf("NO_COLOR is %q, not put back", os.Getenv("NO_COLOR"))
	}
}

// A lock that cannot be given back is said after what a change printed.
func TestTheLockSaidLast(t *testing.T) {
	f, _, file := repo(t)
	keep(t, file, check)
	f.releaseErr = errBroken
	code, stdout, stderr, err := draft(t, f, false, "promote")
	if code != 0 || err != nil || stdout != "check promoted: itos work check (no commit)\n" ||
		stderr != "ran\nitos: the lock cannot be given back: broken\n" {
		t.Errorf("promote: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
}
