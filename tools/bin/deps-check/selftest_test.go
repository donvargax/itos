// The dependency check's self-test (T-067's check, kept as a step in T-125, a Go test since
// T-143), against a module proxy of local files, with no network: a scratch module requires
// modules the proxy says were published a day ago and a month ago, and the check is run on it.
//
//   - a module a day old is refused, naming it and how to except it, and govulncheck is not run;
//   - excepted with a reason, it passes;
//   - a module a month old passes;
//   - an exception that excuses nothing (a version not in the build, or one now old enough)
//     fails, and one without a reason is refused;
//   - a govulncheck finding (its exit status 3) fails the check;
//   - -changed-since checks nothing when go.mod and go.sum did not change.
//
// govulncheck itself needs the network (the vulnerability database, and its own source on a cold
// module cache), so here it is a stub the check is told to run, which proves what the check does
// with its answer; the real one runs wherever the check does, on every commit that stages go.mod
// or go.sum. Every proxy variable points at a closed port, so any request that tried the network
// would fail.
package main_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
)

const (
	young = "example.com/young"
	old   = "example.com/old"
)

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	tmp := t.TempDir()
	proxy := filepath.Join(tmp, "proxy")
	app := filepath.Join(tmp, "app")
	marker := filepath.Join(tmp, "govulncheck-ran")
	bin := scratch.Build(t, "deps-check")
	const closed = "http://127.0.0.1:9"
	env := scratch.With(scratch.OutsideEnv(),
		"GOPROXY=file://"+filepath.ToSlash(proxy), "GOSUMDB=off", "GONOPROXY=", "GOPRIVATE=", "GONOSUMDB=",
		"GOMODCACHE="+filepath.Join(tmp, "modcache"), "GOFLAGS=-modcacherw", "GOTOOLCHAIN=local", "GOWORK=off",
		"HTTP_PROXY="+closed, "HTTPS_PROXY="+closed, "http_proxy="+closed, "https_proxy="+closed,
		"NO_PROXY=", "no_proxy=")

	// A module version the proxy serves: its .info, published ago before now, and its go.mod,
	// whose hash go.sum holds (dirhash's h1 over the one file go.mod).
	modFile := func(path string) string { return "module " + path + "\n\ngo 1.21\n" }
	sum := func(path string) string {
		file := sha256.Sum256([]byte(modFile(path)))
		h1 := sha256.Sum256([]byte(hex.EncodeToString(file[:]) + "  go.mod\n"))
		return path + " v1.0.0/go.mod h1:" + base64.StdEncoding.EncodeToString(h1[:]) + "\n"
	}
	serve := func(path string, ago time.Duration) {
		info, _ := json.Marshal(map[string]string{"Version": "v1.0.0", "Time": time.Now().Add(-ago).UTC().Format(time.RFC3339)})
		scratch.WriteFiles(t, filepath.Join(proxy, path, "@v"), map[string]string{
			"list": "v1.0.0\n", "v1.0.0.info": string(info), "v1.0.0.mod": modFile(path),
		})
	}
	day := 24 * time.Hour
	serve(young, day)
	serve(old, 30*day)
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}

	// The scratch module, requiring each of requires at v1.0.0, with these exceptions.
	type exception struct {
		Module  string `json:"module"`
		Version string `json:"version"`
		Reason  string `json:"reason"`
	}
	setUp := func(requires []string, exceptions ...exception) {
		os.Remove(filepath.Join(app, "deps-check.json"))
		os.Remove(marker)
		var lines, sums string
		for _, r := range requires {
			lines += "\t" + r + " v1.0.0\n"
			sums += sum(r)
		}
		files := map[string]string{
			"go.mod": "module example.com/app\n\ngo 1.21\n\nrequire (\n" + lines + ")\n",
			"go.sum": sums,
		}
		if exceptions != nil {
			data, _ := json.MarshalIndent(map[string]any{"exceptions": exceptions}, "", "\t")
			files["deps-check.json"] = string(data)
		}
		scratch.WriteFiles(t, app, files)
	}
	stub := filepath.Join(tmp, "govulncheck.sh")
	scratch.WriteFiles(t, tmp, map[string]string{
		"govulncheck.sh": "touch '" + filepath.ToSlash(marker) + "'\necho 'stub govulncheck'\nexit \"${STUB_EXIT:-0}\"\n",
	})
	check := func(vulnExit int, args ...string) scratch.Result {
		argv := append([]string{"-govulncheck", "sh " + filepath.ToSlash(stub)}, args...)
		return scratch.Run(app, scratch.With(env, "STUB_EXIT="+strconv.Itoa(vulnExit)), "", bin, argv...)
	}
	ran := func() bool { _, err := os.Stat(marker); return err == nil }
	const reason = "the fix for a vulnerability that cannot wait a week"

	t.Run("a module a day old is refused, and govulncheck not run", func(t *testing.T) {
		setUp([]string{young, old})
		r := check(0)
		if r.Code != 1 {
			t.Errorf("a module a day old should be refused (exit 1), exited %d:\n%s", r.Code, r.Output())
		}
		if !strings.Contains(r.Output(), young+" v1.0.0 was published") || !strings.Contains(r.Output(), "deps-check.json") {
			t.Errorf("the refusal should name %s v1.0.0 and the exceptions file:\n%s", young, r.Output())
		}
		if strings.Contains(r.Output(), old) {
			t.Errorf("a module a month old should not be named:\n%s", r.Output())
		}
		if ran() {
			t.Error("govulncheck ran though a module was refused")
		}
	})

	t.Run("a module a day old excepted with a reason passes, and govulncheck runs", func(t *testing.T) {
		setUp([]string{young, old}, exception{young, "v1.0.0", reason})
		r := check(0)
		if r.Code != 0 {
			t.Errorf("an excepted module a day old should pass, exited %d:\n%s", r.Code, r.Output())
		}
		if !strings.Contains(r.Output(), reason) {
			t.Errorf("the pass should print the exception's reason:\n%s", r.Output())
		}
		if !ran() {
			t.Error("govulncheck did not run once every module had passed")
		}
	})

	t.Run("a module a month old passes", func(t *testing.T) {
		setUp([]string{old})
		r := check(0)
		if r.Code != 0 || !strings.Contains(r.Output(), old+" v1.0.0") {
			t.Errorf("a module a month old should pass, naming the newest module, exited %d:\n%s", r.Code, r.Output())
		}
	})

	t.Run("an exception for a version not in the build fails", func(t *testing.T) {
		setUp([]string{old}, exception{young, "v1.0.0", reason})
		r := check(0)
		if r.Code != 1 || !strings.Contains(r.Output(), "names a version not in the build") {
			t.Errorf("an exception for a version not in the build should fail, exited %d:\n%s", r.Code, r.Output())
		}
	})

	t.Run("an exception for a module old enough fails", func(t *testing.T) {
		setUp([]string{old}, exception{old, "v1.0.0", reason})
		r := check(0)
		if r.Code != 1 || !strings.Contains(r.Output(), "no longer needed") {
			t.Errorf("an exception for a module old enough should fail, exited %d:\n%s", r.Code, r.Output())
		}
	})

	t.Run("an exception without a reason is refused", func(t *testing.T) {
		setUp([]string{young}, exception{young, "v1.0.0", ""})
		r := check(0)
		if r.Code != 2 || !strings.Contains(r.Output(), "reason") {
			t.Errorf("an exception without a reason should be refused (exit 2), exited %d:\n%s", r.Code, r.Output())
		}
	})

	t.Run("a govulncheck finding fails the check", func(t *testing.T) {
		setUp([]string{old})
		r := check(3)
		if r.Code != 1 || !strings.Contains(r.Output(), "govulncheck found") {
			t.Errorf("a govulncheck finding should fail the check, exited %d:\n%s", r.Code, r.Output())
		}
	})

	// -changed-since: nothing to check when go.mod and go.sum did not change since the revision,
	// the whole check when they did.
	git := func(args ...string) {
		t.Helper()
		if r := scratch.Run(app, env, "", scratch.Git(), args...); r.Code != 0 {
			t.Fatalf("git %s:\n%s", strings.Join(args, " "), r.Output())
		}
	}
	setUp([]string{old})
	git("init", "-q")
	git("add", "go.mod", "go.sum")
	git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "old")
	setUp([]string{young, old})
	git("add", "go.mod", "go.sum")
	git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "young")

	t.Run("changed-since checks nothing when go.mod and go.sum did not change", func(t *testing.T) {
		r := check(0, "-changed-since", "HEAD")
		if r.Code != 0 || !strings.Contains(r.Output(), "nothing to check") {
			t.Errorf("-changed-since HEAD should check nothing, exited %d:\n%s", r.Code, r.Output())
		}
	})

	t.Run("changed-since checks when they did", func(t *testing.T) {
		r := check(0, "-changed-since", "HEAD~1")
		if r.Code != 1 || !strings.Contains(r.Output(), young+" v1.0.0") {
			t.Errorf("-changed-since HEAD~1 should check and refuse %s, exited %d:\n%s", young, r.Code, r.Output())
		}
	})
}
