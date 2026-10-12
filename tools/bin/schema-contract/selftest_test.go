// The schema contract's self-test (T-070's check, kept as a step in T-125, a Go test since
// T-143), on schemas this test writes itself, in a scratch repository with a release tag, and no
// network:
//
//   - each breaking change (a key removed, a type narrowed, an enum value removed, a key newly
//     required, an object closed, a default changed) is refused, naming the key, the change and
//     the remedy, when no commit since the tag marks one as breaking; a ! before the tag does not
//     count;
//   - each passes with a commit since the tag that carries a BREAKING-CHANGE: footer, and with
//     one whose header has a !;
//   - each compatible change (a key added, a type widened, an enum value added) passes, and is
//     printed;
//   - -range-from checks nothing when the range has no feat or fix;
//   - the release's schema is downloaded from <release-url>/<tag>/ (here a local server), and a
//     download that fails (a 404, a closed port) stops the check with exit 2, never a pass; no
//     release tag passes, saying so; a shallow clone stops it.
//
// Every proxy variable points at a closed port, so nothing reaches the network.
package main_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
)

// schema is a JSON Schema as this test edits it.
type schema = map[string]any

// release is the release's schema, in the shape tools/bin/config-schema writes.
func release() schema {
	var s schema
	if err := json.Unmarshal([]byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "itos.yaml",
  "type": "object",
  "properties": {
    "version": { "description": "The config's format: 1.", "type": "number" },
    "shell": { "type": "array", "items": { "type": "string" }, "default": ["sh", "-c"] },
    "hooks": {
      "type": "object",
      "properties": {
        "manager": { "type": "string", "enum": ["husky", "lefthook", "vite-hooks"] },
        "bin": { "type": "string", "default": "tools/bin/itos" },
        "timeout": { "anyOf": [{ "type": "string" }, { "type": "number" }] }
      },
      "required": ["manager"],
      "additionalProperties": false
    },
    "tests": {
      "type": "object",
      "additionalProperties": {
        "type": "object",
        "properties": { "root": { "type": "string", "default": "features" } },
        "additionalProperties": false
      }
    },
    "notes": { "type": "object", "properties": { "text": { "type": "string" } } }
  },
  "required": ["version"],
  "additionalProperties": false
}`), &s); err != nil {
		panic(err)
	}
	return s
}

// at is the object at the path of keys under s.
func at(s schema, keys ...string) schema {
	for _, k := range keys {
		s = s[k].(schema)
	}
	return s
}

// changed is the release's schema with edit applied.
func changed(edit func(s schema)) schema {
	s := release()
	edit(s)
	return s
}

// change is one change to the release's schema, the key the check must name and the words its
// change must hold.
type change struct {
	what   string
	schema schema
	key    string
	words  string
}

var breaking = []change{
	{"a key removed", changed(func(s schema) { delete(at(s, "properties", "hooks", "properties"), "bin") }), "hooks.bin", "key removed"},
	{"a type narrowed", changed(func(s schema) { at(s, "properties", "hooks", "properties")["timeout"] = schema{"type": "string"} }),
		"hooks.timeout", "type narrowed: a number is no longer accepted"},
	{"an enum value removed", changed(func(s schema) {
		m := at(s, "properties", "hooks", "properties", "manager")
		m["enum"] = m["enum"].([]any)[1:]
	}), "hooks.manager", `enum value "husky" removed`},
	{"a key newly required", changed(func(s schema) {
		h := at(s, "properties", "hooks")
		h["required"] = append(h["required"].([]any), "bin")
	}), "hooks.bin", "key newly required"},
	{"an object closed", changed(func(s schema) { at(s, "properties", "notes")["additionalProperties"] = false }), "notes", "object closed"},
	{"a default changed", changed(func(s schema) { at(s, "properties", "shell")["default"] = []any{"bash", "-c"} }),
		"shell", `default changed from ["sh","-c"] to ["bash","-c"]`},
	{"a default changed in a map's entries", changed(func(s schema) {
		at(s, "properties", "tests", "additionalProperties", "properties", "root")["default"] = "spec"
	}), "tests.<key>.root", `default changed from "features" to "spec"`},
}

var compatible = []change{
	{"a key added", changed(func(s schema) { at(s, "properties", "hooks", "properties")["color"] = schema{"type": "boolean"} }),
		"hooks.color", "key added"},
	{"a type widened", changed(func(s schema) {
		at(s, "properties")["version"] = schema{"anyOf": []any{schema{"type": "number"}, schema{"type": "string"}}}
	}), "version", "type widened: a string is now accepted"},
	{"an enum value added", changed(func(s schema) {
		m := at(s, "properties", "hooks", "properties", "manager")
		m["enum"] = append(m["enum"].([]any), "overcommit")
	}), "hooks.manager", `enum value "overcommit" added`},
}

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	const closed = "http://127.0.0.1:9"
	rel := scratch.NewRelease(t, "GITHUB_REPOSITORY=example/itos",
		"HTTP_PROXY="+closed, "HTTPS_PROXY="+closed, "http_proxy="+closed, "https_proxy="+closed,
		"NO_PROXY=", "no_proxy=")
	tmp := rel.Tmp
	repo := filepath.Join(tmp, "repo")
	bin := scratch.Build(t, "schema-contract")
	write := func(name string, s schema) string {
		data, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(tmp, name)
		if err := os.WriteFile(file, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return file
	}
	releaseFile := write("release.json", release())
	check := func(dir string, args ...string) scratch.Result { return scratch.Run(dir, rel.Env, "", bin, args...) }

	// The scratch history: a breaking change before the release, the release's tag, then a feat
	// that marks nothing; branches add a commit that does.
	rel.Git(tmp, "init", "-q", "-b", "main", repo)
	rel.Commit(repo, "feat!: an old break, before the release")
	rel.Commit(repo, "build: version 1.0.0")
	rel.Git(repo, "tag", "v1.0.0")
	released := rel.Git(repo, "rev-parse", "HEAD")
	rel.Commit(repo, "feat: change the config\n\nNo breaking change said.\n\nTask: T-1")
	feat := rel.Git(repo, "rev-parse", "HEAD")
	rel.Git(repo, "checkout", "-q", "-b", "footer")
	rel.Commit(repo, "feat: change it again\n\nBody.\n\nBREAKING-CHANGE: rename hooks.bin in itos.yaml")
	rel.Git(repo, "checkout", "-q", "-b", "bang", feat)
	rel.Commit(repo, "fix!: change it again")
	rel.Git(repo, "checkout", "-q", "-b", "docs", released)
	rel.Commit(repo, "docs: say something")
	rel.Git(repo, "checkout", "-q", "main")
	on := func(branch string) { rel.Git(repo, "checkout", "-q", branch) }

	t.Run("the release's own schema passes with no change", func(t *testing.T) {
		r := check(repo, "-old", releaseFile, "-new", releaseFile)
		if r.Code != 0 || !strings.Contains(r.Output(), "no change") {
			t.Errorf("the release's own schema should pass with no change, exited %d:\n%s", r.Code, r.Output())
		}
	})

	for i, c := range breaking {
		file := write("breaking-"+string(rune('0'+i))+".json", c.schema)
		named := c.key + ": " + c.words
		// Refused with no breaking-change commit since the tag, naming the key, the change and
		// the remedy.
		t.Run(c.what+" is refused with no breaking-change commit", func(t *testing.T) {
			on("main")
			r := check(repo, "-old", releaseFile, "-new", file)
			if r.Code != 1 {
				t.Errorf("%s with no breaking-change commit should be refused (exit 1), exited %d:\n%s", c.what, r.Code, r.Output())
			}
			out := r.Output()
			if !strings.Contains(out, named) || !strings.Contains(out, "BREAKING-CHANGE:") || !strings.Contains(out, "v1.0.0") {
				t.Errorf("%s: the refusal should name %s, %q, the release and the remedy:\n%s", c.what, c.key, c.words, out)
			}
		})
		// Passed with a BREAKING-CHANGE footer, and with a ! in a header.
		for _, b := range []struct{ branch, how string }{{"footer", "a BREAKING-CHANGE footer"}, {"bang", "a ! header"}} {
			t.Run(c.what+" passes with "+b.how, func(t *testing.T) {
				on(b.branch)
				r := check(repo, "-old", releaseFile, "-new", file)
				if r.Code != 0 || !strings.Contains(r.Output(), named) || !strings.Contains(r.Output(), "marked by") {
					t.Errorf("%s with a breaking-change commit (%s) should pass, naming it, exited %d:\n%s", c.what, b.branch, r.Code, r.Output())
				}
			})
		}
	}

	for i, c := range compatible {
		t.Run(c.what+" passes and is printed", func(t *testing.T) {
			on("main")
			r := check(repo, "-old", releaseFile, "-new", write("compatible-"+string(rune('0'+i))+".json", c.schema))
			if r.Code != 0 || !strings.Contains(r.Output(), "compatible since v1.0.0: "+c.key+": "+c.words) {
				t.Errorf("%s should pass and be printed, exited %d:\n%s", c.what, r.Code, r.Output())
			}
		})
	}

	// -range-from: a range with no feat or fix checks nothing; one with a feat checks.
	removed := filepath.Join(tmp, "breaking-0.json")
	t.Run("range-from checks nothing for a range of a docs commit", func(t *testing.T) {
		on("docs")
		r := check(repo, "-range-from", released, "-old", releaseFile, "-new", removed)
		if r.Code != 0 || !strings.Contains(r.Output(), "no feat or fix") {
			t.Errorf("a range of a docs commit should check nothing, exited %d:\n%s", r.Code, r.Output())
		}
	})
	t.Run("range-from checks a range with a feat", func(t *testing.T) {
		on("main")
		r := check(repo, "-range-from", released, "-old", releaseFile, "-new", removed)
		if r.Code != 1 || !strings.Contains(r.Output(), "hooks.bin: key removed") {
			t.Errorf("a range with a feat should check, and refuse a key removed, exited %d:\n%s", r.Code, r.Output())
		}
	})

	t.Run("a keyword the comparison does not read stops it", func(t *testing.T) {
		on("main")
		pattern := write("pattern.json", changed(func(s schema) { at(s, "properties", "hooks", "properties", "bin")["pattern"] = "^t" }))
		r := check(repo, "-old", releaseFile, "-new", pattern)
		if r.Code != 2 || !strings.Contains(r.Output(), `"pattern"`) {
			t.Errorf("a keyword the comparison does not read should stop it (exit 2), exited %d:\n%s", r.Code, r.Output())
		}
	})

	// The download: the release's schema served at <url>/v1.0.0/itos.schema.json is compared; a
	// 404 and a closed port stop the check with exit 2.
	served, _ := json.Marshal(release())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/download/v1.0.0/itos.schema.json" {
			w.Header().Set("content-type", "application/json")
			w.Write(served)
			return
		}
		http.NotFound(w, req)
	}))
	defer server.Close()
	fetched := func(url string) scratch.Result {
		return check(repo, "-release-url", url, "-new", removed)
	}
	t.Run("the downloaded schema is compared", func(t *testing.T) {
		on("main")
		r := fetched(server.URL + "/download")
		if r.Code != 1 || !strings.Contains(r.Output(), "hooks.bin: key removed") {
			t.Errorf("the downloaded schema should be compared, and a key removed refused, exited %d:\n%s", r.Code, r.Output())
		}
	})
	t.Run("a release without the schema stops it", func(t *testing.T) {
		on("main")
		r := fetched(server.URL + "/elsewhere")
		if r.Code != 2 || !strings.Contains(r.Output(), "404") {
			t.Errorf("a release without the schema (404) should stop the check (exit 2), exited %d:\n%s", r.Code, r.Output())
		}
	})
	t.Run("no network stops it", func(t *testing.T) {
		on("main")
		r := fetched(closed)
		if r.Code != 2 || !strings.Contains(r.Output(), "cannot download") {
			t.Errorf("no network should stop the check (exit 2), exited %d:\n%s", r.Code, r.Output())
		}
	})

	t.Run("no release tag passes, saying so", func(t *testing.T) {
		r := check(rel.Untagged(), "-release-url", closed, "-new", removed)
		if r.Code != 0 || !strings.Contains(r.Output(), "no vX.Y.Z tag") {
			t.Errorf("a repository with no release tag should pass, saying so, exited %d:\n%s", r.Code, r.Output())
		}
	})

	t.Run("a shallow clone stops it", func(t *testing.T) {
		r := check(rel.Shallow(repo), "-old", releaseFile, "-new", removed)
		if r.Code != 2 || !strings.Contains(r.Output(), "shallow") {
			t.Errorf("a shallow clone should stop the check (exit 2), exited %d:\n%s", r.Code, r.Output())
		}
	})
}
