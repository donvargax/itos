// Command assemble writes internal/cli/commands_gen.go, the command table,
// from the commands' declarations, so a lane moving one command touches only
// its own files and the generated one (ADR-0053, step 5 of
// docs/plans/slices-and-ports.md; as itos-cc's tools/bin/assemble).
//
// A command still in internal/cli declares itself in one of the package's
// own Go files, its tests and commands_gen.go left out, by a package-level
//
//	var taskDeclared = declare.Command{Name: "task", Rank: 10, …}
//
// and a slice, a folder right under internal/slices, in its top folder's own
// Go files by
//
//	var Command = declare.Command{Name: "guard", Rank: 80, …}
//
// declare being github.com/donvargax/itos/v8/internal/declare, under the
// module go.mod names. The source is parsed, never built, so a tree that does
// not compile is read as well as one that does. Refused, naming the file or
// files and writing nothing: a Name that is no non-empty string literal, a
// Rank that is no integer literal, two declarations of one Name or one Rank
// (a slice's and internal/cli's of one Name said apart), a slice with no
// Command and a declaration in a slice's subpackage, which nothing would
// assemble.
//
// The file imports declare and each slice, aliased <package>slice, sorted by
// path; declared lists every declaration in Rank order, itos --help's, and
// dispatch switches, in the same order, from each Name to its run function,
// run<Name in Pascal case> (git-shim's runGitShim), which the compiler holds
// internal/cli to. One entry a line, so a change to one command changes one
// or two lines. It is formatted by go/format and carries the standard "Code
// generated … DO NOT EDIT." line: after adding, removing, renaming or
// re-ranking a declaration, run
//
//	go run ./tools/bin/assemble
//
// from the repository's root and commit the file. With -check it writes
// nothing and fails, naming the file, when the committed one is not what it
// would write.
//
// Exit status: 0 written or current, 1 a declaration refused or the file not
// current, 2 an argument, the tree could not be read or the file written.
package main

import (
	"bytes"
	"cmp"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
)

const self = "assemble"

// output is the file assemble writes, from the repository's root.
const output = cliRoot + "/commands_gen.go"

// cliRoot is the package of the commands not yet moved into slices, and
// sliceRoot the folder each slice is a folder of, from the repository's root.
const (
	cliRoot   = "internal/cli"
	sliceRoot = "internal/slices"
)

func main() {
	check := flag.Bool("check", false, "write nothing; fail when "+output+" is not what assemble writes")
	flag.Parse()
	root, err := os.OpenRoot(".")
	if err != nil {
		say(os.Stderr, "%s: %v\n", self, err)
		os.Exit(2)
	}
	write := func(name string, data []byte) error { return root.WriteFile(name, data, 0o644) }
	code := run(root.FS(), write, *check, flag.Args(), os.Stderr)
	_ = root.Close()
	os.Exit(code)
}

// refused is a declaration assemble cannot assemble: exit 1, as a stale file
// is.
type refused struct{ reason string }

func (r refused) Error() string { return r.reason }

// run writes the file of the repository tree is, through write, or with
// check holds the committed one to it, and returns the exit status.
func run(tree fs.FS, write func(name string, data []byte) error, check bool, args []string, stderr io.Writer) int {
	if len(args) > 0 {
		say(stderr, "%s: unexpected argument %q\n", self, args[0])
		return 2
	}
	source, err := assemble(tree)
	if r := (refused{}); errors.As(err, &r) {
		say(stderr, "%s: %s\n", self, r.reason)
		return 1
	}
	if err != nil {
		say(stderr, "%s: %v\n", self, err)
		return 2
	}
	if check {
		committed, err := fs.ReadFile(tree, output)
		if err != nil || !bytes.Equal(committed, source) {
			say(stderr, "%s: %s is not what the command declarations make: run go run ./tools/bin/assemble and commit it\n", self, output)
			return 1
		}
		return 0
	}
	if err := write(output, source); err != nil {
		say(stderr, "%s: %v\n", self, err)
		return 2
	}
	return 0
}

func say(w io.Writer, format string, a ...any) { _, _ = fmt.Fprintf(w, format, a...) }

// declaration is one command's declaration: the file holding it, the name
// and rank it gives its command, what the generated file calls it by, and,
// for a slice's, the slice's import path and the alias it is imported by.
type declaration struct {
	file, name string
	rank       int64
	ref        string
	importPath string
	alias      string
}

// assemble is the generated file's source over tree, or why a declaration
// is refused.
func assemble(tree fs.FS) ([]byte, error) {
	module, err := modulePath(tree)
	if err != nil {
		return nil, err
	}
	declare := module + "/internal/declare"
	found, err := fromCLI(tree, declare)
	if err != nil {
		return nil, err
	}
	sliced, err := fromSlices(tree, module, declare)
	if err != nil {
		return nil, err
	}
	found = append(found, sliced...)
	if err := unique(found); err != nil {
		return nil, err
	}
	slices.SortFunc(found, func(x, y declaration) int { return cmp.Compare(x.rank, y.rank) })
	return render(declare, found)
}

