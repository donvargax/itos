package cli

import (
	"os"
	"strings"
	"testing"
)

// itos status takes no positional argument, one or more each a usage
// error, exit 2, before anything is read; an --as with an empty value and a
// config that cannot be read are exit 2 too.
func TestStatusCommandRefusesItsArguments(t *testing.T) {
	statusRepo(t, map[string]string{"work-items.yaml": items("one"), "people.yaml": "- a\n"})
	for args, want := range map[string]string{
		"extra":       "status takes no arguments: extra",
		"extra more":  "status takes no arguments: extra more",
		"--as=":       "status takes one --as with a value",
		"--as a more": "status takes no arguments: more",
	} {
		code, stdout, stderr := run(append([]string{"status"}, strings.Fields(args)...)...)
		if code != ExitUsage || stdout != "" || stderr != "itos: "+want+" (itos --help)\n" {
			t.Errorf("itos status %s: exit %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
	if code, stdout, stderr := run("status", "--as", "a"); code != 0 {
		t.Fatalf("itos status --as a: exit %d\n%s%s", code, stdout, stderr)
	}
	if err := os.WriteFile("itos.yaml", []byte("version: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, stdout, stderr := run("status", "--as", "a"); code != ExitUsage || stdout != "" || !strings.Contains(stderr, "itos.yaml") {
		t.Errorf("a config that cannot be read: exit %d\n%s%s", code, stdout, stderr)
	}
}
