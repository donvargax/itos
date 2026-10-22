package guard

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "guard",
	Rank: 190,
	Lines: `  guard claude-code                Claude Code's PreToolUse hook: denies git commit and git push
`,
	Help: map[string]string{
		"": `Usage: itos guard claude-code`,

		"claude-code": `Usage: itos guard claude-code

Claude Code's PreToolUse hook, which the itos plugin runs before each Bash
command. Reads Claude Code's JSON on stdin (tool_name, tool_input.command,
cwd). In a repository itos manages, found from cwd (else the folder itos runs
in) as everywhere else, a Bash command that runs git commit or git push is
denied: exit 0 and Claude Code's deny on stdout, its reason naming itos commit
--task <id> or --scenarios <ids>, or itos push. The command is read as bash
reads it: each command of a list, pipeline, subshell or substitution, its
variable assignments skipped, past command, exec, nohup and env, then git (or
a path ending in /git), its global options (-C moves the folder judged), and
the subcommand; text an argument carries is not a command. sh -c, eval,
scripts, git aliases and words built from variables are not looked into.
Everything else gets no answer (exit 0, nothing on stdout), never an allow,
so Claude Code's permission rules decide. An input it cannot read: exit 1,
the reason on stderr; Claude Code blocks a tool on exit 2 alone. Where the
launcher would hand it to an itos older than 6.0.0, which has no such command
(a pin, ITOS_VERSION or the newest release), it gets no answer either, after
a line on stderr saying why.`,
	},
	Spec: declare.Spec{Subs: map[string]declare.Spec{
		"claude-code": {},
	}},
}
