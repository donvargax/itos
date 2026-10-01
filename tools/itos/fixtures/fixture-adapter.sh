#!/bin/sh
# The fixture kind's adapter (tools/itos/fixtures/command-adapter.yaml):
# three named tests in two files, F-002 not live, and a file with none.
#
#   sh tools/itos/fixtures/fixture-adapter.sh list --at <tree>   the adapter protocol's JSON
#   sh tools/itos/fixtures/fixture-adapter.sh run [<pattern>]    the IDs it matches
case "$1" in
list)
	printf '%s\n' '{"protocol":1,"tests":[{"id":"F-001","file":"one.txt","live":true},{"id":"F-002","file":"one.txt","live":false},{"id":"F-003","file":"two.txt","live":true}],"files":["empty.txt","one.txt","two.txt"]}'
	;;
run)
	printf '%s\n' F-001 F-002 F-003 | grep -E "${2:-.}"
	;;
*)
	echo "usage: fixture-adapter.sh list --at <tree> | run [<pattern>]" >&2
	exit 2
	;;
esac
