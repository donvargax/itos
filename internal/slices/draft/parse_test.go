package draft

import (
	"errors"
	"reflect"
	"testing"

	domain "github.com/donvargax/itos/v8/internal/draft"
	"github.com/donvargax/itos/v8/internal/out"
)

// Parse reads each subcommand's line into its Query, or a usage error
// saying what is wrong with it.
func TestParse(t *testing.T) {
	for _, c := range []struct {
		args  []string
		query Query
		usage string
	}{
		{args: nil, query: Query{Origin: "sub"}},
		{args: []string{"bogus"}, usage: "draft has no subcommand bogus: it takes add, edit, promote or drop, else nothing"},
		{args: []string{"add", "x", "-m", "docs: x", "a", "b"}, query: Query{Sub: "add", Draft: domain.Draft{ID: "x", Message: "docs: x"}, Paths: []string{"a", "b"}, Origin: "sub"}},
		{args: []string{"add", "x", "--message", "docs: x", "a"}, query: Query{Sub: "add", Draft: domain.Draft{ID: "x", Message: "docs: x"}, Paths: []string{"a"}, Origin: "sub"}},
		{args: []string{"add", "x", "--", "work", "add", "-m"}, query: Query{Sub: "add", Draft: domain.Draft{ID: "x", Command: []string{"work", "add", "-m"}}, Paths: []string{}, Origin: "sub"}},
		{args: []string{"add", "x", "-m"}, usage: "draft add -m needs a value"},
		{args: []string{"add", "x", "--message"}, usage: "draft add --message needs a value"},
		{args: []string{"add"}, usage: "draft add needs <id> -m <message> <path>…, or <id> -- <itos args>…"},
		{args: []string{"add", "-m", "docs: x"}, usage: "draft add needs <id> -m <message> <path>…, or <id> -- <itos args>…"},
		{args: []string{"add", "a b", "--", "work"}, usage: `draft add: "a b" is no draft id (letters, digits, '.', '_' and '-', a letter or digit first)`},
		{args: []string{"add", "x", "-m", "a", "-m", "b", "p"}, usage: "draft add takes one -m <message>"},
		{args: []string{"add", "x", "-m", "a", "--", "work"}, usage: "draft add takes -m <message> <path>… or -- <itos args>…, not both"},
		{args: []string{"add", "x", "p", "--", "work"}, usage: "draft add takes -m <message> <path>… or -- <itos args>…, not both"},
		{args: []string{"add", "x", "--"}, usage: "draft add x -- needs the itos arguments to run"},
		{args: []string{"add", "x", "--", "draft", "drop", "y"}, usage: "draft add: a draft cannot run itos draft"},
		{args: []string{"add", "x", "-m", " ", "p"}, usage: "draft add x needs -m <message> and the paths whose change it keeps"},
		{args: []string{"add", "x", "-m", "docs: x"}, usage: "draft add x needs -m <message> and the paths whose change it keeps"},
		{args: []string{"edit", "x", "-m", "docs: x", "a", "a"}, query: Query{Sub: "edit", Draft: domain.Draft{ID: "x", Message: "docs: x"}, Paths: []string{"a", "a"}, Origin: "sub"}},
		{args: []string{"edit", "x", "--message", "docs: x", "a"}, query: Query{Sub: "edit", Draft: domain.Draft{ID: "x", Message: "docs: x"}, Paths: []string{"a"}, Origin: "sub"}},
		{args: []string{"edit", "x", "-m"}, usage: "draft edit -m needs a value"},
		{args: []string{"edit", "x", "--message"}, usage: "draft edit --message needs a value"},
		{args: []string{"edit"}, usage: "draft edit needs <id> -m <message> <path>…"},
		{args: []string{"edit", "a b", "-m", "docs: x", "p"}, usage: `draft edit: "a b" is no draft id (letters, digits, '.', '_' and '-', a letter or digit first)`},
		{args: []string{"edit", "x", "-m", "a", "-m", "b", "p"}, usage: "draft edit takes one -m <message>"},
		{args: []string{"edit", "x", "-m", " ", "p"}, usage: "draft edit x needs -m <message> and the paths to edit"},
		{args: []string{"edit", "x", "-m", "docs: x"}, usage: "draft edit x needs -m <message> and the paths to edit"},
		{args: []string{"promote"}, query: Query{Sub: "promote", Origin: "sub"}},
		{args: []string{"promote", "x", "y"}, usage: "draft promote takes no arguments: x y"},
		{args: []string{"drop", "x"}, query: Query{Sub: "drop", Draft: domain.Draft{ID: "x"}, Origin: "sub"}},
		{args: []string{"drop"}, usage: "draft drop needs one <id>"},
		{args: []string{"drop", "x", "y"}, usage: "draft drop needs one <id>"},
		{args: []string{"drop", "a b"}, usage: `draft drop: "a b" is no draft id`},
	} {
		c0, err := Parse(c.args, "sub", true, true)
		var u out.UsageError
		switch {
		case c.usage != "":
			if !errors.As(err, &u) || err.Error() != c.usage {
				t.Errorf("Parse %q: %v, not the usage error %q", c.args, err, c.usage)
			}
		case err != nil || !reflect.DeepEqual(c0.Query, c.query) || !c0.JSON || !c0.Quiet:
			t.Errorf("Parse %q = %+v, %v; want %+v", c.args, c0, err, c.query)
		}
	}
}
