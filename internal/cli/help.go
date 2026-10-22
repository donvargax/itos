package cli

import (
	"fmt"
	"strings"
)

// The help texts, help.ts ported: what `itos --help` and each command's
// --help print, each command's pages and lines declared beside its spec
// (internal/declare), itos's own here. Their words are for people
// (ADR-0052): tools/itos/conformance/help.yaml pins only that each exits 0
// and names its subcommands.

const globalFlags = `Global flags:
  --config <file>   the config (default ITOS_CONFIG, else itos.yaml, else the
                    stealth mode's <git common dir>/itos/itos.yaml; in a
                    subfolder with no itos.yaml, the repository top's, and
                    itos runs as if started there)
  --root <dir>      run as if started in <dir>
  --json            one object on stdout ("schema": 1, keys only ever added); logs on stderr
  --no-color        no colour (itos prints none; passed on as NO_COLOR)
  -q, --quiet       leave out a check's success line
  -h, --help        this text, or a command's`

const versions = `Versions: a config's pin (pin.version, pin.checksums) runs that itos release,
fetched from ITOS_RELEASES into ITOS_CACHE and checked first; ITOS_VERSION
runs another. A project's config with no pin runs this binary; no config, or a
stealth one with no pin, runs the newest release, asked for once an hour, never
in CI or with ITOS_NO_UPDATE=1; ITOS_NO_UPDATE_NOTICE=1 hides the notice that a
pin has fallen behind it.`

const otherCommands = `Other commands: one itos does not have runs itos-<command> from the PATH, its
exit code the run's, with the arguments after its name unread; global flags
before the name apply first and reach it as ITOS_CONFIG, ITOS_ROOT, ITOS_JSON,
ITOS_BIN and ITOS_VERSION. ` + "`itos help <command>`" + ` runs ` + "`itos-<command> --help`" + `.`

const asGit = `As git: linked as git before the real git on the PATH (itos git-shim install),
itos runs git commit and git push as itos commit and itos push in a repository
with an itos config, from any folder of it, refuses git add -A, -u or of a
folder there, and runs the real git for anything else.`

const exitCodes = `Exit codes: 0 success; 1 policy failure (a check failed, a commit rejected,
a registry or smoke problem, an unknown task, a failing step of ci run);
2 usage or config error; 3 missing environment (--as not among the people, a
release the server does not have or that fails its check, an extension that
cannot start, a git shim with no git to run, a commit or a push itos's hooks
would not run in); 70 an error itos cannot
classify: report it; 75 a failure that may pass when run again unchanged (a
release server or GitHub's API that cannot be reached or fails, a CI run not
finished within ci.watch.timeout).`

// commandsHeader is itos --help's lines before the commands'.
const commandsHeader = `itos: tasks, their checks, commit rules and CI plans

Usage: itos <command> [args] [global flags]

Commands:
`

// mainCommands is the header and each command's lines, in Rank order.
var mainCommands = func() string {
	var b strings.Builder
	b.WriteString(commandsHeader)
	for _, d := range declared {
		b.WriteString(d.Lines)
	}
	return strings.TrimSuffix(b.String(), "\n")
}()

const mainHelpTail = globalFlags + `

` + versions + `

` + otherCommands + `

` + asGit + `

` + exitCodes

// mainHelp is itos's own help where the PATH has no extension; with one, the
// extensions are listed after the commands (extensionsHelp).
var mainHelp = mainCommands + "\n\n" + mainHelpTail

// helpTexts is each command path's help, as each command declares it: the
// command and its subcommands joined by a space, "" for itos's own.
var helpTexts = func() map[string]string {
	texts := map[string]string{"": mainHelp}
	for _, d := range declared {
		for sub, text := range d.Help {
			texts[strings.TrimSpace(d.Name+" "+sub)] = text
		}
	}
	return texts
}()

// helpFor is the help for a command path: the longest one helpTexts knows,
// else itos's own.
func helpFor(path []string) string {
	for n := len(path); n > 0; n-- {
		if text, ok := helpTexts[strings.Join(path[:n], " ")]; ok {
			return text
		}
	}
	return mainHelp
}

// helpPath is the command path a --help asks about: the command and its
// subcommands, up to three words that are not flags.
func helpPath(rest []string) []string {
	var path []string
	for _, a := range rest {
		if len(path) == 3 {
			break
		}
		if !strings.HasPrefix(a, "-") {
			path = append(path, a)
		}
	}
	return path
}

// help prints what `itos`, `itos help <command>…` and any --help ask for,
// and gives the exit code: 2 for a bare `itos`, which names no command, and
// for `itos help <topic>` of a command itos does not have, else 0.
func help(g Globals, o Out) int {
	name, rest := "", []string(nil)
	if len(g.Rest) > 0 {
		name, rest = g.Rest[0], g.Rest[1:]
	}
	path := helpPath(g.Rest)
	if name == "help" {
		path = rest
		// An extension prints its own help; a topic that is neither one nor
		// a command of itos's is an unknown command, exit 2 (slice 89).
		if len(path) > 0 {
			if ext := extensionPath(path[0]); ext != "" {
				return runExtension(g, ext, []string{"--help"}, o)
			}
			if _, ok := dispatch(path[0]); !ok && path[0] != "help" {
				return failure(usage("unknown command: %s", path[0]), o)
			}
		}
	}
	text := helpFor(path)
	if text == mainHelp {
		text = mainCommands + extensionsHelp(o.OS) + "\n\n" + mainHelpTail
	}
	fmt.Fprintln(o.Stdout, text)
	if name == "" && !g.Help {
		return ExitUsage
	}
	return 0
}
