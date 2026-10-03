# The guard, Claude Code's PreToolUse command hook on Bash: itos hook pre-tool-use, run
# by the itos the repository's git hooks run, Claude Code's JSON passed on through stdin.
#
# Which itos, the same rule as resolveItos in register.ts, which the titles use and the
# plugin's tests prove case by case (they run no process, so they cannot run this file):
#   1. the effective hooks.bin, asked of the itos on the PATH in the session's folder with
#      itos config get hooks.bin: a path is relative to the repository's top, and is used
#      when it is executable; a bare word (itos) is that command on the PATH. An itos older
#      than v2.4.0 has no config get (exit 2), which is no answer, as is no itos at all;
#   2. with no answer, tools/bin/itos at the repository's top, when it is executable;
#   3. else the itos on the PATH.
# A hooks.bin of several words runs its first as the program, the rest its first arguments.
#
# Its two guards: nothing found answers nothing (exit 0), and an exit 2, which Claude Code
# would take as a block (an itos older than the guard has no such hook), becomes 1.

set -f
top=$(git rev-parse --show-toplevel 2>/dev/null </dev/null) || top=
bin=
if command -v itos >/dev/null 2>&1; then
	bin=$(itos config get hooks.bin 2>/dev/null </dev/null) || bin=
fi
# shellcheck disable=SC2086 # hooks.bin's words, split as itos splits them
set -- $bin
if [ $# -gt 0 ]; then
	case $1 in
	/*) [ -x "$1" ] || set -- ;;
	*/*)
		w=$1
		shift
		if [ -n "$top" ] && [ -x "$top/$w" ]; then set -- "$top/$w" "$@"; else set --; fi
		;;
	esac
fi
if [ $# -eq 0 ]; then
	if [ -n "$top" ] && [ -x "$top/tools/bin/itos" ]; then
		set -- "$top/tools/bin/itos"
	else
		set -- itos
	fi
fi
command -v "$1" >/dev/null 2>&1 || exit 0
"$@" hook pre-tool-use
s=$?
[ "$s" -ne 2 ] || s=1
exit "$s"
