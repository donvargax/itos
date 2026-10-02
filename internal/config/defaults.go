package config

import (
	"path/filepath"

	"github.com/donvargax/itos/internal/value"
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
		),
		"work", m(
			// Beside the ledger, since it is itos's data as the ledger is, and
			// docs/ is prose: the folder of the config's ledger.files
			// (DefaultsFor); this, with no ledger.
			"registry", "tasks/work-items.yaml",
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
	)
}

// DefaultsFor is the table as it applies to a config file (nil for none):
// work.registry in the folder of its ledger.files, so a ledger in work/ has
// its registry at work/work-items.yaml. With no file, or no ledger, the
// table's own value.
func DefaultsFor(file *value.Map) *value.Map {
	table := defaults()
	if files, ok := value.Prop(file.At("ledger"), "files").(string); ok && files != "" {
		table.At("work").(*value.Map).Set("registry", filepath.Join(filepath.Dir(files), "work-items.yaml"))
	}
	return table
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
// per-kind ones.
func withDefaults(file *value.Map) *value.Map {
	table := DefaultsFor(file)
	perKind := table.At("tests").(*value.Map).At(KindKey)
	table.Delete("tests")
	loaded := layered(table, file).(*value.Map)
	if kinds, ok := file.At("tests").(*value.Map); ok {
		each := value.NewMap()
		for _, name := range kinds.Keys() {
			each.Set(name, layered(perKind, kinds.At(name)))
		}
		loaded.Set("tests", each)
	}
	return loaded
}
