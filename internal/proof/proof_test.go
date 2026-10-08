package proof

import (
	"slices"
	"testing"
)

func TestTouches(t *testing.T) {
	globs := []string{"{cmd,internal}/**/*.go", "src/**"}
	cases := []struct {
		files []string
		want  bool
	}{
		{nil, false},
		{[]string{"README.md", "docs/a.md"}, false},
		{[]string{"README.md", "internal/cli/x.go"}, true},
		{[]string{"cmd/itos/main.go"}, true},
		{[]string{"src/app.go"}, true},
		{[]string{"internal/cli/x.md"}, false},
	}
	for _, c := range cases {
		got, err := Touches(globs, c.files)
		if err != nil || got != c.want {
			t.Errorf("Touches(%q) = %v, %v; want %v", c.files, got, err, c.want)
		}
	}
	if _, err := Touches([]string{"?src"}, []string{"src/a"}); err == nil {
		t.Error("a glob no regular expression can be made of: want an error")
	}
}

func TestCommand(t *testing.T) {
	got := Command("cc check --since {base} --base={base} --json", "abc")
	if want := "cc check --since abc --base=abc --json"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRead(t *testing.T) {
	refused := `{"schema":1,"ok":false,"problems":[{"rule":"mutation.survived","message":"a survived","fix":"test it","function":"a.F","line":3}]}`
	cases := []struct {
		name     string
		code     int
		stdout   string
		outcome  Outcome
		problems []Problem
		why      string
	}{
		{"exit 0 passes whatever it prints", 0, "not json", Passed, nil, ""},
		{"exit 1 with the object refuses with its problems", 1, "\n" + refused + "\n", Refused,
			[]Problem{{Rule: "mutation.survived", Message: "a survived", Fix: "test it"}}, ""},
		{"exit 1 with no problems still refuses", 1, `{"schema":1,"ok":false}`, Refused, nil, ""},
		{"exit 1 with no object cannot run", 1, "boom", CannotRun, nil, "exited 1 without the --json object that names its problems"},
		{"exit 1 with another schema cannot run", 1, `{"schema":2,"ok":false,"problems":[]}`, CannotRun, nil,
			"exited 1 without the --json object that names its problems"},
		{"exit 1 with no ok cannot run", 1, `{"schema":1,"problems":[]}`, CannotRun, nil,
			"exited 1 without the --json object that names its problems"},
		{"exit 1 that says ok cannot run", 1, `{"schema":1,"ok":true}`, CannotRun, nil,
			"exited 1 without the --json object that names its problems"},
		{"exit 2 cannot run, its problems kept", 2, `{"schema":1,"ok":false,"problems":[{"rule":"config.invalid","message":"bad"}]}`,
			CannotRun, []Problem{{Rule: "config.invalid", Message: "bad"}}, "exited 2"},
		{"exit 3 cannot run", 3, "", CannotRun, nil, "exited 3"},
		{"exit 70 cannot run", 70, "", CannotRun, nil, "exited 70"},
		{"exit 75 cannot run", 75, "", CannotRun, nil, "exited 75"},
		{"exit 127 cannot run", 127, "", CannotRun, nil, "exited 127"},
	}
	for _, c := range cases {
		v := Read(c.code, []byte(c.stdout))
		if v.Outcome != c.outcome || !slices.Equal(v.Problems, c.problems) || v.Why != c.why {
			t.Errorf("%s: got %+v", c.name, v)
		}
	}
}
