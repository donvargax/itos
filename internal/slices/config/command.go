package config

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "config",
	Rank: 200,
	Lines: `  config check [--print-defaults]  validate the config, ledger, registry and smoke sets
  config get <key>                 a config key's value as itos reads it, defaults applied
`,
	Help: map[string]string{
		"": `Usage: itos config check [--ledger <file>] [--print-defaults]
       itos config get <key>`,

		"check": `Usage: itos config check [--ledger <file>] [--print-defaults]

Validates the config, then the ledger (or one ledger file), the work registry,
the questions (work.asks), each kind's smoke set and the decision records in
the folder work.decisions names: no two share a number, and a record whose
status is "superseded by ADR-NNNN" names a number a record there has. Exit 2
when the config is invalid, as for every command (among the rest, a proof.code
that names no paths or whose check does not take {base}, a shell with no
program, a number not finite, a ledger.check.timeout of 0 or less), 1 for any
other problem, among them a check's timeout of 0 or less, an except_types type
commits.types does not list, a wip_tag on a kind whose adapter is a command, a
ledger.group.label that is no word itos task can take as its own flag, a
ledger.group.pattern with anchors of its own, {group} in ledger.files' folder,
a ledger.files that matches work.registry, and an open question naming an item
the registry does not have. A warning (WARN) never fails it: a
check static by a ci.cost.static pattern written below a late check of its task,
which ci.cost.keep_written_order runs late, a project's people file missing
or unreadable (a stealth config reads none), and an answered question naming an
item the registry does not have. --print-defaults prints the values
the tools take when a key is left out (work.registry's in this config's ledger
folder; a stealth config's hooks.bin, itos, and no work.people), and checks
nothing.

--json: {"schema":1,"config","valid","problems":[{"rule","message","fix"?,"area"}],"warnings"?}`,

		"get": `Usage: itos config get <key>

Prints the value of a config key, a dotted path (hooks.bin,
hooks.commit_msg.check_timeout, tests.scenario.root), as itos reads it: the
config's own, else its default, a stealth config's hooks.bin itos whatever it
says, from the config every command finds. A string, a number or a boolean
prints bare on one line, a mapping or a list as JSON on one line, and a key
with no value and no default prints nothing. A path is relative to the
repository's top, the files a stealth config names for itos's own data
resolved beside it, where itos reads them. A key the config does not have is
a usage error, exit 2; a config that cannot be read exits 2, as for every
command.

--json: {"schema":1,"key","value"}, value null for a key with no value`,
	},
	Spec: declare.Spec{Subs: map[string]declare.Spec{
		"check": {Flags: []declare.Flag{declare.Switch("--print-defaults"), declare.Value("--ledger")}},
		"get":   {},
	}},
}
