package cli

// Every built-in command's flags, declared in one place and read by one
// parser (slice 88, decision 36; docs/CLI.md rules 19 to 21 and 25). The
// spec says which flags a command takes, whether each takes a value and
// whether it may be given more than once; readLine applies it before the
// command runs, so a command reads its flags by name from arguments the spec
// has already judged. The help text stays hand-written (help.go): the spec is
// for parsing, not for writing help.

import (
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/git"
	"github.com/donvargax/itos/v7/internal/message"
)

// takes is what a flag takes after its name.
type takes int

const (
	// bare is a switch: no value, and --flag=value is refused.
	bare takes = iota
	// needs is a value, always: --flag value or --flag=value.
	needs
	// may is a value when the next argument is no flag, else none (init
	// --plugin [<scope>], which rule 22 of docs/CLI.md has yet to settle).
	may
)

// flagSpec is one flag a command takes.
type flagSpec struct {
	name   string
	takes  takes
	repeat bool
}

func sw(name string) flagSpec   { return flagSpec{name: name} }
func val(name string) flagSpec  { return flagSpec{name: name, takes: needs} }
func vals(name string) flagSpec { return flagSpec{name: name, takes: needs, repeat: true} }

// spec is what a command, or one of its subcommands, takes.
type spec struct {
	flags []flagSpec
	// subs are its subcommands by name, each read by its own spec when it
	// is the first argument after the command's name.
	subs map[string]spec
	// words is whether a command with subcommands takes words of its own
	// (task's IDs); one that does not refuses a first word that names none
	// of them, and says so before any flag after it is judged.
	words bool
	// more are the flags the config adds, read only when an argument is
	// none of flags: the group label's flag, commit's free-text footers'.
	more func() ([]flagSpec, error)
	// others is whether a flag the spec does not have is the command's to
	// pass on (git commit's own), rather than refused.
	others bool
	// unread is whether, after the first `after` positional arguments,
	// every argument but a global flag is the command's, unread: a program
	// it runs takes them (the smoke runner's, git commit's and git push's
	// through the shim, and itos push, which refuses each itself, saying
	// why a forcing one is refused).
	unread bool
	after  int
}

// joined is a list of flags and more, a new slice, so no spec shares an
// array with another.
func joined(list []flagSpec, more ...flagSpec) []flagSpec {
	return append(slices.Clone(list), more...)
}

// group is the flags that name a ledger's group; groupLabel adds the
// label's own (ledger.group.label).
var group = []flagSpec{val("--group"), val("--phase")}

// groupLabel is the flag of the config's group label, when it is not one
// of group's: --milestone for ledger.group.label milestone.
func groupLabel() ([]flagSpec, error) {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return nil, err
	}
	var flags []flagSpec
	for _, name := range groupFlags(cfg) {
		if _, has := find(group, name); !has {
			flags = append(flags, val(name))
		}
	}
	return flags, nil
}

// footerFlags are commit's flags of the config's free-text footers
// (message.TextFlags), none when there is no config to read: commit runs
// without one, and gitCommit says what is wrong with one that fails.
func footerFlags() ([]flagSpec, error) {
	cfg, _, err := commitConfig()
	if err != nil || cfg == nil {
		return nil, nil
	}
	var flags []flagSpec
	for name := range message.TextFlags(cfg) {
		flags = append(flags, vals(name))
	}
	return flags, nil
}

