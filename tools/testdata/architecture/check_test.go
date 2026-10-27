// Package architecture_test proves the arrows of ADR-0053's layout, the
// depguard rules of .golangci.yml and tools/bin/architecture-check's, over
// compile-valid fixtures in a temporary module: for each rule one import it
// accepts and one it refuses. It sits under testdata so ./... leaves it out;
// tools/bin/architecture-check runs it with the pinned linter. Adapted from
// github.com/donvargax/itos-cc's tools/testdata/architecture.
package architecture_test

import (
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The slices, engines and parts the fixtures stand on, beside the slices
// this repository has moved (docs/plans/slices-and-ports.md). internal/outline
// proves slice-packages allows internal/out alone, not every path it
// prefixes.
var stubs = append([]string{
	"internal/cli",
	"internal/declare",
	"internal/engine/alpha",
	"internal/engine/beta",
	"internal/git",
	"internal/idcounter",
	"internal/launch",
	"internal/lock",
	"internal/out",
	"internal/shim",
	"internal/outline",
	"internal/proptest",
	"internal/slices/alpha",
	"internal/slices/alpha/domain",
	"internal/slices/alpha/domain/port",
	"internal/slices/alpha/domain/port/porttest",
	"internal/slices/alpha/adapters/disk",
	"internal/slices/alpha/adapters/net",
	"internal/slices/beta",
	"internal/slices/beta/domain",
	"internal/slices/beta/adapters/disk",
	"internal/store/private/port",
	"internal/store/private/port/porttest",
	"internal/store/private/adapter",
	"internal/store/tree/port",
	"internal/store/tree/port/porttest",
	"internal/store/tree/adapter",
	"internal/store/tracker/port",
	"internal/store/tracker/port/porttest",
	"internal/store/tracker/adapter",
	"internal/vcs/port",
	"internal/vcs/port/porttest",
	"internal/vcs/adapter",
	"cmd/probe",
}, domains...)

// domains are the packages the commands share, which a slice imports
// (T-243): the domain packages, and as debt (p1-slice-io-behind-ports)
// providers, release and shell, which do I/O a slice should reach through
// its Ports.
var domains = []string{
	"internal/adr", "internal/ask", "internal/check", "internal/ci",
	"internal/config", "internal/draft", "internal/glob", "internal/guide",
	"internal/kind", "internal/ledger", "internal/message", "internal/nextid",
	"internal/plan", "internal/proof", "internal/scope", "internal/tests",
	"internal/value", "internal/version", "internal/work",
	"internal/providers", "internal/release", "internal/shell",
}

// stores are the stores, and the VCS, this repository has moved behind a
// port: each a port, its fake and its adapter, held by the same rules.
var stores = []string{"internal/store/private", "internal/store/tree", "internal/store/tracker", "internal/vcs"}

var mocks = []string{"github.com/golang/mock/gomock", "go.uber.org/mock/gomock", "github.com/stretchr/testify/mock", "github.com/vektra/mockery/v2", "github.com/maxbrunsfeld/counterfeiter/v6"}

// rapid is the property library internal/proptest wraps, a local module of one
// constant in the fixtures like the mocking libraries.
const rapid = "pgregory.net/rapid"

// fixture is one package importing one other: in layer/name, from its code
// or its test, refused by the depguard rule named, or by none.
type fixture struct {
	layer, name, imported, symbol, rule string
	test                                bool
}

func (f fixture) dir() string { return f.layer + "/" + f.name }

func (f fixture) path() string {
	if f.test {
		return f.dir() + "/probe_test.go"
	}
	return f.dir() + "/probe.go"
}

func (f fixture) source() string {
	if f.test {
		return fmt.Sprintf("package probe\n\nimport (\n dependency %q\n \"testing\"\n)\n\nfunc TestProbe(t *testing.T) {\n _ = t\n _ = dependency.%s\n}\n", f.imported, f.symbol)
	}
	return fmt.Sprintf("package probe\n\nimport dependency %q\n\nvar Probe = dependency.%s\n", f.imported, f.symbol)
}

// fixtures lists every depguard case, named after what it imports.
func fixtures(module string) []fixture {
	var all []fixture
	ours := func(path string) string { return module + "/" + path }
	add := func(layer, imported, symbol, rule string, test bool) {
		name := strings.NewReplacer("/", "_", ".", "_").Replace(strings.TrimPrefix(imported, module+"/"))
		if rule == "" {
			name += "_allowed"
		}
		if test {
			name += "_test"
		} else {
			name += "_code"
		}
		all = append(all, fixture{layer, name, imported, symbol, rule, test})
	}
	for _, test := range []bool{false, true} {
		// A slice's domain, its subpackages with it, imports nothing of ours
		// outside internal/slices: no UI, engine or cmd. What it imports
		// under internal/slices, depguard lets by and architecture-check's
		// siblings holds.
		for _, layer := range []string{"internal/slices/alpha/domain", "internal/slices/alpha/domain/port"} {
			for _, target := range []string{"internal/cli", "internal/engine/alpha", "internal/git", "internal/out", "cmd/probe"} {
				add(layer, ours(target), "Value", "slice-domain", test)
			}
			for _, target := range []string{"internal/slices/alpha/domain", "internal/slices/alpha/domain/port", "internal/slices/alpha", "internal/slices/alpha/adapters/disk", "internal/slices/beta/domain"} {
				add(layer, ours(target), "Value", "", test)
			}
			add(layer, "strings", "ToLower", "", test)
		}
		// A slice, outside its domain and adapters, imports nothing of ours
		// but its own subpackages, the engines, internal/out and the domain
		// packages: no UI, launcher, shim, other internals or cmd. A
		// fixture under internal/slices/alpha is a package of that slice.
		slice := "internal/slices/alpha"
		for _, target := range []string{"internal/cli", "internal/git", "internal/lock", "internal/idcounter", "internal/launch", "internal/shim", "internal/outline", "cmd/probe"} {
			add(slice, ours(target), "Value", "slice-packages", test)
		}
		for _, target := range append([]string{"internal/out", "internal/engine/alpha", "internal/slices/alpha/domain", "internal/slices/alpha/domain/port"}, domains...) {
			add(slice, ours(target), "Value", "", test)
		}
		add(slice, "strings", "ToLower", "", test)
		// An engine, its tests with it, imports no command, slice, launcher
		// or shim; the rest of ours it may, another engine among them. A
		// fixture under internal/engine/alpha is a package of that engine.
		engine := "internal/engine/alpha"
		for _, target := range []string{"internal/cli", "internal/slices/alpha", "internal/launch", "internal/shim"} {
			add(engine, ours(target), "Value", "engines", test)
		}
		for _, target := range []string{"internal/out", "internal/engine/beta"} {
			add(engine, ours(target), "Value", "", test)
		}
		add(engine, "strings", "ToLower", "", test)
		// The launcher and the shim, their tests with them, import no
		// internal/cli, which runs after them; the rest of ours they may,
		// the shim the launcher among it.
		for _, layer := range []string{"internal/launch", "internal/shim"} {
			add(layer, ours("internal/cli"), "Value", "pre-dispatch", test)
			add(layer, ours("internal/out"), "Value", "", test)
		}
		add("internal/shim", ours("internal/launch"), "Value", "", test)
		// A command's declaration, its tests with it, is a leaf: nothing of
		// ours and no I/O. A slice and internal/cli import it.
		for _, target := range []string{"internal/cli", "internal/engine/alpha", "internal/out"} {
			add("internal/declare", ours(target), "Value", "declare-leaf", test)
		}
		for _, io := range []struct{ pkg, symbol string }{{"os", "ReadFile"}, {"os/exec", "Command"}, {"net", "Dial"}} {
			add("internal/declare", io.pkg, io.symbol, "declare-leaf", test)
		}
		add("internal/declare", "strings", "ToLower", "", test)
		add(slice, ours("internal/declare"), "Value", "", test)
		add("internal/cli", ours("internal/declare"), "Value", "", test)
		for _, store := range stores {
			// A slice imports a store's port; never its adapter, which the
			// assembly binds. An engine takes the port its caller binds,
			// likewise.
			add(slice, ours(store+"/port"), "Value", "", test)
			add(slice, ours(store+"/adapter"), "Value", "slice-packages", test)
			add(engine, ours(store+"/port"), "Value", "", test)
			add(engine, ours(store+"/adapter"), "Value", "engines", test)
			// A store's port and its fake import nothing of ours but the
			// port and the fake: no UI, engine, slice or adapter.
			for _, layer := range []string{store + "/port", store + "/port/porttest"} {
				for _, target := range []string{"internal/cli", "internal/engine/alpha", "internal/git", "internal/slices/alpha", store + "/adapter"} {
					add(layer, ours(target), "Value", "store-ports", test)
				}
				for _, target := range []string{store + "/port", store + "/port/porttest"} {
					add(layer, ours(target), "Value", "", test)
				}
				add(layer, "strings", "ToLower", "", test)
			}
		}
		// The property tier, internal/proptest, and rapid are for tests: a
		// slice may import internal/proptest in its tests (slice-packages
		// allows it) but no package's code imports either, but
		// internal/proptest, which wraps rapid.
		for _, layer := range []string{"internal/slices/alpha", "internal/engine/alpha", "internal/git", "cmd/probe"} {
			tests := ""
			if !test {
				tests = "proptest-tests-only"
			}
			add(layer, ours("internal/proptest"), "Value", tests, test)
			add(layer, rapid, "Value", tests, test)
		}
		// Mocking libraries, nowhere: a slice's domain, an adapter, an
		// engine, the internals and cmd alike.
		for _, layer := range []string{"internal/slices/alpha/domain", "internal/slices/alpha/adapters/disk", "internal/engine/alpha", "internal/git", "cmd/probe"} {
			for _, mock := range mocks {
				add(layer, mock, "Value", "mocks", test)
			}
		}
	}
	// internal/proptest wraps rapid, which only it and tests import.
	add("internal/proptest", rapid, "Value", "", false)
	// A slice's domain does no I/O in its code; its tests may read fixtures.
	for _, io := range []struct{ pkg, symbol string }{{"os", "ReadFile"}, {"os/exec", "Command"}, {"net", "Dial"}, {"net/http", "Get"}} {
		add("internal/slices/alpha/domain", io.pkg, io.symbol, "slice-domain-io", false)
	}
	add("internal/slices/alpha/domain", "os", "ReadFile", "", true)
	// An adapter's code imports nothing of ours outside internal/slices; of
	// its slice, architecture-check's siblings lets it import the domain
	// alone. It does I/O, and its tests are free, a port's fake among them.
	adapter := "internal/slices/alpha/adapters/disk"
	for _, target := range []string{"internal/cli", "internal/git", "cmd/probe"} {
		add(adapter, ours(target), "Value", "slice-adapters", false)
		add(adapter, ours(target), "Value", "", true)
	}
	for _, target := range []string{"internal/slices/alpha/domain", "internal/slices/alpha/domain/port", "internal/slices/alpha", "internal/slices/alpha/adapters/net", "internal/slices/beta/domain"} {
		add(adapter, ours(target), "Value", "", false)
	}
	add(adapter, ours("internal/slices/alpha/domain/port/porttest"), "Value", "", true)
	add(adapter, "os", "ReadFile", "", false)
	for _, store := range stores {
		// A slice's code links no fake, which its tests may use; nor does
		// an engine's.
		add("internal/slices/alpha", ours(store+"/port/porttest"), "Value", "slice-fakes", false)
		add("internal/slices/alpha", ours(store+"/port/porttest"), "Value", "", true)
		add("internal/engine/alpha", ours(store+"/port/porttest"), "Value", "engine-fakes", false)
		add("internal/engine/alpha", ours(store+"/port/porttest"), "Value", "", true)
		// A port and its fake do no I/O in their code; their tests may.
		add(store+"/port", "os", "ReadFile", "store-ports-io", false)
		add(store+"/port/porttest", "net", "Dial", "store-ports-io", false)
		add(store+"/port", "os", "ReadFile", "", true)
		// A store's adapter's code imports no slice and links no fake; it
		// does I/O through the libraries it wraps, and its tests run the
		// port's contract suite, the fake with it.
		adapter := store + "/adapter"
		add(adapter, ours("internal/slices/alpha"), "Value", "store-adapters", false)
		add(adapter, ours(store+"/port/porttest"), "Value", "store-adapters", false)
		add(adapter, ours(store+"/port/porttest"), "Value", "", true)
		add(adapter, ours(store+"/port"), "Value", "", false)
		add(adapter, "os", "ReadFile", "", false)
	}
	// git runs in the VCS's git adapter alone: internal/git, its internals,
	// is no other package's to import, but in a test.
	add("internal/vcs/adapter", ours("internal/git"), "Value", "", false)
	for _, layer := range []string{"internal/store/private/adapter", "internal/store/tree/adapter", "internal/store/tracker/adapter", "internal/cli", "internal/engine/alpha", "cmd/probe"} {
		add(layer, ours("internal/git"), "Value", "git-internals", false)
		add(layer, ours("internal/git"), "Value", "", true)
	}
	// The lock is the stores' internals: their adapters', and the ID
	// counter's, which no other package's code imports, but in a test.
	for _, layer := range []string{"internal/store/private/adapter", "internal/store/tracker/adapter", "internal/idcounter"} {
		add(layer, ours("internal/lock"), "Value", "", false)
	}
	for _, layer := range []string{"internal/vcs/adapter", "internal/cli", "internal/engine/alpha", "cmd/probe"} {
		add(layer, ours("internal/lock"), "Value", "lock-internals", false)
		add(layer, ours("internal/lock"), "Value", "", true)
	}
	// The ID counter is the tracker adapter's internals, which no other
	// package's code imports, but in a test.
	add("internal/store/tracker/adapter", ours("internal/idcounter"), "Value", "", false)
	for _, layer := range []string{"internal/store/private/adapter", "internal/vcs/adapter", "internal/cli", "internal/engine/alpha", "cmd/probe"} {
		add(layer, ours("internal/idcounter"), "Value", "idcounter-internals", false)
		add(layer, ours("internal/idcounter"), "Value", "", true)
	}
	// What is outside internal/slices and importing a slice, which depguard
	// lets by and architecture-check's outside holds.
	add("internal/git", ours("internal/slices/alpha"), "Value", "", false)
	add("internal/cli", ours("internal/slices/alpha/domain"), "Value", "", true)
	add("internal/git", ours("internal/slices/alpha/domain"), "Value", "", true)
	add("internal/git/code", ours("internal/slices/alpha/domain"), "Value", "", false)
	return all
}

// TestArchitecture writes the fixtures into a temporary module named as
// this one, compiles them, lints them with the pinned golangci-lint and
// this repository's .golangci.yml, then runs architecture-check over them.
func TestArchitecture(t *testing.T) {
	root := os.Getenv("ARCHITECTURE_ROOT")
	linter := os.Getenv("ARCHITECTURE_LINTER")
	if root == "" || linter == "" {
		t.Fatal("run tools/bin/architecture-check")
	}
	dir := t.TempDir()
	moduleBytes, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	module := strings.Fields(string(moduleBytes))[1]
	write := func(path, contents string) {
		t.Helper()
		if strings.HasSuffix(path, ".go") {
			formatted, err := format.Source([]byte(contents))
			if err != nil {
				t.Fatalf("format %s: %v", path, err)
			}
			contents = string(formatted)
		}
		path = filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range stubs {
		write(path+"/stub.go", "package probe\n\nconst Value = 1\n")
	}
	// Each mocking library a local module of one constant, so the fixtures
	// compile with no network and no new dependency.
	mod := "module " + module + "\n\ngo 1.27\n"
	for i, path := range append(slices.Clone(mocks), rapid) {
		stub := fmt.Sprintf("stubs/library%d", i)
		version := "v0.0.0"
		for _, major := range []string{"/v2", "/v6"} {
			if strings.HasSuffix(path, major) {
				version = major[1:] + ".0.0"
			}
		}
		mod += fmt.Sprintf("\nrequire %s %s\nreplace %s => ./%s\n", path, version, path, stub)
		write(stub+"/go.mod", "module "+path+"\n\ngo 1.27\n")
		write(stub+"/stub.go", "package probe\n\nconst Value = 1\n")
	}
	write("go.mod", mod)
	cases := fixtures(module)
	for _, c := range cases {
		write(c.path(), c.source())
		if c.test {
			// A test file needs a package to test.
			write(c.dir()+"/stub.go", "package probe\n")
		}
	}
	// cmd assembles the slices; a binary linking a fake and one whose fake
	// enters only through a test, for --linked.
	fake := module + "/internal/slices/alpha/domain/port/porttest"
	write("cmd/slices/main.go", fmt.Sprintf("package main\n\nimport (\n _ %q\n _ %q\n _ %q\n _ %q\n)\n\nfunc main() {}\n", module+"/internal/slices/alpha", module+"/internal/slices/alpha/domain", module+"/internal/slices/alpha/adapters/disk", module+"/internal/slices/beta"))
	write("cmd/linked/main.go", fmt.Sprintf("package main\n\nimport _ %q\n\nfunc main() {}\n", fake))
	var storeFakes []string
	for _, store := range stores {
		storeFake := module + "/" + store + "/port/porttest"
		storeFakes = append(storeFakes, storeFake)
		write("cmd/"+path.Base(store)+"linked/main.go", fmt.Sprintf("package main\n\nimport _ %q\n\nfunc main() {}\n", storeFake))
	}
	write("cmd/tested/main.go", "package main\n\nfunc main() {}\n")
	write("cmd/tested/main_test.go", fmt.Sprintf("package main\n\nimport (\n _ %q\n \"testing\"\n)\n\nfunc TestProbe(t *testing.T) {\n _ = t\n}\n", fake))
	var expected []expectation
	for _, c := range cases {
		e := expectation{path: c.path()}
		if c.rule != "" {
			e = expectation{c.path(), "depguard", []string{"'" + c.imported + "'", "from list '" + c.rule + "'"}}
		}
		expected = append(expected, e)
	}
	expected = append(expected, otherLinters(write, module)...)
	env := append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOFLAGS=")
	compile := exec.Command("go", "test", "-count=1", "./...")
	compile.Dir = dir
	compile.Env = env
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("the fixtures must compile and their tests pass before lint: %v\n%s", err, output)
	}
	t.Run("lint", func(t *testing.T) { lint(t, linter, root, dir, env, expected) })
	check := filepath.Join(root, "tools", "bin", "architecture-check")
	checkLinked(t, check, dir, env, fake, storeFakes)
	checkSiblings(t, check, dir, env, module)
	checkOutside(t, check, dir, env, module)
}

// expectation is what the linter says of one fixture's file: nothing when
// linter is empty, else a diagnostic of that linter saying every text.
type expectation struct {
	path, linter string
	texts        []string
}

// otherLinters writes the fixtures of the linters beside depguard, each in a
// package of its own under internal/git, and says what each must get.
// ADR-0053 (5): a context is the first parameter, never a struct's field;
// containedctx refuses the field and revive's context-as-argument a context
// after another parameter. A type switch over a sealed failure set
// (//sumtype:decl) names every kind, a default arm not counting
// (gochecksumtype).
func otherLinters(write func(path, contents string), module string) []expectation {
	var expected []expectation
	contexts := func(name, body, linter, text string) {
		path := "internal/git/" + name + "/probe.go"
		write(path, "package probe\n\nimport \"context\"\n\n"+body)
		e := expectation{path: path}
		if linter != "" {
			e = expectation{path, linter, []string{text}}
		}
		expected = append(expected, e)
	}
	contexts("context_held", "type Holder struct {\n Ctx context.Context\n}\n\nfunc Probe(ctx context.Context) Holder {\n return Holder{Ctx: ctx}\n}\n", "containedctx", "contains a context.Context field")
	contexts("context_second", "func Probe(n int, ctx context.Context) error {\n _ = n\n return ctx.Err()\n}\n", "revive", "context.Context should be the first parameter")
	contexts("context_first_allowed", "func Probe(ctx context.Context, n int) error {\n _ = n\n return ctx.Err()\n}\n", "", "")
	write("internal/git/sealed/sealed.go", `package sealed

// Failure is a sealed set of two kinds.
//
//sumtype:decl
type Failure interface {
	error
	failure()
}

// Missing is one kind.
type Missing struct{ Path string }

func (*Missing) failure() {}
func (e *Missing) Error() string { return "missing: " + e.Path }

// Unreadable is the other.
type Unreadable struct{ Path string }

func (*Unreadable) failure() {}
func (e *Unreadable) Error() string { return "unreadable: " + e.Path }
`)
	sumtype := func(name, arms string, refused bool) {
		path := "internal/git/" + name + "/probe.go"
		write(path, fmt.Sprintf("package probe\n\nimport sealed %q\n\n// Code is failure's exit code.\nfunc Code(failure sealed.Failure) int {\n code := 70\n switch failure.(type) {\n%s }\n return code\n}\n", module+"/internal/git/sealed", arms))
		e := expectation{path: path}
		if refused {
			e = expectation{path, "gochecksumtype", []string{"missing cases for Unreadable"}}
		}
		expected = append(expected, e)
	}
	missing := "case *sealed.Missing:\n code = 3\n"
	sumtype("sumtype_allowed", missing+"case *sealed.Unreadable:\n code = 4\n", false)
	sumtype("sumtype_omitted", missing, true)
	sumtype("sumtype_default", missing+"default:\n", true)
	return expected
}

// lint runs the linter over the temporary module and holds each fixture to
// its expectation: a refused one refused by its linter, saying its texts,
// and nothing else reported.
func lint(t *testing.T, linter, root, dir string, env []string, cases []expectation) {
	outputPath := filepath.Join(dir, "issues.json")
	// Absolute output paths, so the config and the fixtures may sit on
	// different Windows drives.
	run := exec.Command(linter, "run", "--config", filepath.Join(root, ".golangci.yml"), "--output.json.path", outputPath, "--path-mode=abs", "--max-issues-per-linter=0", "--max-same-issues=0", "./...")
	run.Dir = dir
	run.Env = env
	output, err := run.CombinedOutput()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			t.Fatalf("the linter failed to run, not a refusal: %v\n%s", err, output)
		}
	}
	var report struct {
		Issues []struct {
			FromLinter, Text string
			Pos              struct{ Filename string }
		}
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read the linter's JSON: %v\n%s", err, output)
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	canonicalDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	refused := make(map[string]bool)
	for _, issue := range report.Issues {
		// macOS's temporary folder is reached through /var -> /private/var.
		filename, err := filepath.EvalSymlinks(issue.Pos.Filename)
		if err != nil {
			t.Fatal(err)
		}
		rel, err := filepath.Rel(canonicalDir, filename)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.ToSlash(rel)
		i := slices.IndexFunc(cases, func(c expectation) bool { return c.path == path })
		if i < 0 || cases[i].linter != issue.FromLinter || !saysAll(issue.Text, cases[i].texts) {
			t.Errorf("a diagnostic no fixture expects: %s: %s: %s", path, issue.FromLinter, issue.Text)
			continue
		}
		refused[path] = true
	}
	for _, c := range cases {
		if c.linter != "" && !refused[c.path] {
			t.Errorf("%s: no %s refusal saying %q", c.path, c.linter, c.texts)
		}
	}
}

