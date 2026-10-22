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
	args := os.Args[1:]
	if shim.Named(os.Args[0], goos) {
		var code int
		var done bool
		if args, code, done = shim.Main(args, os.Stderr, v); done {
			os.Exit(code)
		}
	}
	args = launch.Args(args)
	v.Export()
	if code := launch.Main(args, os.Stderr, launch.Platform{OS: goos, Arch: runtime.GOARCH}); code != nil {
		os.Exit(*code)
	}
	os.Exit(cli.Main(args, os.Stdout, os.Stderr, goos))
}
