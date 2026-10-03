// Command deps-check is the dependency check (T-067): it refuses a Go module of
// the build younger than a week, unless an exception names it with a reason,
// and then runs govulncheck over the module, a finding failing the check.
//
// Supply-chain attacks are common and easy, so a module is trusted only once it
// has been public for a while, as pnpm's minimumReleaseAge does for npm. Go has
// no such setting, but go get runs no code of a dependency, so a check between
// go get and the commit is safe. The pre-commit hook runs it when go.mod or
// go.sum is staged, and CI when a push's range changes them.
//
//   - Every module of the build, the tools' included, is what
//     `go list -m -json all` lists; a module replaced by another version is
//     checked as that version, and one replaced by a local folder is not
//     checked (it is this repository's own code).
//   - When each was published is what the module proxy says: GOPROXY (as
//     `go env` gives it, default proxy.golang.org) is asked for
//     <module>/@v/<version>.info, whose Time is the publication time. A module
//     GONOPROXY (or GOPRIVATE) names is never asked about, as the go command
//     never asks, and is left out; one no proxy knows is refused.
//   - The exceptions are deps-check.json at the module's root:
//     {"exceptions": [{"module": …, "version": …, "reason": …}]}. An urgent
//     security fix is exactly a young version. An exception no longer needed
//     (its version left the build, or is now old enough) fails the check too,
//     so the file never holds one that excuses nothing.
//   - govulncheck runs only once every module has passed, since running it
//     runs its own code and its dependencies': it is a tool dependency of the
//     module (`go get -tool`, `go tool govulncheck`), so its version is held
//     to the same age.
//
// It imports nothing but the standard library, so no module it is about to
// refuse runs inside it; that is also why the exceptions are JSON.
//
//	go run ./tools/bin/deps-check [-changed-since <rev>] [-exceptions <file>]
//	                              [-min-age <duration>] [-govulncheck <command>]
//
// -changed-since <rev> checks nothing when go.mod and go.sum are the same at
// <rev> and HEAD, which is how CI runs it over a push's range; an empty <rev>
// checks. Exit status: 0 passed, 1 refused or a finding, 2 the check could not
// run.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const day = 24 * time.Hour

// module is what `go list -m -json` prints of a module, as much as is read.
type module struct {
	Path    string
	Version string
	Main    bool
	Replace *module
}

// exception is one entry of the exceptions file.
type exception struct {
	Module  string `json:"module"`
	Version string `json:"version"`
	Reason  string `json:"reason"`
}

// published is a module of the build and when the proxy says it was published
// (zero when no proxy knows), or why it could not be asked.
type published struct {
	Path, Version string
	Time          time.Time
	Err           error
}

func main() {
	os.Exit(run())
}

func run() int {
	changedSince := flag.String("changed-since", "", "check nothing when go.mod and go.sum are unchanged between this revision and HEAD")
	exceptionsFile := flag.String("exceptions", "deps-check.json", "the exceptions file; a missing one excepts nothing")
	minAge := flag.Duration("min-age", 7*day, "how long a module must have been public")
	govulncheck := flag.String("govulncheck", "go tool govulncheck -test ./...", "the vulnerability check, run once every module has passed")
	flag.Parse()
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "deps-check: unexpected argument %q\n", flag.Arg(0))
		return 2
	}

	if *changedSince != "" {
		err := exec.Command("git", "diff", "--quiet", *changedSince, "HEAD", "--", "go.mod", "go.sum").Run()
		var exit *exec.ExitError
		switch {
		case err == nil:
			fmt.Printf("deps-check: go.mod and go.sum are unchanged since %s: nothing to check\n", *changedSince)
			return 0
		case !errors.As(err, &exit) || exit.ExitCode() != 1:
			fmt.Fprintf(os.Stderr, "deps-check: cannot compare go.mod and go.sum with %s: %v\n", *changedSince, err)
			return 2
		}
	}

	exceptions, err := readExceptions(*exceptionsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "deps-check: %v\n", err)
		return 2
	}
	modules, err := listModules()
	if err != nil {
		fmt.Fprintf(os.Stderr, "deps-check: %v\n", err)
		return 2
	}
	proxies, private, err := goEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "deps-check: %v\n", err)
		return 2
	}

	var asked []module
	for _, m := range modules {
		if matchesPrefix(private, m.Path) {
			fmt.Printf("deps-check: %s %s is private (GONOPROXY): not asked about\n", m.Path, m.Version)
			continue
		}
		asked = append(asked, m)
	}
	results := publication(proxies, asked)

	now := time.Now()
	problems := judge(results, exceptions, *exceptionsFile, *minAge, now)
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, p)
		}
		fmt.Fprintf(os.Stderr, "\ndeps-check: %d problem(s); govulncheck was not run\n", len(problems))
		return 1
	}
	summary(results, *minAge, now)

	words := strings.Fields(*govulncheck)
	if len(words) == 0 {
		fmt.Fprintln(os.Stderr, "deps-check: -govulncheck is empty")
		return 2
	}
	fmt.Printf("$ %s\n", strings.Join(words, " "))
	cmd := exec.Command(words[0], words[1:]...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 3 {
			fmt.Fprintln(os.Stderr, "deps-check: govulncheck found a vulnerability the code reaches")
		} else {
			fmt.Fprintf(os.Stderr, "deps-check: govulncheck failed: %v\n", err)
		}
		return 1
	}
	return 0
}