// specs is every built-in command's spec, by its name.
var specs = map[string]spec{
	"task": {flags: joined(group, val("--skip"), sw("--pending")), more: groupLabel, words: true, subs: map[string]spec{
		"list": {flags: group, more: groupLabel},
		"add": {flags: joined(group, val("--type"), val("--title"), val("--why"), vals("--check"), vals("--timeout")),
			more: groupLabel},
		"next-id": {},
	}},
	"work": {flags: []flagSpec{val("--as")}, subs: map[string]spec{
		"list":    {flags: []flagSpec{sw("--all"), val("--tag")}},
		"show":    {flags: []flagSpec{sw("--patch")}},
		"take":    {flags: []flagSpec{val("--as")}},
		"promote": {flags: []flagSpec{val("--id"), val("--kind"), val("--title")}},
		"done":    {},
		"add": {flags: []flagSpec{val("--title"), val("--why"), val("--kind"), val("--phase"), val("--owner"),
			val("--depends-on"), val("--refs"), val("--tags")}},
		"edit":   {flags: []flagSpec{val("--title"), val("--depends-on"), val("--refs"), val("--tags"), val("--note")}},
		"queue":  {flags: []flagSpec{sw("--top"), sw("--remove"), val("--before"), val("--after")}},
		"drop":   {flags: []flagSpec{val("--why")}},
		"defer":  {flags: []flagSpec{val("--why")}},
		"resume": {},
		"check":  {},
	}},
	"commit": {flags: []flagSpec{vals("--task"), vals("--item"), vals("--scenarios"), vals("--breaking")},
		more: footerFlags, others: true, subs: map[string]spec{
			"check-message": {flags: []flagSpec{val("--at")}},
			"check-paths":   {flags: []flagSpec{val("--type")}},
			"footers":       {},
		}},
	"verify": {},
	"tests": {subs: map[string]spec{
		"list":    {flags: []flagSpec{val("--at")}},
		"moves":   {},
		"next-id": {flags: []flagSpec{val("--count")}},
		"smoke": {subs: map[string]spec{
			"check": {flags: []flagSpec{val("--features")}},
			"ids":   {},
			"run":   {unread: true, after: 1},
		}},
	}},
	"ci": {subs: map[string]spec{
		"plan":  {flags: []flagSpec{sw("--nightly"), sw("--whole"), val("--data-at")}},
		"run":   {flags: []flagSpec{sw("--nightly")}},
		"scope": {},
		"watch": {},
		"range": {flags: []flagSpec{val("--head"), val("--base")}},
	}},
	"hook": {subs: map[string]spec{
		"commit-msg": {},
		"pre-push":   {},
		"install":    {flags: []flagSpec{sw("--print"), sw("--force")}},
	}},
	"guard": {subs: map[string]spec{
		"claude-code": {},
	}},
	"config": {subs: map[string]spec{
		"check": {flags: []flagSpec{sw("--print-defaults"), val("--ledger")}},
		"get":   {},
	}},
	"version": {flags: []flagSpec{sw("--check")}},
	"push":    {unread: true},
	"git-shim": {subs: map[string]spec{
		"install":   {flags: []flagSpec{val("--dir")}},
		"uninstall": {flags: []flagSpec{val("--dir")}},
		"run":       {unread: true},
	}},
	"pin":     {},
	"upgrade": {},
	"init": {flags: []flagSpec{sw("--stealth"), val("--policy"), {name: "--plugin", takes: may}, sw("--git-shim"), sw("--no-git-shim"),
		val("--git-shim-dir"), sw("--agent-rules"), sw("--no-agent-rules")}},
	"followup": {flags: []flagSpec{sw("--all")}, subs: map[string]spec{
		"add":   {flags: []flagSpec{val("--with"), val("--title"), val("--note")}},
		"note":  {},
		"close": {flags: []flagSpec{val("--note")}},
		"show":  {},
		"doc":   {flags: []flagSpec{sw("--force")}},
	}},
	"decision": {flags: []flagSpec{sw("--all")}, subs: map[string]spec{
		"add":    {flags: []flagSpec{val("--item")}},
		"answer": {},
		"record": {flags: []flagSpec{val("--title"), vals("--option"), val("--consequences"), val("--supersedes"),
			sw("--none")}},
		"show": {},
	}},
	"draft": {subs: map[string]spec{
		"add":     {flags: []flagSpec{val("-m"), val("--message")}},
		"edit":    {flags: []flagSpec{val("-m"), val("--message")}},
		"promote": {},
		"drop":    {},
	}},
	"go":     {},
	"guide":  {},
	"status": {flags: []flagSpec{val("--as")}},
}

// flagShaped is an argument that reads as a flag: a dash or two and a name
// with no space in it, then a value after "=" or nothing. "-" (stdin) and
// "--" are no flags, nor is a value that only starts with a dash, such as
// "- a list item".
var flagShaped = regexp.MustCompile(`^--?[^-\s=][^\s=]*(=|$)`)

// commandPath is the command's path in the arguments the global flags have
// left (Globals.Rest, the name first) and its spec: the name, then each
// subcommand while the next argument is one.
func commandPath(rest []string) ([]string, spec) {
	s := specs[rest[0]]
	path := []string{rest[0]}
	for _, word := range rest[1:] {
		sub, ok := s.subs[word]
		if !ok {
			break
		}
		s, path = sub, append(path, word)
	}
	return path, s
}

