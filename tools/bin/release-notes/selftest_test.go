// The upgrade asset's self-test (T-091's check, kept as a step in T-125, a Go test since T-143):
// a release's upgrading.json is its Upgrading section as data, what itos upgrade (slice-75) reads
// of each release between a project's pin and the new one, walking back by its previous. It
// proves the asset release-notes -json writes for the range since the last release up to HEAD, as
// the version tools/bin/release-version computes (the next patch when the range releases
// nothing):
//
//   - it is one JSON object of exactly schema (1), version (the version released), previous,
//     breaking, upgrading, changes and config, each list an array, never null;
//   - previous is the last release, X.Y.Z with no v, "" with none, which is what the release
//     workflow passes as -from (tools/bin/release-version's last=);
//   - upgrading is every Upgrading footer itos commit footers Upgrading <last release> <end>
//     --json lists (those saying none left out), in its order, each {commit, header, text} its
//     sha, its subject and its text, that text continued by the lines a wrapped footer runs onto;
//   - changes is every Changes entry itos commit footers Changes lists, the same way;
//   - breaking is every commit of the range marked breaking (a BREAKING-CHANGE: or BREAKING
//     CHANGE: footer in its message's last paragraph, or a ! in its header), oldest first, each
//     with its header and what it asks;
//   - config is every finding of go run ./tools/bin/schema-contract -json -release <last
//     release>, in its order, each {key, change} its path and its change; none with no last
//     release.
//
// It needs the network: the schema contract downloads the last release's itos.schema.json.
package main_test

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
)

type entry struct {
	Commit string `json:"commit"`
	Header string `json:"header"`
	Text   string `json:"text"`
}

type asset struct {
	Schema    int     `json:"schema"`
	Version   string  `json:"version"`
	Previous  string  `json:"previous"`
	Breaking  []entry `json:"breaking"`
	Upgrading []entry `json:"upgrading"`
	Changes   []entry `json:"changes"`
	Config    []struct {
		Key    string `json:"key"`
		Change string `json:"change"`
	} `json:"config"`
}

var space = regexp.MustCompile(`\s+`)

