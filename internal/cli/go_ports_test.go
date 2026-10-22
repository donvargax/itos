package cli

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// goPorts bind the go slice to the real code: the guide slice's ports, and
// statusOf for where things stand, its problems when the registry has them;
// runGo reads the line and runs the slice over them.
func TestGoPortsBindTheRealCode(t *testing.T) {
	gitConfigRepo(t, "version: 1\nwork: { registry: work-items.yaml }\n")
	writeFile(t, "work-items.yaml", "phases: { 1: null }\nitems: []\n")
	var stderr strings.Builder
	p := goPorts{o: Out{Stderr: &stderr, OS: runtime.GOOS}}
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	st, found, err := p.Standing(t.Context(), cfg)
	if st == nil || found != nil || err != nil {
		t.Errorf("Standing = %+v, %v, %v, want where things stand", st, found, err)
	}
	writeFile(t, "work-items.yaml", "phases: { 1: null }\nitems: [{ id: p1-a }]\n")
	if st, found, err := p.Standing(t.Context(), cfg); st != nil || len(found) == 0 || err != nil {
		t.Errorf("Standing of a registry with problems = %+v, %v, %v, want its problems", st, found, err)
	}
	if err := os.Remove("work-items.yaml"); err != nil {
		t.Fatal(err)
	}
	var stdout strings.Builder
	if code, err := runGo(nil, Out{Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS}); code != 0 || err != nil ||
		!strings.HasPrefix(stdout.String(), "# ") {
		t.Errorf("runGo = %d, %v, stdout %.60q", code, err, stdout.String())
	}
	if _, err := runGo([]string{"x"}, Out{}); err == nil || err.Error() != "go takes no arguments: x" {
		t.Errorf("runGo x = %v, want the usage error", err)
	}
}