// saysAll is whether text says every one of texts.
func saysAll(text string, texts []string) bool {
	for _, want := range texts {
		if !strings.Contains(text, want) {
			return false
		}
	}
	return true
}

// checkLinked runs architecture-check --linked: a binary linking a port's
// fake refused, naming both, a slice's port's or a store's, and one whose
// fake is a test's accepted.
func checkLinked(t *testing.T, check, dir string, env []string, fake string, storeFakes []string) {
	type linked struct {
		pkg, fake string
		refused   bool
	}
	cases := []linked{{"./cmd/linked", fake, true}, {"./cmd/tested", fake, false}}
	for i, store := range stores {
		cases = append(cases, linked{"./cmd/" + path.Base(store) + "linked", storeFakes[i], true})
	}
	for _, c := range cases {
		t.Run("linked "+c.pkg, func(t *testing.T) {
			output, err := runCheck(check, dir, env, "--linked", c.pkg)
			expect(t, output, err, c.refused, c.pkg+" links "+c.fake, "_test.go")
		})
	}
}

// checkSiblings runs architecture-check --siblings over the fixtures that
// import under internal/slices: another slice refused, and within a slice
// the domain importing anything of it but the domain, and an adapter
// anything of it but the domain and itself.
func checkSiblings(t *testing.T, check, dir string, env []string, module string) {
	domain := "./internal/slices/alpha/domain"
	adapter := "./internal/slices/alpha/adapters/disk"
	for _, c := range []struct{ pkg, imported, says string }{
		{domain + "/internal_slices_alpha_domain_allowed_code", "", ""},
		{domain + "/internal_slices_alpha_domain_port_allowed_test", "", ""},
		{domain + "/port/internal_slices_alpha_domain_allowed_code", "", ""},
		{domain + "/internal_slices_beta_domain_allowed_code", "internal/slices/beta/domain", "a slice imports no other slice"},
		{domain + "/internal_slices_beta_domain_allowed_test", "internal/slices/beta/domain", "a slice imports no other slice"},
		{domain + "/internal_slices_alpha_allowed_code", "internal/slices/alpha", "a slice's domain imports nothing of its slice but its domain"},
		{domain + "/internal_slices_alpha_adapters_disk_allowed_test", "internal/slices/alpha/adapters/disk", "a slice's domain imports nothing of its slice but its domain"},
		{adapter + "/internal_slices_alpha_domain_allowed_code", "", ""},
		{adapter + "/internal_slices_alpha_domain_port_porttest_allowed_test", "", ""},
		{adapter + "/internal_slices_alpha_allowed_code", "internal/slices/alpha", "an adapter imports nothing of its slice but its domain"},
		{adapter + "/internal_slices_alpha_adapters_net_allowed_code", "internal/slices/alpha/adapters/net", "an adapter imports nothing of its slice but its domain"},
		{adapter + "/internal_slices_beta_domain_allowed_code", "internal/slices/beta/domain", "a slice imports no other slice"},
		{"./cmd/slices", "", ""},
	} {
		t.Run("siblings "+c.pkg, func(t *testing.T) {
			output, err := runCheck(check, dir, env, "--siblings", c.pkg)
			importer := module + "/" + strings.TrimPrefix(c.pkg, "./")
			expect(t, output, err, c.imported != "", importer+" imports "+module+"/"+c.imported+";", c.says, "ADR-0053")
		})
	}
}

// runCheck runs architecture-check with args in the temporary module.
func runCheck(check, dir string, env []string, args ...string) ([]byte, error) {
	run := exec.Command("sh", append([]string{check}, args...)...)
	run.Dir = dir
	run.Env = env
	return run.CombinedOutput()
}

// expect holds a run of architecture-check to its outcome: exit 0 when
// accepted, exit 1 saying each text when refused.
func expect(t *testing.T, output []byte, err error, refused bool, texts ...string) {
	t.Helper()
	if !refused {
		if err != nil {
			t.Errorf("refused what it accepts: %v\n%s", err, output)
		}
		return
	}
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		t.Fatalf("no refusal: %v\n%s", err, output)
	}
	for _, text := range texts {
		if !strings.Contains(string(output), text) {
			t.Errorf("the refusal does not say %q:\n%s", text, output)
		}
	}
}
