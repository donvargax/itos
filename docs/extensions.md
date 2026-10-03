# Writing an extension

A command itos does not have runs the program `itos-<command>` found on the
`PATH`, as git runs `git-<command>`: `itos hello a --b c` runs
`itos-hello a --b c` and exits with its exit code. Only the `PATH` is
searched, and a built-in command always wins, so an `itos-work` never runs in
place of `itos work`. `itos --help` lists the extensions it finds, and
`itos help hello` runs `itos-hello --help`.

## What an extension is given

- **Its arguments**: everything after its name, unread by itos, so it may take
  flags of any name, `--json` and `--help` among them.
- **Its folder**: the one itos runs in, after a `--root` written before the
  name.
- **Its environment**: itos's own, with
  - `ITOS_CONFIG`, the absolute path of the config itos would read (it may not
    exist);
  - `ITOS_ROOT`, the absolute path of the folder it runs in;
  - `ITOS_JSON`, `1` when `--json` came before the name, else unset;
  - `ITOS_BIN`, the itos binary that ran it, to call back;
  - `ITOS_VERSION`, that binary's version. The launcher reads it too, so
    `"$ITOS_BIN" <command>` runs this same version and never fetches another.

Only the global flags written before the name are itos's: in
`itos --json hello --json`, the first `--json` sets `ITOS_JSON` and the second
is `itos-hello`'s argument.

## itos-hello

The trivial first one, which prints what it received. Save it as `itos-hello`
in a folder on the `PATH` and make it executable:

```sh
#!/bin/sh
# itos-hello: prints what itos handed it.
if [ "${1:-}" = --help ]; then
	echo "Usage: itos hello [args]: prints what itos handed it"
	exit 0
fi
echo "hello from itos $ITOS_VERSION ($ITOS_BIN)"
echo "arguments: $*"
echo "folder: $(pwd) (ITOS_ROOT=$ITOS_ROOT)"
echo "config: $ITOS_CONFIG"
[ "${ITOS_JSON:-}" = 1 ] && echo "--json was given"
"$ITOS_BIN" version
```

```console
$ itos --json hello a --b c
hello from itos 2.1.0 (/home/me/.local/bin/itos)
arguments: a --b c
folder: /home/me/project (ITOS_ROOT=/home/me/project)
config: /home/me/project/itos.yaml
--json was given
itos 2.1.0
```

An extension that follows itos's conventions exits 0 on success, 1 on a policy
failure, 2 on a usage error and 3 when its environment is missing, and with
`ITOS_JSON=1` prints one JSON object with `"schema": 1` on stdout and its logs
on stderr (`PLAN.md` §7).
