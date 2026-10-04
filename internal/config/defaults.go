package config

import (
	"path"
	"path/filepath"

	"github.com/donvargax/itos/v2/internal/value"
)

// m is a mapping of the pairs key, value, key, value…, and l a list.
func m(pairs ...any) *value.Map { return value.NewMap(pairs...) }
func l(items ...any) []any      { return items }

// KindKey is where the table holds the defaults of every kind of tests.
const KindKey = "<kind>"

// defaults are the values the tools take when the config leaves a key out:
// the one table (config.ts's DEFAULTS). The loader lays the file over it, and
// `config check --print-defaults` prints it, so a default is applied exactly
// when it is printed; tests.<kind> is laid under each kind the file has. A
// key with no default (ledger.id, a kind's root) is absent when the file
// leaves it out. It is built afresh for each use, so nothing can change it.
func defaults() *value.Map {
	return m(
		"shell", l("sh", "-c"),
		// What the ledger's groups are called in what the tools print, and the
		// flag itos task takes a group by beside --group.
		"ledger", m(
			"group", m("label", "phase", "pattern", "[^/]+", "numeric", false),
			"check", m("timeout", 600.0),
		),
		"commits", m("reject_message", "Commit rejected:"),
		"tests", m(
			KindKey, m(
				"adapter", "gherkin",
				"tag_prefix", "@",
				"wip_tag", "@wip",
				// How a run of several patterns joins them: each in each's {p}, by sep.
				"run", m("join", m("each", "{p}", "sep", "|")),
				"smoke", m("every_file", true),
			),
		),
		"ci", m(
			"wait_on_status", l("todo"),
			"stop_at_first_failure", true,
			"range", m(
				"provider", "github",
				"github", m(
					"workflow", "ci.yml",
					"branch", "main",
					"repository_env", "GITHUB_REPOSITORY",
					"token_env", l("GITHUB_TOKEN", "GH_TOKEN"),
				),
			),
			// Opt in (slice 51): none, so a config without ci.watch pushes as
			// before. A poll every ten seconds is two API requests a run, far
			// inside GitHub's hourly limit; half an hour outlasts a slow run.
			"watch", m(
				"provider", "none",
				"github", m("workflow", "ci.yml"),
				"interval", 10.0,
				"timeout", 1800.0,
			),
		),
		"work", m(
			// Beside the ledger, since it is itos's data as the ledger is, and
			// docs/ is prose: the folder of the config's ledger.files
			// (DefaultsFor); this, with no ledger.
			"registry", "tasks/work-items.yaml",
			// itos ask's questions, beside the registry wherever it is
			// (slice 62, DefaultsFor).
			"asks", "tasks/asks.yaml",
			// itos ask record's decision records, in MADR's own folder
			// (slice 71); a stealth config's are beside it (stealthOnly).
			"decisions", "docs/decisions",
			"groups_key", "phases",
			"statuses", l("todo", "doing", "done", "blocked"),
			"people", m("source", "all-contributors-md", "file", "CONTRIBUTORS.md"),
			"identity", m("provider", "github", "hint", "pass --as <handle>"),
		),
		"hooks", m(
			// How the project calls itos: the wrapper this repository and its
			// template ship.
			"bin", "tools/bin/itos",
			// A static check takes seconds (the cost rule), so a minute leaves
			// room for a slow machine while a commit is never held for minutes;
			// a check that needs longer is late.
			"commit_msg", m("task_checks", true, "check_timeout", 60.0),
		),
		// The repository's own notes, which itos go appends to the
		// coordinator's guide (slice 64): beside its other docs.
		"guide", m("orchestrating", "docs/ORCHESTRATING.md"),
	)
}

// GlobalBin is how a stealth config's hooks call itos: the global launcher,
// on the PATH, since a repository that does not use itos has no wrapper of
// its own to call.
const GlobalBin = "itos"

// DefaultsFor is the table as it applies to a config file (nil for none),
// and whether that file is the stealth config (stealth.go): work.registry
// in the folder of its ledger.files, so a ledger in work/ has its registry
// at work/work-items.yaml, with no file, or no ledger, the table's own
// value; and work.asks beside the registry, the file's or that default, so
// work/asks.yaml there. A stealth config's hooks.bin is itos and it has no work.people
// (stealthOnly). The registry is joined with a slash on every platform, as
// git names the paths it is compared with (bug 9).
func DefaultsFor(file *value.Map, stealth bool) *value.Map {
	table := defaults()
	if files, ok := value.Prop(file.At("ledger"), "files").(string); ok && files != "" {
		table.At("work").(*value.Map).Set("registry", path.Join(path.Dir(filepath.ToSlash(files)), "work-items.yaml"))
	}
	registry, ok := value.Prop(file.At("work"), "registry").(string)
	if !ok || registry == "" {
		registry = table.At("work").(*value.Map).At("registry").(string)
	}
	table.At("work").(*value.Map).Set("asks", path.Join(path.Dir(filepath.ToSlash(registry)), "asks.yaml"))
	if stealth {
		stealthOnly(table)
	}
	return table
}

// stealthOnly sets in a table what a stealth config has whatever its file
// says, the one person's itos in a repository that does not use it:
// hooks.bin is itos, the global launcher, and there is no work.people, so
// no people file is read, the person being the only one; work.decisions is
// decisions, which beside resolves to <git common dir>/itos/decisions.
func stealthOnly(table *value.Map) {
	table.At("hooks").(*value.Map).Set("bin", GlobalBin)
	table.At("work").(*value.Map).Set("decisions", "decisions")
	table.At("work").(*value.Map).Delete("people")
}

// layered is over laid on under: a mapping in both is merged key by key,
// anything else (a list, a value) is over's when over has it, a copy of
// under's when not.
func layered(under, over any) any {
	if over == value.Undefined {
		return value.Copy(under)
	}
	u, uok := under.(*value.Map)
	o, ook := over.(*value.Map)
	if !uok || !ook {
		return over
	}
	merged := value.NewMap()
	for _, k := range u.Keys() {
		merged.Set(k, layered(u.At(k), o.At(k)))
	}
	for _, k := range o.Keys() {
		if !u.Has(k) {
			merged.Set(k, layered(value.Undefined, o.At(k)))
		}
	}
	return merged
}

// withDefaults is the file laid over the defaults, each kind over the
// per-kind ones, and a stealth config's own values over both.
func withDefaults(file *value.Map, stealth bool) *value.Map {
	table := DefaultsFor(file, stealth)
	perKind := table.At("tests").(*value.Map).At(KindKey)
	table.Delete("tests")
	loaded := layered(table, file).(*value.Map)
	if stealth {
		stealthOnly(loaded)
	}
	if kinds, ok := file.At("tests").(*value.Map); ok {
		each := value.NewMap()
		for _, name := range kinds.Keys() {
			each.Set(name, layered(perKind, kinds.At(name)))
		}
		loaded.Set("tests", each)
	}
	return loaded
}
