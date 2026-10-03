package cli

import (
	"strings"
	"testing"
)

func TestParsePluginFlag(t *testing.T) {
	for _, c := range []struct {
		args  []string
		read  int
		flag  pluginFlag
		error string
	}{
		{[]string{"--plugin"}, 1, pluginFlag{true, ""}, ""},
		{[]string{"--plugin", "--stealth"}, 1, pluginFlag{true, ""}, ""},
		{[]string{"--plugin", "user"}, 2, pluginFlag{true, "user"}, ""},
		{[]string{"--plugin=no"}, 1, pluginFlag{true, "no"}, ""},
		{[]string{"--plugin", "everywhere"}, 0, pluginFlag{true, ""}, "--plugin takes project, user, local or no (everywhere)"},
		{[]string{"--plugin=all"}, 0, pluginFlag{}, "--plugin takes project, user, local or no (all)"},
		{[]string{"--stealth"}, 0, pluginFlag{}, ""},
	} {
		var f pluginFlag
		n, err := parsePluginFlag(c.args, 0, &f)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if n != c.read || f != c.flag || got != c.error {
			t.Errorf("%q: read %d, %+v, %q; want %d, %+v, %q", c.args, n, f, got, c.read, c.flag, c.error)
		}
	}
}

func TestInstalledPlugin(t *testing.T) {
	for _, c := range []struct {
		list      string
		installed bool
		scope     string
		error     bool
	}{
		{`[]`, false, "", false},
		{`[{"id":"itos@itos","scope":"project","enabled":false}]`, false, "", false},
		{`[{"id":"other@itos","scope":"user","enabled":true},{"id":"itos@itos","scope":"local","enabled":true}]`, true, "local", false},
		{`not json`, false, "", true},
	} {
		e, ok, err := installedPlugin([]byte(c.list))
		if ok != c.installed || e.Scope != c.scope || (err != nil) != c.error {
			t.Errorf("%s: %v %q %v", c.list, ok, e.Scope, err)
		}
	}
}

func TestPluginQuestion(t *testing.T) {
	for _, c := range []struct {
		stealth bool
		input   string
		answer  string
	}{
		{false, "\n", "project"},
		{true, "\n", "local"},
		{false, "User\n", "user"},
		{true, "project\nno\n", "no"},
		{false, "maybe\nlocal", "local"},
		{false, "", ""},
	} {
		var log strings.Builder
		p := pluginOffer{stealth: c.stealth, log: &log, answers: strings.NewReader(c.input)}
		if got := p.question(); got != c.answer {
			t.Errorf("%q (stealth %v): %q, want %q\n%s", c.input, c.stealth, got, c.answer, log.String())
		}
	}
}
