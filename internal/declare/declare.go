package declare

// Command is one built-in command's declaration. tools/bin/assemble reads
// Name and Rank from the source, so both are literals.
type Command struct {
	// Name is the word that runs it.
	Name string
	// Rank is its place in itos --help and in the command table, unique
	// among the commands.
	Rank int
	// Lines are its lines in itos --help's Commands list, verbatim, each
	// ending in a newline.
	Lines string
	// Help is its --help pages by subpath: "" its own, "list" itos <name>
	// list's, "smoke run" itos <name> smoke run's.
	Help map[string]string
	// Spec is what it takes.
	Spec Spec
}

// Takes is what a flag takes after its name.
type Takes int

const (
	// Bare is a switch: no value, and --flag=value is refused.
	Bare Takes = iota
	// Needs is a value, always: --flag value or --flag=value.
	Needs
	// May is a value when the next argument is no flag, else none (init
	// --plugin [<scope>], which rule 22 of docs/CLI.md has yet to settle).
	May
)

// Flag is one flag a command takes.
type Flag struct {
	Name  string
	Takes Takes
	// Repeat is whether it may be given more than once.
	Repeat bool
}

// Switch is a flag that takes no value.
func Switch(name string) Flag { return Flag{Name: name} }

// Value is a flag that takes a value, given once.
func Value(name string) Flag { return Flag{Name: name, Takes: Needs} }

// Values is a flag that takes a value, given any number of times.
func Values(name string) Flag { return Flag{Name: name, Takes: Needs, Repeat: true} }

// Optional is a flag that may take a value, given once.
func Optional(name string) Flag { return Flag{Name: name, Takes: May} }

// More names the flags the config adds to a spec, which the parser reads
// the config for only when an argument is none of the spec's own: a
// declaration reads nothing.
type More int

const (
	// NoMore adds none.
	NoMore More = iota
	// GroupLabel adds the flag of the config's group label when it is not
	// one of the spec's (--milestone for ledger.group.label milestone).
	GroupLabel
	// Footers adds a flag for each of the config's free-text footers,
	// itos commit's.
	Footers
)

// Spec is what a command, or one of its subcommands, takes.
type Spec struct {
	Flags []Flag
	// Subs are its subcommands by name, each read by its own spec when it
	// is the first argument after the command's name.
	Subs map[string]Spec
	// Words is whether a command with subcommands takes words of its own
	// (task's IDs); one that does not refuses a first word that names none
	// of them, and says so before any flag after it is judged.
	Words bool
	// Others is whether a flag the spec does not have is the command's to
	// pass on (git commit's own), rather than refused.
	Others bool
	// Unread is whether, after the first After positional arguments, every
	// argument but a global flag is the command's, unread: a program it
	// runs takes them (the smoke runner's, git commit's and git push's
	// through the shim, and itos push, which refuses each itself, saying
	// why a forcing one is refused).
	Unread bool
	After  int
	// More are the flags the config adds.
	More More
}
