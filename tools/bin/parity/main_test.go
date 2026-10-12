package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// events is go test -json's stdout for a top test and its subtests, each name its verdict.
func events(top string, verdicts map[string]string, order ...string) []byte {
	var b strings.Builder
	b.WriteString(`{"Action":"run","Test":"` + top + `"}` + "\n")
	for _, name := range order {
		b.WriteString(`{"Action":"run","Test":"` + top + `/` + name + `"}` + "\n")
		b.WriteString(`{"Action":"output","Test":"` + top + `/` + name + `","Output":"said ` + name + `\n"}` + "\n")
		b.WriteString(`{"Action":"` + verdicts[name] + `","Test":"` + top + `/` + name + `"}` + "\n")
	}
	b.WriteString(`{"Action":"pass","Test":"` + top + `"}` + "\n")
	return []byte(b.String())
}

func TestReadGoTest(t *testing.T) {
	recorded := func() *record {
		return &record{Read: "go-test", Test: "TestSelftest", Cases: []kase{
			{"a case passes", "pass"}, {"another case, too", "pass"},
		}}
	}
	for _, c := range []struct {
		name     string
		verdicts map[string]string
		order    []string
		want     []string
	}{
		{"every case as recorded holds", map[string]string{"a_case_passes": "pass", "another_case,_too": "pass"},
			[]string{"a_case_passes", "another_case,_too"}, nil},
		{"a case dropped fails, named", map[string]string{"a_case_passes": "pass"},
			[]string{"a_case_passes"}, []string{`case "another case, too": recorded pass, not run`}},
		{"a verdict changed fails, named", map[string]string{"a_case_passes": "fail", "another_case,_too": "pass"},
			[]string{"a_case_passes", "another_case,_too"}, []string{`case "a case passes": recorded pass, ran fail`}},
		{"a case not recorded fails, named", map[string]string{"a_case_passes": "pass", "another_case,_too": "pass", "a_new_one": "pass"},
			[]string{"a_case_passes", "another_case,_too", "a_new_one"}, []string{`case "a_new_one": ran pass, not recorded`}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := recorded()
			readGoTest(r, events("TestSelftest", c.verdicts, c.order...), 0, "")
			if !slices.Equal(r.verdict, c.want) {
				t.Errorf("verdict %q, want %q", r.verdict, c.want)
			}
		})
	}
	t.Run("a top test that never ran fails", func(t *testing.T) {
		r := recorded()
		readGoTest(r, []byte("# a build error\n"), 1, "")
		if len(r.verdict) == 0 || r.verdict[0] != "TestSelftest never ran (exit 1)" {
			t.Errorf("verdict %q", r.verdict)
		}
	})
	t.Run("a top test skipped fails", func(t *testing.T) {
		r := recorded()
		readGoTest(r, []byte(`{"Action":"run","Test":"TestSelftest"}`+"\n"+`{"Action":"skip","Test":"TestSelftest"}`+"\n"), 0, "")
		if len(r.verdict) == 0 || r.verdict[0] != "TestSelftest was skipped, its cases not run" {
			t.Errorf("verdict %q", r.verdict)
		}
	})
}

func TestReadStdout(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "out.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &record{Read: "stdout", Stdout: "out.txt"}
	readStdout(r, []byte("one\ntwo\n"), 0, dir)
	if len(r.verdict) != 0 {
		t.Errorf("the same output: verdict %q", r.verdict)
	}
	r = &record{Read: "stdout", Stdout: "out.txt"}
	readStdout(r, []byte("one\nthree\n"), 0, dir)
	if len(r.verdict) != 1 || !strings.Contains(r.verdict[0], "from line 2 on") {
		t.Errorf("another output: verdict %q", r.verdict)
	}
}

func TestReadVerdicts(t *testing.T) {
	recorded := func() *record {
		return &record{Read: "verdicts", Cases: []kase{{"a.yaml: one/two", "pass"}, {"b.yaml: three", "pass"}}}
	}
	line := func(name, verdict string) string {
		return `{"name":"` + name + `","verdict":"` + verdict + `","problems":["exit: expected 0, got 1"]}` + "\n"
	}
	for _, c := range []struct {
		name, stdout string
		want         []string
	}{
		{"every case as recorded holds", line("a.yaml: one/two", "pass") + "a summary\n" + line("b.yaml: three", "pass"), nil},
		{"a case dropped fails, named", line("a.yaml: one/two", "pass"), []string{`case "b.yaml: three": recorded pass, not run`}},
		{"a verdict changed fails, named", line("a.yaml: one/two", "fail") + line("b.yaml: three", "pass"),
			[]string{`case "a.yaml: one/two": recorded pass, ran fail`}},
		{"a case not recorded fails, named", line("a.yaml: one/two", "pass") + line("b.yaml: three", "pass") + line("c.yaml: new", "pass"),
			[]string{`case "c.yaml: new": ran pass, not recorded`}},
		{"no verdict at all fails", "FAIL conformance runner setup: no git\n", []string{"it said no verdict (exit 2)"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := recorded()
			readVerdicts(r, []byte(c.stdout), 2, "")
			if !slices.Equal(r.verdict, c.want) {
				t.Errorf("verdict %q, want %q", r.verdict, c.want)
			}
		})
	}
	t.Run("a verdict changed notes what the tool said", func(t *testing.T) {
		r := recorded()
		readVerdicts(r, []byte(line("a.yaml: one/two", "fail")+line("b.yaml: three", "pass")), 1, "")
		if len(r.notes) != 1 || r.notes[0] != "exit: expected 0, got 1" {
			t.Errorf("notes %q", r.notes)
		}
	})
}

func TestCheck(t *testing.T) {
	for _, c := range []struct {
		name string
		r    record
		want string
	}{
		{"no command", record{Read: "go-test"}, "run names no command"},
		{"an unknown reader", record{Run: []string{"x"}, Read: "lines"}, `read "lines" is no reader`},
		{"a case with a slash", record{Run: []string{"x"}, Read: "go-test", Test: "T", Cases: []kase{{"a/b", "pass"}}}, "holds no slash"},
		{"a case twice", record{Run: []string{"x"}, Read: "go-test", Test: "T", Cases: []kase{{"a b", "pass"}, {"a_b", "pass"}}}, "recorded twice"},
		{"a verdict unknown", record{Run: []string{"x"}, Read: "go-test", Test: "T", Cases: []kase{{"a", "ok"}}}, "is not pass, fail or skip"},
		{"no output file", record{Run: []string{"x"}, Read: "stdout"}, "names the file of its output"},
		{"no verdicts", record{Run: []string{"x"}, Read: "verdicts"}, "names its cases"},
		{"a verdicts case twice", record{Run: []string{"x"}, Read: "verdicts", Cases: []kase{{"a.yaml: a/b", "pass"}, {"a.yaml: a/b", "fail"}}}, "recorded twice"},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := check(&c.r)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("check said %v, want %q", err, c.want)
			}
		})
	}
}

func TestSubtest(t *testing.T) {
	if got := subtest("a module a day old is refused, and govulncheck not run"); got != "a_module_a_day_old_is_refused,_and_govulncheck_not_run" {
		t.Errorf("subtest said %q", got)
	}
}
