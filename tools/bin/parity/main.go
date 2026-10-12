// Command parity holds a Go replacement of one of this repository's TypeScript tools to what
// the TypeScript said (T-143 to T-147, the user's call of 2026-10-10): before a tool is deleted,
// the first commit of the task that replaces it records what it says under tools/parity/<tool>/,
// and the task's done check runs parity <tool>, which runs the replacement and compares what it
// says with that record, so a port that drops a case, or says another verdict, fails. T-149
// deletes this program and tools/parity/ once the last of those tasks is done.
//
//	go run ./tools/bin/parity [-j <n>] <tool>
//
// # The record
//
// tools/parity/<tool>/ holds one record per file, <name>.json, read in name order. A record is a
// JSON object:
//
//		{
//		  "was": "node tools/selftest/ci-scope.ts",
//		  "run": ["go", "test", "-count=1", "-json", "-run", "^TestSelftest$", "./tools/selftest/ci-scope"],
//		  "env": ["ITOS_SELFTEST=1"],
//		  "read": "go-test",
//		  "test": "TestSelftest",
//		  "cases": [{ "name": "a range with no start runs everything", "verdict": "pass" }]
//		}
//
//	  - was: the TypeScript the record was taken from, for people; parity does not run it.
//	  - run: the replacement's command, its words as given (no shell), run at the repository's top.
//	  - dir: the folder the command runs in instead, relative to the repository's top; optional.
//	  - env: NAME=value entries laid over parity's own environment for that command. A value's
//	    $NAME and ${NAME} are expanded from parity's environment, and $PARITY_ROOT is the
//	    repository's top folder, so a record can put a fake program first on PATH (T-146).
//	  - read: how the command's stdout is read, one of the readers below.
//
// Each reader has fields of its own:
//
//   - go-test: the stdout is go test -json's events (so run must hold -json). test names the top
//     test, and cases is every subtest directly under it, each its name as the TypeScript's case
//     was named and its verdict (pass, fail or skip). The names are compared as go test writes
//     them, a space an underscore, so a case name holds no slash, which would nest it. A case run
//     and not recorded, one recorded and not run, and one whose verdict differs each fail,
//     named; so does a top test that never ran.
//   - stdout: the stdout is compared byte for byte with the file the record's stdout names,
//     beside the record (a recorded output), and the command must exit 0.
//   - verdicts: the stdout is JSON lines, one object a case, {"name": "…", "verdict": "pass"},
//     as the conformance runner's --json writes them (T-144); a line that is not such an object
//     is passed over. cases is every case, its name as the TypeScript named it, a slash allowed,
//     and its verdict (pass, fail or skip), judged as go-test's are: a case run and not recorded,
//     one recorded and not run, and one whose verdict differs each fail, named; so does a command
//     that says no verdict at all.
//   - exit: the command's exit code is compared with the record's exit, and its stdout, with the
//     repository's top folder written <root>, byte for byte with the file the record's stdout
//     names, beside the record (T-146's programs, whose usage errors and failures say only their
//     code on stdout). Its stderr is not compared: it names the command, which the port changes.
//
// A tool whose replacement says something else adds a reader here, with its fields, before its
// first record uses it.
//
// The records run at once, -j at a time (4 by default), each printed with its verdict, a failing
// one with what differs and the output of a case that failed. Exit 0 when every record holds, 1
// when one does not, 2 for a usage error, a tool with no records or a record parity cannot read.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

// record is one file of tools/parity/<tool>/.
type record struct {
	file    string
	Was     string   `json:"was"`
	Run     []string `json:"run"`
	Env     []string `json:"env"`
	Read    string   `json:"read"`
	Test    string   `json:"test"`
	Cases   []kase   `json:"cases"`
	Stdout  string   `json:"stdout"`
	Dir     string   `json:"dir"`
	Exit    *int     `json:"exit"`
	root    string   // the repository's top folder
	verdict []string // what differs, empty when the record holds
	notes   []string // the output of what failed
}

// kase is a recorded case.
type kase struct {
	Name    string `json:"name"`
	Verdict string `json:"verdict"`
}

