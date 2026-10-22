package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// runGitShim reads its line through the slice, --dir from the folder the
// person stood in, and binds its ports to internal/shim on the OS itos runs
// on and to itos commit and itos push: install links this binary, uninstall
// takes the link away, and run hands git's arguments to the command.
func TestRunGitShimBindsItsPorts(t *testing.T) {
	dir := scratchFolder(t)
	var stdout, stderr strings.Builder
	o := Out{Stdout: &stdout, Stderr: &stderr, Origin: "sub", OS: runtime.GOOS}
	if code, err := runGitShim([]string{"bogus"}, o); code != 0 || ExitCode(err) != ExitUsage {
		t.Errorf("bogus: exit %d, %v", code, err)
	}
	self, link, err := (gitShimPorts{o}).Place(context.Background(), filepath.Join(dir, "sub", "bin"))
	if err != nil || filepath.Dir(link) != filepath.Join(dir, "sub", "bin") {
		t.Fatalf("the place: %s, %v", link, err)
	}
	if code, err := runGitShim([]string{"install", "--dir", "bin"}, o); code != 0 || err != nil {
		t.Fatalf("install: exit %d, %v, stderr %q", code, err, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "Linked "+link+" to "+self+".\n"+filepath.Dir(link)+" is not on the PATH") {
		t.Errorf("install said %q", stdout.String())
	}
	if mine, _ := (gitShimPorts{o}).LinksItos(context.Background(), link); !mine {
		t.Errorf("%s does not link this itos", link)
	}
	if at, _, _ := (gitShimPorts{o}).Standing(context.Background(), filepath.Dir(link)); at != -1 {
		t.Errorf("the folder stands at %d on the PATH", at)
	}
	stdout.Reset()
	if code, err := runGitShim([]string{"uninstall", "--dir", "bin"}, o); code != 0 || err != nil || stdout.String() != "Removed "+link+".\n" {
		t.Errorf("uninstall: exit %d, %v, %q", code, err, stdout.String())
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Errorf("the link is still there: %v", err)
	}
	for _, line := range [][]string{{"run", "commit", "--no-verify"}, {"run", "--", "push", "--force"}} {
		if _, err := runGitShim(line, o); ExitCode(err) != ExitUsage {
			t.Errorf("%q: %v", line, err)
		}
	}
}