// readLine reads the arguments of a built-in command by its spec: the
// global flags wherever they stand before a "--", the command's own flags,
// and its other arguments. A flag the spec does not have, a flag with no
// value that needs one (the end, or a flag next), a switch given a value and
// a flag given twice that may be given once are usage errors naming it.
// --flag=value is read as --flag value, and a value is always its flag's,
// never a global flag. It gives the global flags with Rest the command's
// name and its arguments, each flag written "--flag value", so the command
// reads them by name; after a "--", every argument stays as given.
func readLine(args []string) (Globals, error) {
	var g Globals
	read := ParseGlobals(args)
	path, s := commandPath(read.Rest)
	if rest := read.Rest[len(path):]; len(s.subs) > 0 && !s.words && len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		// A subcommand it does not have: the command names those it has,
		// which a usage error for a flag after it would hide.
		return read, nil
	}
	name := strings.Join(path, " ")
	words, positionals := len(path), 0
	seen := map[string]bool{}
	var more []flagSpec
	loaded := false
	// value is the value of the flag at i, said as whose: after its "=",
	// else the next argument when that is no flag, which it takes.
	value := func(i *int, whose, after string, hasValue bool) (string, error) {
		switch {
		case hasValue:
			return after, nil
		case *i+1 < len(args) && !flagShaped.MatchString(args[*i+1]) && args[*i+1] != "--":
			*i++
			return args[*i], nil
		}
		return "", usage("%s needs a value", whose)
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			g.Rest = append(g.Rest, args[i:]...)
			break
		}
		flag, after, hasValue := strings.Cut(arg, "=")
		if !flagShaped.MatchString(arg) {
			flag, hasValue = "", false
		}
		switch {
		case switches[flag] != nil:
			if hasValue {
				return g, usage("%s takes no value", flag)
			}
			switches[flag](&g)
			continue
		case flag == "--config" || flag == "--root":
			v, err := value(&i, flag, after, hasValue)
			if err != nil {
				return g, err
			}
			if flag == "--config" {
				g.Config = v
			} else {
				g.Root = v
			}
			continue
		case words > 0:
			// The command's name and its subcommands, which only global
			// flags stand between.
			g.Rest = append(g.Rest, arg)
			words--
			continue
		case s.unread && positionals >= s.after:
			g.Rest = append(g.Rest, arg)
			continue
		case flag == "":
			g.Rest = append(g.Rest, arg)
			positionals++
			continue
		}
		f, ok := find(s.flags, flag)
		if !ok && s.more != nil {
			if !loaded {
				var err error
				if more, err = s.more(); err != nil {
					return g, err
				}
				loaded = true
			}
			f, ok = find(more, flag)
		}
		switch {
		case !ok && s.others:
			g.Rest = append(g.Rest, arg)
			continue
		case !ok && renamedFlag(name, flag) != nil:
			return g, renamedFlag(name, flag)
		case !ok:
			return g, usage("%s does not take %s; %s", name, flag, takesWhat(joined(s.flags, more...), s.subs))
		case seen[f.name] && !f.repeat:
			return g, usage("%s takes one %s", name, flag)
		}
		seen[f.name] = true
		switch f.takes {
		case bare:
			if hasValue {
				return g, usage("%s %s takes no value", name, flag)
			}
			g.Rest = append(g.Rest, flag)
		case may:
			g.Rest = append(g.Rest, arg)
			if !hasValue && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				g.Rest = append(g.Rest, args[i])
			}
		case needs:
			v, err := value(&i, name+" "+flag, after, hasValue)
			if err != nil {
				return g, err
			}
			g.Rest = append(g.Rest, flag, v)
		}
	}
	return g, nil
}

// takesWhat says which flags a command takes, and its subcommands, as a
// usage error for a flag it does not take names them.
func takesWhat(list []flagSpec, subs map[string]spec) string {
	names := make([]string, len(list))
	for i, f := range list {
		names[i] = f.name
	}
	words := slices.Sorted(maps.Keys(subs))
	switch {
	case len(names) == 0 && len(words) == 0:
		return "it takes no flag"
	case len(names) == 0:
		return "it takes a subcommand: " + either(words)
	case len(words) == 0:
		return "it takes " + either(names)
	}
	return "it takes " + either(names) + ", else a subcommand: " + either(words)
}

// either is a list as a sentence offers it: "a, b or c".
func either(list []string) string {
	if len(list) == 1 {
		return list[0]
	}
	return strings.Join(list[:len(list)-1], ", ") + " or " + list[len(list)-1]
}

// find is the flag of the list by its name, and whether it has one.
func find(list []flagSpec, name string) (flagSpec, bool) {
	i := slices.IndexFunc(list, func(f flagSpec) bool { return f.name == name })
	if i < 0 {
		return flagSpec{}, false
	}
	return list[i], true
}

// objectName is a commit's full name, as a CI provider gives a range's
// start.
var objectName = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// rangeRefs is commitRefs for a range a CI run reads (ci plan, run and
// scope): its start may be a full commit name the repository does not have,
// a history rewritten since the provider saw it, which the plan reads as a
// range it cannot read, and runs everything for (internal/plan).
func rangeRefs(command, from, to string, more ...string) error {
	if objectName.MatchString(from) {
		from = ""
	}
	return commitRefs(command, append([]string{from, to}, more...)...)
}

// commitRefs refuses a ref that names no commit, before command uses it: a
// usage error naming the ref, in a line for people rather than git's
// command line (rule 25 of docs/CLI.md). An empty or all-zero start (a new
// branch, which ranges read as everything up to its end) and git.Unpushed
// name none to check; outside a git repository nothing is checked, and the
// command says what it says there.
func commitRefs(command string, refs ...string) error {
	for _, ref := range refs {
		if ref == git.Unpushed || config.NewBranch(ref) {
			continue
		}
		if git.Succeeds("rev-parse", "--verify", "--quiet", ref+"^{commit}") {
			continue
		}
		if !git.Succeeds("rev-parse", "--git-dir") {
			return nil
		}
		return usage("%s: %s is not a commit of this repository", command, ref)
	}
	return nil
}