// flat is text with its whitespace aside: a wrapped footer's lines are joined with newlines.
func flat(text string) string { return strings.TrimSpace(space.ReplaceAllString(text, " ")) }

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	root := scratch.Root(t)
	env := scratch.OutsideEnv()
	sh := func(t *testing.T, name string, args ...string) string {
		t.Helper()
		r := scratch.Run(root, env, "", name, args...)
		if r.Code != 0 {
			t.Fatalf("%s %s (exit %d): %s", name, strings.Join(args, " "), r.Code, r.Stderr)
		}
		return r.Stdout
	}

	// The version and range judged: this tree's, as the release workflow would cut it.
	got := map[string]string{}
	for _, line := range strings.Split(sh(t, "go", "run", "./tools/bin/release-version"), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			got[k] = v
		}
	}
	end, last, version := "HEAD", got["last"], got["next"]
	if version == "" {
		if last == "" {
			version = "0.1.0"
		} else {
			parts := strings.Split(strings.TrimPrefix(last, "v"), ".")
			patch, err := strconv.Atoi(parts[2])
			if err != nil {
				t.Fatalf("the last release %s is not vX.Y.Z", last)
			}
			version = parts[0] + "." + parts[1] + "." + strconv.Itoa(patch+1)
		}
	}

	written := sh(t, "go", "run", "./tools/bin/release-notes", "-json", "-version", version, "-to", end)
	var a asset
	if err := json.Unmarshal([]byte(written), &a); err != nil {
		t.Fatalf("release-notes -json wrote no JSON: %v", err)
	}

	t.Run("it is one object of exactly its keys", func(t *testing.T) {
		dec := json.NewDecoder(strings.NewReader(written))
		var keys []string
		dec.Token() // {
		for dec.More() {
			k, err := dec.Token()
			if err != nil {
				t.Fatal(err)
			}
			keys = append(keys, k.(string))
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				t.Fatal(err)
			}
		}
		want := []string{"schema", "version", "previous", "breaking", "upgrading", "changes", "config"}
		if !slices.Equal(keys, want) {
			t.Errorf("its keys are %v, not %v", keys, want)
		}
	})
	t.Run("its schema is 1", func(t *testing.T) {
		if a.Schema != 1 {
			t.Errorf("its schema is %d, not 1", a.Schema)
		}
	})
	t.Run("its version is the version released", func(t *testing.T) {
		if a.Version != version {
			t.Errorf("its version is %s, not %s", a.Version, version)
		}
	})
	t.Run("its previous is the last release", func(t *testing.T) {
		if a.Previous != strings.TrimPrefix(last, "v") {
			t.Errorf("its previous is %q, not the last release %q", a.Previous, strings.TrimPrefix(last, "v"))
		}
	})
	t.Run("its lists are arrays", func(t *testing.T) {
		var raw map[string]json.RawMessage
		json.Unmarshal([]byte(written), &raw)
		for _, list := range []string{"breaking", "upgrading", "changes", "config"} {
			if !strings.HasPrefix(strings.TrimSpace(string(raw[list])), "[") {
				t.Errorf("its %s is not a list: %s", list, raw[list])
			}
		}
	})

	// A list of entries held to what a footer listing says, in its order.
	sameFooters := func(t *testing.T, name, footer string, got []entry) {
		if last == "" {
			if len(got) != 0 {
				t.Errorf("%s lists %d with no last release", name, len(got))
			}
			return
		}
		var listed struct {
			Footers []struct{ Sha, Subject, Text string } `json:"footers"`
		}
		out := sh(t, filepath.Join(root, "tools", "bin", "itos"), "commit", "footers", footer, last, end, "--json")
		if err := json.Unmarshal([]byte(out), &listed); err != nil {
			t.Fatal(err)
		}
		if len(got) != len(listed.Footers) {
			t.Errorf("%s lists %d, itos commit footers %s %s %s %d", name, len(got), footer, last, end, len(listed.Footers))
		}
		for i, f := range listed.Footers {
			if i >= len(got) || got[i].Commit != f.Sha || got[i].Header != f.Subject || !strings.HasPrefix(flat(got[i].Text), flat(f.Text)) {
				t.Errorf("%s[%d] is not %s's %s footer: %s", name, i, f.Sha[:7], footer, f.Text)
			}
		}
		for _, e := range got {
			if strings.ToLower(strings.TrimSpace(e.Text)) == "none" {
				t.Errorf("%s keeps %s's none", name, e.Commit[:7])
			}
		}
	}
	t.Run("upgrading is every Upgrading footer of the range", func(t *testing.T) {
		sameFooters(t, "upgrading", "Upgrading", a.Upgrading)
	})
	t.Run("changes is every Changes entry of the range", func(t *testing.T) {
		sameFooters(t, "changes", "Changes", a.Changes)
	})

	// The range's breaking commits, oldest first: its marked footer or its ! header.
	t.Run("breaking is every breaking commit of the range, each saying what it asks", func(t *testing.T) {
		span := end
		if last != "" {
			span = last + ".." + end
		}
		log := sh(t, scratch.Git(), "log", "--reverse", "--format=%H%x1f%B%x1e", span)
		bang := regexp.MustCompile(`^[a-zA-Z]+(\([^)]*\))?!: `)
		marked := regexp.MustCompile(`(?m)^BREAKING[- ]CHANGE: `)
		var want [][2]string
		for _, record := range strings.Split(log, "\x1e") {
			sha, message, ok := strings.Cut(strings.TrimLeft(record, "\n"), "\x1f")
			if !ok || sha == "" {
				continue
			}
			text := strings.TrimSpace(message)
			header, rest, multi := strings.Cut(text, "\n")
			footers := ""
			if multi {
				paragraphs := strings.Split(strings.TrimSpace(rest), "\n\n")
				footers = paragraphs[len(paragraphs)-1]
			}
			if bang.MatchString(header) || marked.MatchString(footers) {
				want = append(want, [2]string{sha, header})
			}
		}
		var gotPairs [][2]string
		for _, b := range a.Breaking {
			gotPairs = append(gotPairs, [2]string{b.Commit, b.Header})
			if strings.TrimSpace(b.Text) == "" {
				t.Errorf("breaking %s says nothing it asks", b.Commit[:7])
			}
		}
		if !slices.Equal(gotPairs, want) {
			t.Errorf("breaking lists %v, the range's breaking commits are %v", gotPairs, want)
		}
	})

	t.Run("config is every finding of the schema contract", func(t *testing.T) {
		if last == "" {
			if len(a.Config) != 0 {
				t.Errorf("config lists %d with no last release", len(a.Config))
			}
			return
		}
		var found struct {
			Findings []struct{ Path, Change string } `json:"findings"`
		}
		if err := json.Unmarshal([]byte(sh(t, "go", "run", "./tools/bin/schema-contract", "-json", "-release", last)), &found); err != nil {
			t.Fatal(err)
		}
		if len(found.Findings) != len(a.Config) {
			t.Fatalf("config is %+v, schema-contract -release %s finds %+v", a.Config, last, found.Findings)
		}
		for i, f := range found.Findings {
			if a.Config[i].Key != f.Path || a.Config[i].Change != f.Change {
				t.Errorf("config[%d] is %+v, schema-contract -release %s finds %+v", i, a.Config[i], last, f)
			}
		}
	})
}