// readers is how a record's command is judged, by its read.
var readers = map[string]func(r *record, stdout []byte, code int, dir string){
	"go-test":  readGoTest,
	"stdout":   readStdout,
	"verdicts": readVerdicts,
	"exit":     readExit,
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("parity", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jobs := flags.Int("j", 4, "how many records run at once")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 || *jobs < 1 {
		fmt.Fprintln(stderr, "usage: go run ./tools/bin/parity [-j <n>] <tool>")
		return 2
	}
	root, err := top()
	if err != nil {
		fmt.Fprintf(stderr, "parity: %v\n", err)
		return 2
	}
	tool := flags.Arg(0)
	dir := filepath.Join(root, "tools", "parity", tool)
	records, err := load(dir)
	if err != nil {
		fmt.Fprintf(stderr, "parity: %v\n", err)
		return 2
	}
	work := make(chan *record)
	var wg sync.WaitGroup
	for i := 0; i < *jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range work {
				judge(r, root, dir)
			}
		}()
	}
	for _, r := range records {
		work <- r
	}
	close(work)
	wg.Wait()

	failed := 0
	for _, r := range records {
		if len(r.verdict) == 0 {
			fmt.Fprintf(stdout, "ok   %s/%s: %s\n", tool, r.file, held(r))
			continue
		}
		failed++
		fmt.Fprintf(stdout, "FAIL %s/%s (%s, was %s):\n", tool, r.file, strings.Join(r.Run, " "), r.Was)
		for _, v := range r.verdict {
			fmt.Fprintf(stdout, "  - %s\n", v)
		}
		for _, n := range r.notes {
			fmt.Fprintf(stdout, "%s\n", indent(n))
		}
	}
	if failed > 0 {
		fmt.Fprintf(stdout, "\n%d of %d records of %s do not hold: the replacement says something else than tools/parity/%s/ recorded\n", failed, len(records), tool, tool)
		return 1
	}
	fmt.Fprintf(stdout, "\nparity %s: all %d records hold\n", tool, len(records))
	return 0
}

// held says what a record that holds held.
func held(r *record) string {
	if r.Read == "go-test" || r.Read == "verdicts" {
		return fmt.Sprintf("%d cases as recorded", len(r.Cases))
	}
	if r.Read == "exit" {
		return fmt.Sprintf("its output and exit %d as recorded", *r.Exit)
	}
	return "its output as recorded"
}

// top is the repository's top folder: the nearest folder above the working one that holds go.mod.
func top() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod above the working folder")
		}
		dir = parent
	}
}

// load reads every record of dir, in name order, refusing one it cannot judge.
func load(dir string) ([]*record, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s holds no record", dir)
	}
	sort.Strings(files)
	var records []*record
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		r := &record{file: filepath.Base(file)}
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(r); err != nil {
			return nil, fmt.Errorf("%s: %v", file, err)
		}
		if err := check(r); err != nil {
			return nil, fmt.Errorf("%s: %v", file, err)
		}
		records = append(records, r)
	}
	return records, nil
}

// check refuses a record parity cannot judge.
func check(r *record) error {
	if len(r.Run) == 0 {
		return errors.New("run names no command")
	}
	if _, ok := readers[r.Read]; !ok {
		return fmt.Errorf("read %q is no reader of parity's (go-test, stdout, verdicts, exit)", r.Read)
	}
	for _, kv := range r.Env {
		if !strings.Contains(kv, "=") {
			return fmt.Errorf("env %q is not NAME=value", kv)
		}
	}
	switch r.Read {
	case "go-test":
		if r.Test == "" || len(r.Cases) == 0 {
			return errors.New("a go-test record names its test and its cases")
		}
		return checkCases(r.Cases, subtest, true)
	case "verdicts":
		if len(r.Cases) == 0 {
			return errors.New("a verdicts record names its cases")
		}
		return checkCases(r.Cases, func(name string) string { return name }, false)
	case "stdout":
		if r.Stdout == "" {
			return errors.New("a stdout record names the file of its output")
		}
	case "exit":
		if r.Stdout == "" || r.Exit == nil {
			return errors.New("an exit record names the file of its output and its exit code")
		}
	}
	return nil
}

