package cli

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// readLine reads a built-in command's line by its spec: --flag=value as
// --flag value, the global flags wherever they stand, a value its flag's,
// and everything after a "--" as given.
func TestReadLine(t *testing.T) {
	for line, want := range map[string]string{
		"task list --group=9":                             "task list --group 9",
		"--json task list --group 9 -q":                   "task list --group 9",
		"task list --root=. --group 9":                    "task list --group 9",
		"work add x --why - --title=-a":                   "work add x --why - --title -a",
		"decision record q-1 --consequences '- a' --none": "decision record q-1 --consequences '- a' --none",
		"decision record q-1 --option a --option b":       "decision record q-1 --option a --option b",
		"ci range --head h --base ''":                     "ci range --head h --base ''",
		"commit check-message -":                          "commit check-message -",
		"tests smoke run scenario --workers=1 -q":         "tests smoke run scenario --workers=1",
		"tests smoke run scenario -- --json":              "tests smoke run scenario -- --json",
		"commit --task T-1 -m x --amend":                  "commit --task T-1 -m x --amend",
		"push --force":                                    "push --force",
		"init --plugin --stealth":                         "init --plugin --stealth",
		"init --plugin=user":                              "init --plugin=user",
		"work bogus --x":                                  "work bogus --x",
		"ci bogus --x":                                    "ci bogus --x",
	} {
		g, err := readLine(fields(line))
		if err != nil {
			t.Errorf("itos %s: %v", line, err)
			continue
		}
		if got := strings.Join(quoted(g.Rest), " "); got != want {
			t.Errorf("itos %s: %s, not %s", line, got, want)
		}
	}
}

// A flag the spec does not have, a flag with no value, a switch given one
// and a flag given twice are usage errors naming it.
func TestReadLineRefuses(t *testing.T) {
	for line, want := range map[string]string{
		"work list --bogus":          "work list does not take --bogus; it takes --all or --tag",
		"task list --group --json":   "task list --group needs a value",
		"task list --group":          "task list --group needs a value",
		"task list --group -- 1":     "task list --group needs a value",
		"work list --all=yes":        "work list --all takes no value",
		"work take x --as a --as b":  "work take takes one --as",
		"--json=1 version":           "--json takes no value",
		"version --root":             "--root needs a value",
		"version --config --json":    "--config needs a value",
		"ci plan HEAD HEAD --nightl": "ci plan does not take --nightl; it takes --nightly, --whole or --data-at",
		"ci --x":                     "ci does not take --x; it takes a subcommand: plan, range, run, scope or watch",
		"verify -x HEAD":             "verify does not take -x; it takes no flag",
	} {
		_, err := readLine(fields(line))
		if err == nil || ExitCode(err) != ExitUsage || err.Error() != want {
			t.Errorf("itos %s: %v, not %s", line, err, want)
		}
	}
}

// Every flag a command's usage lines name is in its spec: the help is
// written by hand, and a flag it documents that the spec does not have
// would be refused.
func TestUsageFlagsAreInTheSpecs(t *testing.T) {
	flag := regexp.MustCompile(`--[a-z][a-z-]*`)
	for key, text := range helpTexts {
		if key == "" {
			continue
		}
		usageBlock, _, _ := strings.Cut(text, "\n\n")
		var s spec
		var path []string
		for _, line := range strings.Split(usageBlock, "\n") {
			if _, cmd, ok := strings.Cut(line, "itos "); ok && !strings.HasPrefix(strings.TrimSpace(line), "-") {
				var words []string
				for _, w := range strings.Fields(cmd) {
					if strings.ContainsAny(w[:1], "-<[") || strings.Contains(w, "|") {
						break
					}
					words = append(words, w)
				}
				if len(words) == 0 {
					// itos --version is itos version (launch.Args).
					words = []string{"version"}
				}
				if _, known := specs[words[0]]; !known {
					t.Errorf("%s: no spec for itos %s", key, words[0])
					continue
				}
				path, s = commandPath(words)
			}
			if s.others || s.unread {
				continue
			}
			for _, f := range flag.FindAllString(line, -1) {
				if _, ok := find(s.flags, f); !ok && !(f == "--version" && path[0] == "version") {
					t.Errorf("help %q: itos %s documents %s, which its spec does not have", key, strings.Join(path, " "), f)
				}
			}
		}
	}
}

// fields splits a command line on spaces, ” an empty argument and a
// quoted one kept whole.
func fields(line string) []string {
	var out []string
	for _, part := range regexp.MustCompile(`'[^']*'|\S+`).FindAllString(line, -1) {
		out = append(out, strings.Trim(part, "'"))
	}
	return out
}

// quoted is fields turned back: an argument holding a space, or empty, in
// quotes.
func quoted(args []string) []string {
	out := slices.Clone(args)
	for i, a := range out {
		if a == "" || strings.Contains(a, " ") {
			out[i] = "'" + a + "'"
		}
	}
	return out
}
