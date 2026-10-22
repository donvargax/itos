package version

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "version",
	Rank: 210,
	Lines: `  version [--check]                the version; --check against the config's requires
  --version                        itos version, as the first argument
`,
	Help: map[string]string{
		"": `Usage: itos version [--check]
       itos --version [--check]

Prints itos's version, and needs no config; a first argument --version is itos
version. In a project that pins a version, the pinned itos answers, and the
itos that was called says on stderr that the repository pins it and which
version it is itself. --check exits 1 when it does not satisfy the config's
requires, and 2 when the config cannot be read.

--json: {"schema":1,"version","requires"?,"satisfied"?}`,
	},
	Spec: declare.Spec{Flags: []declare.Flag{declare.Switch("--check")}},
}
