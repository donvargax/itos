// Command real-git prints the git itos runs, as internal/git finds it
// (git.Bin): the git ITOS_GIT names, unless it is an itos, else the first git
// on the PATH that is not an itos (T-122).
//
// A git on the PATH may be an itos linked as git (itos git-shim install),
// whose git commit is itos commit, held to whatever policy the itos installed
// globally has. The conformance runner (tools/itos/conformance/run.ts) starts
// git itself, so it asks this program for the real one rather than look git up
// on the PATH, and hands it on as ITOS_GIT, as an itos run hands its git to
// everything it starts; the gates' self-tests, Go tests since T-143, call
// git.Bin themselves. The rule stays internal/git's alone; this program only
// says what it finds.
//
// No git on the PATH but an itos exits 3, internal/git's ErrNoGit.
//
//	go run ./tools/bin/real-git
package main

import (
	"fmt"
	"os"

	"github.com/donvargax/itos/v8/internal/git"
)

func main() {
	bin := git.Bin()
	if bin == "git" {
		fmt.Fprintf(os.Stderr, "real-git: %v\n", git.ErrNoGit)
		os.Exit(3)
	}
	fmt.Println(bin)
}
