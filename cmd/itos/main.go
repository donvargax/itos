package main

import (
	"os"
	"runtime"

	"github.com/donvargax/itos/v8/internal/cli"
	"github.com/donvargax/itos/v8/internal/launch"
	"github.com/donvargax/itos/v8/internal/shim"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

func main() {
	goos := runtime.GOOS
	v := vcs.Git{Platform: goos}
	cl := commandLine()
	args := os.Args[1:]
	if shim.Named(os.Args[0], goos) {
		var code int
		var done bool
		if args, code, done = shim.Main(args, os.Stderr, v, cl); done {
			os.Exit(code)
		}
	}
	args = launch.Args(args)
	v.Export()
	if code := launch.Main(cl, args, os.Stderr, launch.Platform{OS: goos, Arch: runtime.GOARCH}); code != nil {
		os.Exit(*code)
	}
	os.Exit(cli.Main(args, os.Stdout, os.Stderr, goos))
}

// commandLine is what the launcher, and the shim through it, needs of the
// command line, which neither imports.
func commandLine() launch.CLI {
	return launch.CLI{Parse: parse, UpgradeTarget: cli.UpgradeTarget, ExitCode: cli.ExitCode}
}

// parse is cli.Parse, as the launcher reads it.
func parse(args []string) launch.Line {
	g := cli.Parse(args)
	return launch.Line{Help: g.Help, Config: g.Config, Root: g.Root, Rest: g.Rest, Extension: g.Extension}
}