// checkCases refuses a case with no name, a slash in its name where slash says a name cannot
// hold one, a verdict that is not pass, fail or skip, and two cases whose names key alike.
func checkCases(cases []kase, key func(string) string, noSlash bool) error {
	seen := map[string]bool{}
	for _, c := range cases {
		switch {
		case c.Name == "":
			return errors.New("a case's name is not empty")
		case noSlash && strings.Contains(c.Name, "/"):
			return fmt.Errorf("case %q: a case's name is not empty and holds no slash", c.Name)
		case c.Verdict != "pass" && c.Verdict != "fail" && c.Verdict != "skip":
			return fmt.Errorf("case %q: verdict %q is not pass, fail or skip", c.Name, c.Verdict)
		case seen[key(c.Name)]:
			return fmt.Errorf("case %q is recorded twice", c.Name)
		}
		seen[key(c.Name)] = true
	}
	return nil
}

// judge runs the record's command and reads what it says.
func judge(r *record, root, dir string) {
	cmd := exec.Command(r.Run[0], r.Run[1:]...)
	cmd.Dir = filepath.Join(root, filepath.FromSlash(r.Dir))
	cmd.Env = append(os.Environ(), expand(r.Env, root)...)
	r.root = root
	var out, errs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errs
	code := 0
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			r.verdict = append(r.verdict, fmt.Sprintf("%s did not start: %v", r.Run[0], err))
			return
		}
		code = exit.ExitCode()
	}
	readers[r.Read](r, out.Bytes(), code, dir)
	if len(r.verdict) > 0 && errs.Len() > 0 {
		r.notes = append(r.notes, "its stderr:\n"+errs.String())
	}
}

// expand is a record's env entries with $NAME and ${NAME} expanded from parity's environment,
// and $PARITY_ROOT the repository's top folder.
func expand(env []string, root string) []string {
	var out []string
	for _, kv := range env {
		out = append(out, os.Expand(kv, func(name string) string {
			if name == "PARITY_ROOT" {
				return root
			}
			return os.Getenv(name)
		}))
	}
	return out
}