// modulePath is the module path go.mod at tree's root names.
func modulePath(tree fs.FS) (string, error) {
	mod, err := fs.ReadFile(tree, "go.mod")
	if err != nil {
		return "", err
	}
	for line := range strings.Lines(string(mod)) {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`), nil
		}
	}
	return "", errors.New("go.mod names no module")
}

// fromCLI is every declaration internal/cli's own Go files hold, each called
// by its variable's name.
func fromCLI(tree fs.FS, declare string) ([]declaration, error) {
	var found []declaration
	err := eachFile(tree, cliRoot, func(file string, f *ast.File) error {
		vars, err := declarations(f, file, declare)
		for _, v := range vars {
			found = append(found, declaration{file: file, name: v.name, rank: v.rank, ref: v.variable})
		}
		return err
	})
	return found, err
}

// fromSlices is the declaration of each slice under sliceRoot, or why one is
// refused.
func fromSlices(tree fs.FS, module, declare string) ([]declaration, error) {
	entries, err := fs.ReadDir(tree, sliceRoot)
	if err != nil {
		return nil, err
	}
	var found []declaration
	for _, e := range entries {
		if !e.IsDir() || skipped(e.Name()) {
			continue
		}
		d, err := fromSlice(tree, module, declare, sliceRoot+"/"+e.Name())
		if err != nil {
			return nil, err
		}
		found = append(found, d)
	}
	return found, nil
}

// fromSlice is the declaration of the slice in folder, its var Command, or
// why it is refused.
func fromSlice(tree fs.FS, module, declare, folder string) (declaration, error) {
	d := declaration{importPath: module + "/" + folder}
	declared := false
	err := eachFile(tree, folder, func(file string, f *ast.File) error {
		vars, err := declarations(f, file, declare)
		for _, v := range vars {
			if v.variable == "Command" {
				d.file, d.name, d.rank, declared = file, v.name, v.rank, true
				d.alias = f.Name.Name + "slice"
				d.ref = d.alias + ".Command"
			}
		}
		return err
	})
	if err != nil {
		return declaration{}, err
	}
	if !declared {
		return declaration{}, refused{fmt.Sprintf("%s declares no command: give it a package-level var Command = declare.Command{Name: …, Rank: …}", folder)}
	}
	return d, inSubpackages(tree, declare, folder)
}

// inSubpackages refuses a declaration in a package below folder's, which
// nothing assembles.
func inSubpackages(tree fs.FS, declare, folder string) error {
	return fs.WalkDir(tree, folder, func(dir string, e fs.DirEntry, err error) error {
		if err != nil || !e.IsDir() || dir == folder {
			return err
		}
		if skipped(e.Name()) {
			return fs.SkipDir
		}
		return eachFile(tree, dir, func(file string, f *ast.File) error {
			vars, err := declarations(f, file, declare)
			if err == nil && len(vars) > 0 {
				err = refused{fmt.Sprintf("%s declares %s in a subpackage of %s: a slice declares its command in its top folder, as var Command", file, vars[0].variable, folder)}
			}
			return err
		})
	})
}

// skipped is whether the go tool leaves a folder of this name out of
// ./...: testdata, and one whose name starts with . or _.
func skipped(name string) bool {
	return name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

// eachFile parses each Go file right in dir, its tests and the generated
// file left out, and hands it to do, stopping at do's first error.
func eachFile(tree fs.FS, dir string, do func(file string, f *ast.File) error) error {
	entries, err := fs.ReadDir(tree, dir)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		file := dir + "/" + e.Name()
		if e.IsDir() || !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") || file == output {
			continue
		}
		src, err := fs.ReadFile(tree, file)
		if err != nil {
			return err
		}
		f, err := parser.ParseFile(fset, file, src, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		if err := do(file, f); err != nil {
			return err
		}
	}
	return nil
}

// literal is one package-level var X = declare.Command{…}: X, and the name
// and rank it gives.
type literal struct {
	variable, name string
	rank           int64
}

// declarations is every package-level var X = declare.Command{…} in f, the
// file at file, or why one is refused.
func declarations(f *ast.File, file, declare string) ([]literal, error) {
	pkg := importName(f, declare)
	if pkg == "" {
		return nil, nil
	}
	var found []literal
	for _, d := range f.Decls {
		g, isGen := d.(*ast.GenDecl)
		if !isGen || g.Tok != token.VAR {
			continue
		}
		for _, spec := range g.Specs {
			v := spec.(*ast.ValueSpec)
			if len(v.Values) != len(v.Names) {
				continue
			}
			for i, value := range v.Values {
				lit, isLit := value.(*ast.CompositeLit)
				if !isLit || !isSelector(lit.Type, pkg, "Command") {
					continue
				}
				variable := v.Names[i].Name
				name, rank, err := fields(lit)
				if err != nil {
					return nil, refused{fmt.Sprintf("%s: %s %v", file, variable, err)}
				}
				found = append(found, literal{variable: variable, name: name, rank: rank})
			}
		}
	}
	return found, nil
}

// fields is the Name and the Rank lit gives, or why they are no literals.
func fields(lit *ast.CompositeLit) (string, int64, error) {
	name, rank := "", int64(0)
	named, ranked := false, false
	for _, el := range lit.Elts {
		kv, isKV := el.(*ast.KeyValueExpr)
		if !isKV {
			continue
		}
		key, _ := kv.Key.(*ast.Ident)
		value, isBasic := kv.Value.(*ast.BasicLit)
		if key == nil || !isBasic {
			continue
		}
		switch {
		case key.Name == "Name" && value.Kind == token.STRING:
			name, _ = strconv.Unquote(value.Value)
			named = name != ""
		case key.Name == "Rank" && value.Kind == token.INT:
			n, err := strconv.ParseInt(value.Value, 0, 64)
			rank, ranked = n, err == nil
		}
	}
	switch {
	case !named:
		return "", 0, errors.New(`names its command by no string literal: give it a Name such as Name: "task"`)
	case !ranked:
		return "", 0, errors.New("ranks its command by no integer literal: give it a Rank, its place in itos --help, such as Rank: 10")
	}
	return name, rank, nil
}

// importName is the name f refers to the package at importPath by, "" when
// f does not import it.
func importName(f *ast.File, importPath string) string {
	for _, spec := range f.Imports {
		if p, _ := strconv.Unquote(spec.Path.Value); p == importPath {
			if spec.Name != nil {
				return spec.Name.Name
			}
			return path.Base(p)
		}
	}
	return ""
}

// isSelector is whether e is pkg.name.
func isSelector(e ast.Expr, pkg, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == pkg && sel.Sel.Name == name
}

// unique refuses two declarations of one Name or one Rank, naming both
// files. found holds internal/cli's declarations before the slices'.
func unique(found []declaration) error {
	for i, d := range found {
		for _, other := range found[:i] {
			switch {
			case other.name == d.name && other.importPath == "" && d.importPath != "":
				return refused{fmt.Sprintf("%s and %s both declare the command %q: a command moved into its slice leaves internal/cli", d.file, other.file, d.name)}
			case other.name == d.name:
				return refused{fmt.Sprintf("%s and %s both declare the command %q: a name runs one command", other.file, d.file, d.name)}
			case other.rank == d.rank:
				return refused{fmt.Sprintf("%s and %s both rank their command %d: each command has its own place in itos --help", other.file, d.file, d.rank)}
			}
		}
	}
	return nil
}

// header is the file's first lines, the standard line go tooling and
// golangci-lint read a generated file by among them.
const header = `// Code generated by tools/bin/assemble from the command declarations; DO NOT EDIT.
// Run go run ./tools/bin/assemble after adding, removing, renaming or re-ranking one.

package cli

`

// render is the file's source over found, in Rank order, formatted by
// go/format.
func render(declare string, found []declaration) ([]byte, error) {
	var b strings.Builder
	b.WriteString(header)
	imports := []string{strconv.Quote(declare)}
	for _, d := range found {
		if d.importPath != "" {
			imports = append(imports, d.alias+" "+strconv.Quote(d.importPath))
		}
	}
	slices.SortFunc(imports, func(x, y string) int {
		return strings.Compare(x[strings.IndexByte(x, '"'):], y[strings.IndexByte(y, '"'):])
	})
	b.WriteString("import (\n")
	for _, i := range imports {
		fmt.Fprintf(&b, "\t%s\n", i)
	}
	b.WriteString(")\n\n")
	b.WriteString("// declared is every command's declaration, in Rank order: itos --help's.\n")
	b.WriteString("var declared = []declare.Command{\n")
	for _, d := range found {
		fmt.Fprintf(&b, "\t%s,\n", d.ref)
	}
	b.WriteString("}\n\n")
	b.WriteString("// dispatch is the run function of the command name names, false when\n// none is.\n")
	b.WriteString("func dispatch(name string) (command, bool) {\n\tswitch name {\n")
	for _, d := range found {
		fmt.Fprintf(&b, "\tcase %q:\n\t\treturn run%s, true\n", d.name, pascal(d.name))
	}
	b.WriteString("\t}\n\treturn nil, false\n}\n")
	return format.Source([]byte(b.String()))
}

// pascal is name in Pascal case, its hyphens dropped: git-shim's GitShim.
func pascal(name string) string {
	var b strings.Builder
	for word := range strings.SplitSeq(name, "-") {
		if word != "" {
			b.WriteString(strings.ToUpper(word[:1]) + word[1:])
		}
	}
	return b.String()
}