// readExceptions reads the exceptions file, refusing an entry without a
// module, a version or a reason, and a module and version named twice.
func readExceptions(file string) ([]exception, error) {
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc struct {
		Exceptions []exception `json:"exceptions"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("%s: %v", file, err)
	}
	seen := map[string]bool{}
	for i, e := range doc.Exceptions {
		if e.Module == "" || e.Version == "" || strings.TrimSpace(e.Reason) == "" {
			return nil, fmt.Errorf("%s: exception %d needs a module, a version and a reason", file, i+1)
		}
		key := e.Module + "@" + e.Version
		if seen[key] {
			return nil, fmt.Errorf("%s: %s %s is excepted twice", file, e.Module, e.Version)
		}
		seen[key] = true
	}
	return doc.Exceptions, nil
}

// listModules lists every module of the build but the main one, each once,
// a replacement by another version standing for the module it replaces.
func listModules() ([]module, error) {
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list -m -json all: %v\n%s", err, stderr.String())
	}
	decoder := json.NewDecoder(bytes.NewReader(stdout))
	seen := map[string]bool{}
	var modules []module
	for {
		var m module
		if err := decoder.Decode(&m); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("go list -m -json all: %v", err)
		}
		if m.Main {
			continue
		}
		if m.Replace != nil {
			if m.Replace.Version == "" {
				continue // a local folder: this repository's own code
			}
			m = *m.Replace
		}
		if key := m.Path + "@" + m.Version; !seen[key] {
			seen[key] = true
			modules = append(modules, m)
		}
	}
	return modules, nil
}

// proxy is one entry of GOPROXY: a URL, "direct" or "off", and whether the
// next entry is tried after any error (|) or only after not found (,).
type proxy struct {
	URL         string
	AnyFallback bool
}

// goEnv reads GOPROXY and GONOPROXY as the go command resolves them.
func goEnv() ([]proxy, []string, error) {
	out, err := exec.Command("go", "env", "GOPROXY", "GONOPROXY").Output()
	if err != nil {
		return nil, nil, fmt.Errorf("go env GOPROXY GONOPROXY: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	for len(lines) < 2 {
		lines = append(lines, "")
	}
	var proxies []proxy
	rest := strings.TrimSpace(lines[0])
	for rest != "" {
		i := strings.IndexAny(rest, ",|")
		entry, any := rest, false
		if i >= 0 {
			entry, any, rest = rest[:i], rest[i] == '|', rest[i+1:]
		} else {
			rest = ""
		}
		if entry = strings.TrimSpace(entry); entry != "" {
			proxies = append(proxies, proxy{URL: strings.TrimSuffix(entry, "/"), AnyFallback: any})
		}
	}
	var private []string
	for _, p := range strings.Split(lines[1], ",") {
		if p = strings.TrimSpace(p); p != "" {
			private = append(private, p)
		}
	}
	return proxies, private, nil
}

// matchesPrefix is the go command's GONOPROXY match: a pattern matches a
// path whose first elements, as many as the pattern has, it matches as a glob.
func matchesPrefix(patterns []string, target string) bool {
	elements := strings.Split(target, "/")
	for _, pattern := range patterns {
		n := strings.Count(pattern, "/") + 1
		if n > len(elements) {
			continue
		}
		if ok, _ := path.Match(pattern, strings.Join(elements[:n], "/")); ok {
			return true
		}
	}
	return false
}

var client = &http.Client{Timeout: 30 * time.Second}

// errNotFound is a proxy saying it does not have a module's version.
var errNotFound = errors.New("not found")

// publication asks the proxies when each module was published, eight at a
// time, and gives the answers sorted by module path.
func publication(proxies []proxy, modules []module) []published {
	results := make([]published, len(modules))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for i := range jobs {
				m := modules[i]
				t, err := info(proxies, m.Path, m.Version)
				results[i] = published{Path: m.Path, Version: m.Version, Time: t, Err: err}
			}
		})
	}
	for i := range modules {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].Path < results[j].Path })
	return results
}

// info asks the proxies in GOPROXY's order for a version's .info and gives
// its Time: the next proxy after a not found, or after anything behind a |.
// "direct" and "off" end the list, as no time can be had from them that a
// module's author could not set.
func info(proxies []proxy, modPath, version string) (time.Time, error) {
	last := fmt.Errorf("GOPROXY names no proxy")
	for _, p := range proxies {
		if p.URL == "direct" || p.URL == "off" {
			break
		}
		t, err := fetchInfo(p.URL, modPath, version)
		if err == nil {
			return t, nil
		}
		last = fmt.Errorf("%s: %w", p.URL, err)
		if !p.AnyFallback && !errors.Is(err, errNotFound) {
			break
		}
	}
	return time.Time{}, last
}

func fetchInfo(base, modPath, version string) (time.Time, error) {
	rel := escape(modPath) + "/@v/" + escape(version) + ".info"
	var data []byte
	if local, ok := strings.CutPrefix(base, "file://"); ok {
		if u, err := url.Parse(base); err == nil && u.Path != "" {
			local = u.Path
		}
		b, err := os.ReadFile(filepath.Join(filepath.FromSlash(local), filepath.FromSlash(rel)))
		if errors.Is(err, os.ErrNotExist) {
			return time.Time{}, errNotFound
		}
		if err != nil {
			return time.Time{}, err
		}
		data = b
	} else {
		resp, err := client.Get(base + "/" + rel)
		if err != nil {
			return time.Time{}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
			return time.Time{}, errNotFound
		}
		if resp.StatusCode != http.StatusOK {
			return time.Time{}, fmt.Errorf("%s", resp.Status)
		}
		if data, err = io.ReadAll(io.LimitReader(resp.Body, 1<<20)); err != nil {
			return time.Time{}, err
		}
	}
	var doc struct{ Time time.Time }
	if err := json.Unmarshal(data, &doc); err != nil {
		return time.Time{}, fmt.Errorf("%s: %v", rel, err)
	}
	if doc.Time.IsZero() {
		return time.Time{}, fmt.Errorf("%s has no Time", rel)
	}
	return doc.Time, nil
}

// escape is the module proxy's case encoding: each capital letter becomes !
// and the letter in lower case.
func escape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if 'A' <= r && r <= 'Z' {
			b.WriteByte('!')
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// judge gives every problem: a module too young or of unknown age that no
// exception names, and an exception that excuses nothing.
func judge(results []published, exceptions []exception, file string, minAge time.Duration, now time.Time) []string {
	excepted := map[string]exception{}
	for _, e := range exceptions {
		excepted[e.Module+"@"+e.Version] = e
	}
	inBuild := map[string]published{}
	var problems []string
	for _, r := range results {
		key := r.Path + "@" + r.Version
		inBuild[key] = r
		e, ok := excepted[key]
		switch {
		case r.Err != nil && !ok:
			problems = append(problems, fmt.Sprintf(
				"deps-check: %s %s: no proxy says when it was published (%v).\n  Except it %s",
				r.Path, r.Version, r.Err, howToExcept(file, r.Path, r.Version)))
		case r.Err == nil && now.Sub(r.Time) < minAge && !ok:
			problems = append(problems, fmt.Sprintf(
				"deps-check: %s %s was published %s, %s ago, less than %s.\n  Wait until %s, or except it %s",
				r.Path, r.Version, r.Time.UTC().Format(time.RFC3339), age(now.Sub(r.Time)), age(minAge),
				r.Time.Add(minAge).UTC().Format(time.RFC3339), howToExcept(file, r.Path, r.Version)))
		case ok && r.Err != nil:
			fmt.Printf("deps-check: %s %s excepted (no proxy knows it): %s\n", r.Path, r.Version, e.Reason)
		case ok && now.Sub(r.Time) < minAge:
			fmt.Printf("deps-check: %s %s excepted (published %s, %s ago): %s\n",
				r.Path, r.Version, r.Time.UTC().Format(time.RFC3339), age(now.Sub(r.Time)), e.Reason)
		case ok:
			problems = append(problems, fmt.Sprintf(
				"deps-check: the exception for %s %s in %s is no longer needed: it was published %s, %s ago. Remove it.",
				r.Path, r.Version, file, r.Time.UTC().Format(time.RFC3339), age(now.Sub(r.Time))))
		}
	}
	for _, e := range exceptions {
		if _, ok := inBuild[e.Module+"@"+e.Version]; !ok {
			problems = append(problems, fmt.Sprintf(
				"deps-check: the exception for %s %s in %s names a version not in the build. Remove it.",
				e.Module, e.Version, file))
		}
	}
	return problems
}

func howToExcept(file, modPath, version string) string {
	return fmt.Sprintf("in %s, with the reason it cannot wait:\n    {\"module\": %q, \"version\": %q, \"reason\": \"…\"}",
		file, modPath, version)
}

// age is a duration in whole days, or hours under two days.
func age(d time.Duration) string {
	if d < 2*day {
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
	return fmt.Sprintf("%d days", int(d/day))
}

// summary is the one line a passing age check prints: how many modules, and
// the newest of those not excepted.
func summary(results []published, minAge time.Duration, now time.Time) {
	var newest *published
	for i := range results {
		r := &results[i]
		if r.Err == nil && now.Sub(r.Time) >= minAge && (newest == nil || r.Time.After(newest.Time)) {
			newest = r
		}
	}
	line := fmt.Sprintf("deps-check: %d modules, each published at least %s ago or excepted", len(results), age(minAge))
	if newest != nil {
		line += fmt.Sprintf(" (the newest: %s %s, %s, %s ago)", newest.Path, newest.Version,
			newest.Time.UTC().Format("2006-01-02"), age(now.Sub(newest.Time)))
	}
	fmt.Println(line)
}
