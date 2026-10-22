package cli

// Every built-in command's flags, declared in one place and read by one
// parser (slice 88, decision 36; docs/CLI.md rules 19 to 21 and 25). The
// spec (internal/declare's Spec) says which flags a command takes, whether
// each takes a value and whether it may be given more than once, and which
// the config adds; readLine applies it before the command runs, so a command
// reads its flags by name from arguments the spec has already judged. The
// help text stays hand-written (help.go): the spec is for parsing, not for
// writing help.

import (
	"context"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/declare"
	"github.com/donvargax/itos/v8/internal/message"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// A spec and its flags are what a command declares (internal/declare); the
// parser is this file's.
type (
	spec     = declare.Spec
	flagSpec = declare.Flag
)

var (
	sw   = declare.Switch
	val  = declare.Value
	vals = declare.Values
)

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
	cfg, err := loadConfig()
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

// moreFlags are the flags the config adds to a spec, by what its More
// names: groupLabel's, footerFlags' or none.
func moreFlags(more declare.More) ([]flagSpec, error) {
	switch more {
	case declare.GroupLabel:
		return groupLabel()
	case declare.Footers:
		return footerFlags()
	}
	return nil, nil
}

// specs is every built-in command's spec, by its name.
var specs = map[string]spec{
	"task": {Flags: joined(group, val("--skip"), sw("--pending")), More: declare.GroupLabel, Words: true, Subs: map[string]spec{
		"list": {Flags: group, More: declare.GroupLabel},
		"add": {Flags: joined(group, val("--type"), val("--title"), val("--why"), vals("--check"), vals("--timeout")),
			More: declare.GroupLabel},
		"next-id": {},
	}},
	"work": {Flags: []flagSpec{val("--as")}, Subs: map[string]spec{
		"list": {Flags: []flagSpec{sw("--all"), val("--tag")}},
		"show": {Flags: []flagSpec{sw("--patch")}},
		"take": {Flags: []flagSpec{val("--as")}},
		"promote": {Flags: []flagSpec{val("--id"), val("--kind"), val("--title"), val("--why"), val("--type"),
			vals("--check"), vals("--timeout")}},
		"done": {},
		"add": {Flags: []flagSpec{val("--title"), val("--why"), val("--kind"), val("--phase"), val("--owner"),
			val("--depends-on"), val("--refs"), val("--tags")}},
		"edit":      {Flags: []flagSpec{val("--title"), val("--depends-on"), val("--refs"), val("--tags"), val("--note")}},
		"queue":     {Flags: []flagSpec{sw("--top"), sw("--remove"), val("--before"), val("--after")}},
		"drop":      {Flags: []flagSpec{val("--why")}},
		"defer":     {Flags: []flagSpec{val("--why")}},
		"resume":    {},
		"give-back": {Flags: []flagSpec{val("--why"), val("--as")}},
		"check":     {},
	}},
	"commit": {Flags: []flagSpec{vals("--task"), vals("--item"), vals("--scenarios"), vals("--breaking")},
		More: declare.Footers, Others: true, Subs: map[string]spec{
			"check-message": {Flags: []flagSpec{val("--at")}},
			"check-paths":   {Flags: []flagSpec{val("--type")}},
			"footers":       {},
		}},
	"verify": {},
	"tests": {Subs: map[string]spec{
		"list":    {Flags: []flagSpec{val("--at")}},
		"moves":   {},
		"next-id": {Flags: []flagSpec{val("--count")}},
		"smoke": {Subs: map[string]spec{
			"check": {Flags: []flagSpec{val("--features")}},
			"ids":   {},
			"run":   {Unread: true, After: 1},
		}},
	}},
	"ci": {Subs: map[string]spec{
		"plan":  {Flags: []flagSpec{sw("--nightly"), sw("--whole"), val("--data-at")}},
		"run":   {Flags: []flagSpec{sw("--nightly"), sw("--tests")}},
		"scope": {},
		"watch": {},
		"range": {Flags: []flagSpec{val("--head"), val("--base")}},
	}},
	"hook": {Subs: map[string]spec{
		"commit-msg": {},
		"pre-push":   {},
		"install":    {Flags: []flagSpec{sw("--print"), sw("--force")}},
	}},
	"guard": {Subs: map[string]spec{
		"claude-code": {},
	}},
	"config": {Subs: map[string]spec{
		"check": {Flags: []flagSpec{sw("--print-defaults"), val("--ledger")}},
		"get":   {},
	}},
	"version": {Flags: []flagSpec{sw("--check")}},
	"push":    {Unread: true},
	"git-shim": {Subs: map[string]spec{
		"install":   {Flags: []flagSpec{val("--dir")}},
		"uninstall": {Flags: []flagSpec{val("--dir")}},
		"run":       {Unread: true},
	}},
	"pin":     {},
	"upgrade": {},
	"init": {Flags: []flagSpec{sw("--stealth"), val("--policy"), declare.Optional("--plugin"), sw("--git-shim"), sw("--no-git-shim"),
		val("--git-shim-dir"), sw("--agent-rules"), sw("--no-agent-rules")}},
	"followup": {Flags: []flagSpec{sw("--all")}, Subs: map[string]spec{
		"add":   {Flags: []flagSpec{val("--with"), val("--title"), val("--note")}},
		"note":  {},
		"close": {Flags: []flagSpec{val("--note")}},
		"show":  {},
		"doc":   {Flags: []flagSpec{sw("--force")}},
	}},
	"decision": {Flags: []flagSpec{sw("--all")}, Subs: map[string]spec{
		"add":    {Flags: []flagSpec{val("--item")}},
		"answer": {},
		"record": {Flags: []flagSpec{val("--title"), vals("--option"), val("--consequences"), val("--supersedes"),
			sw("--none")}},
		"show": {},
	}},
	"draft": {Subs: map[string]spec{
		"add":     {Flags: []flagSpec{val("-m"), val("--message")}},
		"edit":    {Flags: []flagSpec{val("-m"), val("--message")}},
		"promote": {},
		"drop":    {},
	}},
	"go":     {},
	"guide":  {},
	"status": {Flags: []flagSpec{val("--as")}},
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
		sub, ok := s.Subs[word]
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
	if rest := read.Rest[len(path):]; len(s.Subs) > 0 && !s.Words && len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
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
		case s.Unread && positionals >= s.After:
			g.Rest = append(g.Rest, arg)
			continue
		case flag == "":
			g.Rest = append(g.Rest, arg)
			positionals++
			continue
		}
		f, ok := find(s.Flags, flag)
		if !ok && s.More != declare.NoMore {
			if !loaded {
				var err error
				if more, err = moreFlags(s.More); err != nil {
					return g, err
				}
				loaded = true
			}
			f, ok = find(more, flag)
		}
		switch {
		case !ok && s.Others:
			g.Rest = append(g.Rest, arg)
			continue
		case !ok && renamedFlag(name, flag) != nil:
			return g, renamedFlag(name, flag)
		case !ok:
			return g, usage("%s does not take %s; %s", name, flag, takesWhat(joined(s.Flags, more...), s.Subs))
		case seen[f.Name] && !f.Repeat:
			return g, usage("%s takes one %s", name, flag)
		}
		seen[f.Name] = true
		switch f.Takes {
		case declare.Bare:
			if hasValue {
				return g, usage("%s %s takes no value", name, flag)
			}
			g.Rest = append(g.Rest, flag)
		case declare.May:
			g.Rest = append(g.Rest, arg)
			if !hasValue && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				g.Rest = append(g.Rest, args[i])
			}
		case declare.Needs:
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
		names[i] = f.Name
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
	i := slices.IndexFunc(list, func(f flagSpec) bool { return f.Name == name })
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
// branch, which ranges read as everything up to its end) and vcsport.Unpushed
// name none to check; outside a git repository nothing is checked, and the
// command says what it says there.
func commitRefs(command string, refs ...string) error {
	ctx, v := context.Background(), vcs.Git{}
	for _, ref := range refs {
		if ref == vcsport.Unpushed || config.NewBranch(ref) {
			continue
		}
		if _, err := v.Resolve(ctx, ref); err == nil {
			continue
		}
		if _, err := v.Locate(ctx, ""); err != nil {
			return nil
		}
		return usage("%s: %s is not a commit of this repository", command, ref)
	}
	return nil
}
