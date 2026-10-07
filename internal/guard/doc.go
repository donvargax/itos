// Package guard is itos guard claude-code (features/guard.feature): Claude
// Code's PreToolUse hook, which the itos plugin wires on Bash, asks before
// each tool runs, sending the tool's name, its input and the session's
// folder as JSON on stdin. In a repository itos manages (config.Managed,
// judged from the input's folder, as the git shim judges git's), a Bash
// command that runs git commit or git push is denied, the reason naming the
// itos command to use instead, which Claude Code shows the agent. Everything
// else gets no answer, so Claude Code's own permission rules decide as if no
// hook ran: the guard never allows anything, since an allow would skip the
// person's rules for every command it let through.
//
// Read refuses an input that is not one JSON object, has no string
// tool_name, or is a Bash input with no string command. GitCalls reads the
// command as bash reads it (mvdan.cc/sh/v3/syntax), walking every call
// expression, so a call inside a list, a pipeline, a subshell, a compound
// command, a function's body or a command substitution is met in its turn.
// literal gives a word's text when it is fixed (unquoted with bash's
// backslashes taken out, single-quoted, double-quoted with no expansion),
// and the first word that is not fixed ends what the call can say. gitCall
// skips the call's leading assignments and the precommands command, exec,
// nohup and env (with env's NAME=VALUE words), takes git or a path ending in
// /git, then git's global options (-C <path> moving the folder judged, as
// git would move, the valued options passing over their next word, any
// other - word passed over) and the subcommand. Text an argument carries
// (grep 'git commit') is a word, not a command. Reason asks config.Managed
// of each call's folder and names each guarded subcommand once, in
// Guarded's order; Deny encodes Claude Code's hookSpecificOutput, with no
// HTML escaping so the reason reads as written.
//
// A guardrail for agents that follow it, not a fortress (the user's call,
// 2026-10-03): a command in sh -c, eval, a script, a git alias, a word built
// from a variable, a command that moves with cd first, or one bash cannot
// parse is not looked into, and the commit-msg hook and CI's verify stay the
// gates.
//
// The command line (internal/cli/guard.go) never returns an error from it,
// since a usage error's exit 2 is what Claude Code takes as a block: an
// input Read refuses is one stderr line and exit 1. The folder judged is the
// input's cwd, read from where itos was started when relative. The launcher
// answers for the guard where it would hand it to an itos without one
// (internal/launch).
package guard
