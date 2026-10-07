package main

import (
	"os"

	"github.com/donvargax/itos/v6/internal/cli"
	"github.com/donvargax/itos/v6/internal/git"
	"github.com/donvargax/itos/v6/internal/launch"
	"github.com/donvargax/itos/v6/internal/shim"
)

func main() {
	args := os.Args[1:]
	if shim.Named(os.Args[0]) {
		var code int
		var done bool
		if args, code, done = shim.Main(args, os.Stderr); done {
			os.Exit(code)
		}
	}
	args = launch.Args(args)
	git.Export()
	if code, launched := launch.Main(args, os.Stderr); launched {
		os.Exit(code)
	}
	os.Exit(cli.Main(args, os.Stdout, os.Stderr))
}
