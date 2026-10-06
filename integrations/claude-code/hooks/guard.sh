# The guard, Claude Code's PreToolUse command hook on Bash: itos guard claude-code, run
# by the itos on the PATH, Claude Code's JSON passed on through stdin.
#
# Only the itos on the PATH runs, the same rule as register.ts's titles (T-097): never
# the repository's hooks.bin, nor a tools/bin/itos at its top. The guard runs before
# every Bash command, in every repository the plugin is enabled for, so a program a
# cloned repository ships would run just by opening Claude Code there. A global install
# is the launcher, which fetches the version the repository's pin names from itos's
# releases, checked against their checksums. A developer of itos who wants a build of
# their own to answer puts it first on the PATH.
#
# Its two guards: no itos on the PATH answers nothing (exit 0), and an exit 2, which
# Claude Code would take as a block (an itos older than v6.0.0 has no guard
# claude-code), becomes 1.

command -v itos >/dev/null 2>&1 || exit 0
itos guard claude-code
s=$?
[ "$s" -ne 2 ] || s=1
exit "$s"