// subtest is a case's name as go test writes a subtest's: a space an underscore, a character it
// cannot print quoted (testing's rewrite).
func subtest(name string) string {
	var b strings.Builder
	for _, c := range name {
		switch {
		case unicode.IsSpace(c):
			b.WriteByte('_')
		case !unicode.IsPrint(c):
			s := strconv.QuoteRune(c)
			b.WriteString(s[1 : len(s)-1])
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}

// event is one of go test -json's.
type event struct {
	Action string
	Test   string
	Output string
}

// readGoTest holds the subtests under the record's test to its cases.
func readGoTest(r *record, stdout []byte, code int, _ string) {
	ran := map[string]string{} // a subtest, its verdict
	outputs := map[string]*strings.Builder{}
	var order []string
	topRan, topVerdict := false, ""
	lines := bufio.NewScanner(bytes.NewReader(stdout))
	lines.Buffer(make([]byte, 1<<20), 64<<20)
	for lines.Scan() {
		var e event
		if !bytes.HasPrefix(lines.Bytes(), []byte("{")) || json.Unmarshal(lines.Bytes(), &e) != nil {
			continue
		}
		if e.Test == r.Test {
			topRan = true
			if e.Action == "pass" || e.Action == "fail" || e.Action == "skip" {
				topVerdict = e.Action
			}
			continue
		}
		name, ok := strings.CutPrefix(e.Test, r.Test+"/")
		if !ok || strings.Contains(name, "/") {
			continue
		}
		if outputs[name] == nil {
			outputs[name] = &strings.Builder{}
			order = append(order, name)
		}
		switch e.Action {
		case "pass", "fail", "skip":
			ran[name] = e.Action
		case "output":
			outputs[name].WriteString(e.Output)
		}
	}
	if !topRan {
		r.verdict = append(r.verdict, fmt.Sprintf("%s never ran (exit %d)", r.Test, code))
	} else if topVerdict == "skip" && len(ran) == 0 {
		r.verdict = append(r.verdict, fmt.Sprintf("%s was skipped, its cases not run", r.Test))
	}
	compareCases(r, subtest, ran, order, func(name string) string { return outputs[name].String() })
}

// compareCases holds what ran, each case by its key its verdict, keys in the order they ran, to
// the record's cases: a case recorded and not run, one whose verdict differs (its output a note)
// and one run and not recorded each fail, named.
func compareCases(r *record, key func(string) string, ran map[string]string, order []string, output func(string) string) {
	recorded := map[string]bool{}
	for _, c := range r.Cases {
		name := key(c.Name)
		recorded[name] = true
		got, ok := ran[name]
		switch {
		case !ok:
			r.verdict = append(r.verdict, fmt.Sprintf("case %q: recorded %s, not run", c.Name, c.Verdict))
		case got != c.Verdict:
			r.verdict = append(r.verdict, fmt.Sprintf("case %q: recorded %s, ran %s", c.Name, c.Verdict, got))
			if out := output(name); out != "" {
				r.notes = append(r.notes, out)
			}
		}
	}
	for _, name := range order {
		if !recorded[name] {
			if v, ok := ran[name]; ok {
				r.verdict = append(r.verdict, fmt.Sprintf("case %q: ran %s, not recorded", name, v))
			}
		}
	}
}

// verdictLine is one line of a verdicts record's stdout: a case and its verdict, and what the
// tool said of a case that failed.
type verdictLine struct {
	Name     string   `json:"name"`
	Verdict  string   `json:"verdict"`
	Problems []string `json:"problems"`
}

// readVerdicts holds the JSON lines of the command's stdout, a case each, to the record's cases.
func readVerdicts(r *record, stdout []byte, code int, _ string) {
	ran := map[string]string{}
	said := map[string]string{}
	var order []string
	lines := bufio.NewScanner(bytes.NewReader(stdout))
	lines.Buffer(make([]byte, 1<<20), 64<<20)
	for lines.Scan() {
		var v verdictLine
		if !bytes.HasPrefix(lines.Bytes(), []byte("{")) || json.Unmarshal(lines.Bytes(), &v) != nil || v.Name == "" || v.Verdict == "" {
			continue
		}
		if _, twice := ran[v.Name]; !twice {
			order = append(order, v.Name)
		}
		ran[v.Name] = v.Verdict
		said[v.Name] = strings.Join(v.Problems, "\n")
	}
	if len(ran) == 0 {
		r.verdict = append(r.verdict, fmt.Sprintf("it said no verdict (exit %d)", code))
		return
	}
	compareCases(r, func(name string) string { return name }, ran, order, func(name string) string { return said[name] })
}

// readStdout holds the command's stdout to the recorded output, byte for byte.
func readStdout(r *record, stdout []byte, code int, dir string) {
	if code != 0 {
		r.verdict = append(r.verdict, fmt.Sprintf("it exited %d", code))
	}
	sameOutput(r, stdout, dir)
}

// readExit holds the command's exit code to the recorded one, and its stdout, the repository's
// top folder written <root>, to the recorded output.
func readExit(r *record, stdout []byte, code int, dir string) {
	if code != *r.Exit {
		r.verdict = append(r.verdict, fmt.Sprintf("it exited %d, recorded %d", code, *r.Exit))
	}
	if r.root != "" {
		stdout = bytes.ReplaceAll(stdout, []byte(r.root), []byte("<root>"))
	}
	sameOutput(r, stdout, dir)
}

// sameOutput holds stdout to the record's recorded output, byte for byte.
func sameOutput(r *record, stdout []byte, dir string) {
	want, err := os.ReadFile(filepath.Join(dir, r.Stdout))
	if err != nil {
		r.verdict = append(r.verdict, fmt.Sprintf("its recorded output: %v", err))
		return
	}
	if !bytes.Equal(stdout, want) {
		at := 0
		for at < len(stdout) && at < len(want) && stdout[at] == want[at] {
			at++
		}
		line := 1 + bytes.Count(want[:at], []byte("\n"))
		r.verdict = append(r.verdict, fmt.Sprintf("its stdout (%d bytes) is not %s (%d bytes), from line %d on", len(stdout), r.Stdout, len(want), line))
	}
}

// indent sets text off under a record's lines.
func indent(text string) string {
	text = strings.TrimRight(text, "\n")
	return "      " + strings.ReplaceAll(text, "\n", "\n      ")
}
