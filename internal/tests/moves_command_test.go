package tests

import (
	"reflect"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/value"
)

func title(s string) *string { return &s }

// A command kind's listing at the start of a commit: a live test with a
// title, a wip one, and a live one the adapter gives no title.
var listedBefore = List{Protocol: 1, Tests: []Test{
	{ID: "ID-A-01", File: "a.test.ts", Live: true, Title: title("starts in the clearing")},
	{ID: "ID-A-02", File: "a.test.ts", Live: false, Title: title("ends")},
	{ID: "ID-A-03", File: "b.test.ts", Live: true},
}}

// with is listedBefore with the test of the ID replaced, or dropped when
// the replacement has no ID, and the extra tests after it.
func with(id string, replaced Test, extra ...Test) List {
	list := List{Protocol: 1}
	for _, t := range listedBefore.Tests {
		switch {
		case t.ID != id:
			list.Tests = append(list.Tests, t)
		case replaced.ID != "":
			list.Tests = append(list.Tests, replaced)
		}
	}
	list.Tests = append(list.Tests, extra...)
	return list
}

func TestListedProblems(t *testing.T) {
	renames := map[string]string{"ID-A-01": "starts in the glade"}
	for _, c := range []struct {
		name  string
		after List
		want  []string
	}{
		{"nothing changed", listedBefore, nil},
		{"a live test moved to another file",
			with("ID-A-01", Test{ID: "ID-A-01", File: "c.test.ts", Live: true, Title: title("starts in the clearing")}), nil},
		{"a wip test added and one removed",
			with("ID-A-02", Test{}, Test{ID: "ID-A-04", File: "a.test.ts"}), nil},
		{"a live test turned wip",
			with("ID-A-01", Test{ID: "ID-A-01", File: "a.test.ts", Title: title("starts in the clearing")}),
			[]string{"turns the live test @ID-A-01 of a.test.ts wip"}},
		{"a wip test made live",
			with("ID-A-02", Test{ID: "ID-A-02", File: "a.test.ts", Live: true, Title: title("ends")}),
			[]string{"brings the test @ID-A-02 live in a.test.ts, which needs the check's code set to be allowed outside except_types"}},
		{"a live test retitled",
			with("ID-A-01", Test{ID: "ID-A-01", File: "a.test.ts", Live: true, Title: title("starts in the meadow")}),
			[]string{`retitles the live test @ID-A-01 of a.test.ts from "starts in the clearing" to "starts in the meadow": a live test keeps its title`}},
		{"a retitle allowed_renames lists",
			with("ID-A-01", Test{ID: "ID-A-01", File: "a.test.ts", Live: true, Title: title("starts in the glade")}), nil},
		{"a title given where the adapter gave none",
			with("ID-A-03", Test{ID: "ID-A-03", File: "b.test.ts", Live: true, Title: title("crosses")}), nil},
		{"a live test added and one lost",
			with("ID-A-03", Test{}, Test{ID: "ID-A-05", File: "b.test.ts", Live: true}),
			[]string{"brings the test @ID-A-05 live in b.test.ts, which needs the check's code set to be allowed outside except_types", "loses the live test @ID-A-03 of b.test.ts"}},
	} {
		if got := ListedProblems(listedBefore, c.after, "@", renames, Live{}); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// A test going live on a command kind (slice 134), a wip one made live or a
// new live one, passes when the check has code and the footer names it and
// the commit touches none of the code.
func TestListedGoingLive(t *testing.T) {
	named := Live{Code: true, Known: true, Named: map[string]bool{"ID-A-02": true, "ID-A-05": true}, Footer: "Scenarios:"}
	made := with("ID-A-02", Test{ID: "ID-A-02", File: "a.test.ts", Live: true, Title: title("ends")},
		Test{ID: "ID-A-05", File: "b.test.ts", Live: true})
	if got := ListedProblems(listedBefore, made, "@", nil, named); got != nil {
		t.Errorf("named, no code touched: %q", got)
	}
	named.Named = map[string]bool{"ID-A-02": true}
	want := []string{"brings the test @ID-A-05 live in b.test.ts without naming it in Scenarios:"}
	if got := ListedProblems(listedBefore, made, "@", nil, named); !reflect.DeepEqual(got, want) {
		t.Errorf("one unnamed: %q, want %q", got, want)
	}
}

// A merge is judged by its own paths: a live test the second parent added
// in a file the merge did not change is not the merge's.
func TestOwnList(t *testing.T) {
	root := "src"
	k := config.Kind{Root: &root}
	parent := List{Protocol: 1, Tests: []Test{{ID: "ID-A-01", File: "a.test.ts", Live: true}}}
	merged := List{Protocol: 1, Tests: []Test{
		{ID: "ID-A-01", File: "a.test.ts", Live: false},
		{ID: "ID-B-01", File: "b.test.ts", Live: true},
	}}
	if got := ListedProblems(ownList(k, parent, merged, []string{"src/c.test.ts"}), merged, "@", nil, Live{}); got != nil {
		t.Errorf("a merge whose own paths hold no test: %q", got)
	}
	got := ListedProblems(ownList(k, parent, merged, []string{"src/a.test.ts"}), merged, "@", nil, Live{})
	if want := []string{"turns the live test @ID-A-01 of a.test.ts wip"}; !reflect.DeepEqual(got, want) {
		t.Errorf("a merge that turned a test wip in its own path: %q, want %q", got, want)
	}
}

// The protocol's title is optional, and a string when given.
func TestProtocolTitle(t *testing.T) {
	parse := func(text string) (List, string) {
		t.Helper()
		raw, err := value.ParseJSON(text)
		if err != nil {
			t.Fatal(err)
		}
		return protocolList(raw)
	}
	list, why := parse(`{"protocol":1,"files":["a"],"tests":[{"id":"x","file":"a","live":true,"title":"opens"},{"id":"y","file":"a","live":false}]}`)
	if why != "" || list.Tests[0].Title == nil || *list.Tests[0].Title != "opens" || list.Tests[1].Title != nil {
		t.Errorf("titles: %+v, %q", list.Tests, why)
	}
	if _, why := parse(`{"protocol":1,"files":[],"tests":[{"id":"x","file":"a","live":true,"title":3}]}`); why == "" {
		t.Error("a title that is not a string was read")
	}
}
