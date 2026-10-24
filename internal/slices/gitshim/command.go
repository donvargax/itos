package gitshim

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "git-shim",
	Rank: 110,
	Lines: `  git-shim install|uninstall [--dir <folder>]
                                   link itos as git, for itos's commit and push
`,
	Help: map[string]string{
		"": `Usage: itos git-shim install [--dir <folder>]
       itos git-shim uninstall [--dir <folder>]
       itos git-shim run <commit|push> [<git args>…]

The git shim is itos started under the name git, through a link named git
(git.exe on windows) to it in a folder before the real git on the PATH. In a
repository itos manages (an itos.yaml in the folder git runs in or at the
repository's top, or a stealth config), git commit runs as itos commit and git
push as itos push, with git's arguments, none of them read as itos's global
flags; git's -C <path> and -c <name>=<value> before the command are honoured.
Every other command, every command elsewhere, and every git an itos run starts
run the real git, the first git on the PATH that is not itos, with the same
arguments, streams and exit code; with no such git the shim exits 3. In a
pinned repository git commit and git push are the pinned itos's; a pin, or
ITOS_VERSION, older than 2.2.0, which has no git shim, runs the real git
instead, after a line on stderr saying so. itos commit and itos push refuse
--no-verify, commit's -n and -a, and a core.hooksPath set by -c, exit 2.

In a repository itos manages, git add (or git stage) that stages more than
the files it names is refused, exit 2, naming git add <path>, and nothing
runs: -A, --all, -u or --update, or a folder, . among them. git add of files
by their paths runs the real git, as everywhere else.

install links the itos running as git in <folder> (default: the folder holding
it) and says whether that folder comes before the real git on the PATH. It
never replaces a git that is not itos (exit 1), and replaces another itos, a
link to one or a copy. uninstall removes the link, and leaves any other git
alone (exit 1). Both run the binary called, whatever a pin says. run is what the
link runs in a repository itos manages: git commit or git push, as itos's.

--json: {"schema":1,"link","action","target"?,"on_path"?,"before_git"?,"git"?},
        action one of linked, kept, replaced, refused (install) or removed,
        absent, refused (uninstall)`,
	},
	Spec: declare.Spec{Subs: map[string]declare.Spec{
		"install":   {Flags: []declare.Flag{declare.Value("--dir")}},
		"uninstall": {Flags: []declare.Flag{declare.Value("--dir")}},
		"run":       {Unread: true},
	}},
}
