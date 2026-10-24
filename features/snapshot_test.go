package features

import (
	"fmt"
	"github.com/cucumber/godog"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

type snapshotFixture struct {
	files                   map[string]string
	git                     map[string]string
	context                 map[string]string
	config                  string
	common, temporary, mode string
}

const snapshotProbe = `#!/bin/sh
set -u
n=$(find "$SNAPSHOT_LOG" -name 'run-*' -type d | wc -l | tr -d ' ')
out="$SNAPSHOT_LOG/run-$n"
mkdir "$out" || exit 90
(pwd -W 2>/dev/null || pwd -P) > "$out/cwd"
printf '%s' "${GOCACHE-}" > "$out/cache"
git config --local --get core.filemode > "$out/filemode"
git config --local --get core.symlinks > "$out/symlinks"
for p in added.txt mode-entry gen/out.txt; do
 if test -L "$p"; then readlink "$p" > "$out/$(basename "$p").link"
 elif test -f "$p"; then cat "$p" > "$out/$(basename "$p")"; fi
done
for p in deleted.txt node_modules .cache; do
 if test -e "$p"; then printf '%s\n' "$p" >> "$out/present"; fi
done
if test -x mode-entry; then touch "$out/executable"; fi
case "$1" in
 require) if test -f "$2"; then echo present > "$out/require"; exit 0; else echo absent > "$out/require"; exit 1; fi ;;
 expect-text) cat "$2" > "$out/text"; test "$(cat "$2")" = "$3" ;;
 generate) mkdir -p "$(dirname "$2")"; printf generated > "$2" ;;
 leave-artifacts) mkdir -p gen; printf 'private-artifact\n' > gen/artifact.txt; printf 'snapshot-artifact-output\n'; exit "$2" ;;
 record-run) exit 0 ;;
 git-context)
 { git rev-parse --verify -q HEAD || echo unborn; } > "$out/head"
 { git symbolic-ref -q HEAD || echo detached; } > "$out/branch"
 { git rev-parse --verify -q HEAD~1 || echo none; } > "$out/parent"
 git write-tree > "$out/index"
 git for-each-ref --format='%(refname) %(objectname)' > "$out/refs"
 git config --get-regexp '^remote\.' | sort > "$out/remotes"
 { git rev-parse --symbolic-full-name '@{u}' 2>/dev/null || echo none; } > "$out/upstream"
 exit 0 ;;
 write-git)
 git update-ref refs/heads/probe-branch HEAD || exit 1
 git tag probe-tag || exit 1
 up=$(git rev-parse --symbolic-full-name '@{u}') || exit 1
 git update-ref "$up" 'HEAD~1' || exit 1
 git config remote.origin.url /nowhere || exit 1
 blob=$(printf snapshot-private-object | git hash-object -w --stdin) || exit 1
 git commit -q --allow-empty -m probe || exit 1
 commit=$(git rev-parse HEAD) || exit 1
 printf '%s\n%s\n' "$blob" "$commit" > "$out/objects"
 git read-tree --empty || exit 1
 git symbolic-ref HEAD refs/heads/probe-branch || exit 1
 exit 0 ;;
 inspect-entries)
 git ls-files -s > "$out/entries"
 while read -r mode hash stage path; do
  git cat-file blob "$hash" > "$out/expected" || exit 91
  if test "$mode" = 120000 && test "$(git config core.symlinks)" != false; then
   test -L "$path" && test "$(readlink "$path")" = "$(cat "$out/expected")" || { echo "$path"; exit 1; }
  else
   test -f "$path" && cmp -s "$out/expected" "$path" || { echo "$path"; exit 1; }
   if test "$(git config core.filemode)" = true; then
    if test "$mode" = 100755; then test -x "$path" || exit 1
    else test ! -x "$path" || exit 1; fi
   fi
  fi
 done < "$out/entries" || exit 1
 find . -path './.git' -prune -o -type f -print > "$out/files"
 while read -r path; do git ls-files --error-unmatch -- "$path" >/dev/null 2>&1 || { echo "$path"; exit 1; }; done < "$out/files"
 ;;
 *) exit 92 ;;
esac
`

func initializeSnapshotSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^snapshot probe commands are on PATH with an observation log outside the repository$`, func() error {
		common, err := w.gitOutput("rev-parse", "--path-format=absolute", "--git-common-dir")
		if err != nil {
			return err
		}
		w.snapFixture = &snapshotFixture{common: strings.TrimSpace(common)}
		bin, err := w.binOnPath()
		if err != nil {
			return err
		}
		log := filepath.Join(w.support, "snapshot-observations")
		for _, p := range []string{bin, log} {
			if err := os.MkdirAll(p, 0o755); err != nil {
				return err
			}
		}
		if err := w.writeProgram(filepath.Join(bin, "snapshot-probe"), snapshotProbe); err != nil {
			return err
		}
		w.vars = append(w.vars, "SNAPSHOT_LOG="+filepath.ToSlash(log))
		return nil
	})
	sc.Step(`^check-output temporary files use an existing fixture-owned temporary directory$`, func() error {
		p := filepath.Join(w.support, "check-output")
		if err := os.MkdirAll(p, 0o755); err != nil {
			return err
		}
		w.snapFixture.temporary = p
		w.vars = append(w.vars, "TMPDIR="+p, "TMP="+p, "TEMP="+p)
		return nil
	})
	sc.Step(`^the task "([^"]*)" has these static checks in written order:$`, func(task string, table *godog.Table) error {
		var checks []string
		for _, row := range table.Rows[1:] {
			checks = append(checks, staticCheck(row.Cells[0].Value))
		}
		return w.stagedChecks(task, checks...)
	})
	sc.Step(`^the fixture commits its retained-check task data as the base$`, func() error {
		for p, text := range map[string]string{"deleted.txt": "base", ".gitignore": "node_modules/\n.cache/\n"} {
			if err := w.write(p, text); err != nil {
				return err
			}
			if err := w.git("add", "--", p); err != nil {
				return err
			}
		}
		return w.commit("docs: retain task data")
	})
	sc.Step(`^"([^"]*)" is staged with the text "([^"]*)"$`, func(p, text string) error {
		if err := w.write(p, text); err != nil {
			return err
		}
		return w.git("add", "--", p)
	})
	sc.Step(`^the working-tree copy of "([^"]*)" is replaced with "([^"]*)"$`, w.write)
	sc.Step(`^the working-tree copy of "([^"]*)" is missing$`, func(p string) error { return os.Remove(filepath.Join(w.dir, p)) })
	sc.Step(`^the probe recorded that "([^"]*)" was absent from its execution directory$`, func(string) error { return w.probeEquals(0, "require", "absent\n") })
	sc.Step(`^the probe recorded the text "([^"]*)" read from "([^"]*)"$`, func(text, _ string) error { return w.probeEquals(0, "text", text) })
	sc.Step(`^the fixture saves the source state$`, func() (err error) {
		w.snapFixture.files, err = snapshotFiles(w.dir)
		if err != nil {
			return err
		}
		if w.snapFixture.git, err = w.snapshotGit(); err != nil {
			return err
		}
		if w.snapFixture.config, err = w.gitOutput("config", "--local", "--list"); err != nil {
			return err
		}
		w.snapFixture.context = w.gitContextIn(w.dir)
		return nil
	})
	sc.Step(`^the source files, (?:modes, )?staged entries, HEAD and refs match the saved state$`, w.snapshotUnchanged)
	sc.Step(`^no stash or registered worktree was created$`, w.snapshotUnchanged)
	sc.Step(`^the snapshot fixture runs on (.+)$`, func(platform string) error {
		if map[string]string{"Linux": "linux", "macOS": "darwin", "Windows": "windows"}[platform] != runtime.GOOS {
			return godog.ErrSkip
		}
		return nil
	})
	sc.Step(`^the index adds "([^"]*)" containing "([^"]*)", deletes "([^"]*)" and adds "([^"]*)" with mode (\d+)$`, func(add, text, del, entry, mode string) error {
		w.snapFixture.mode = mode
		if err := w.write(add, text); err != nil {
			return err
		}
		if err := w.git("add", "--", add); err != nil {
			return err
		}
		if err := w.git("rm", "-q", "--", del); err != nil {
			return err
		}
		return w.write(entry, "disk-version")
	})
	sc.Step(`^the staged mode entry has (?:the text|the link target) "([^"]*)"$`, func(text string) error {
		p := filepath.Join(w.support, "entry-content")
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			return err
		}
		hash, err := w.gitOutput("hash-object", "-w", p)
		if err != nil {
			return err
		}
		return w.git("update-index", "--add", "--cacheinfo", w.snapFixture.mode+","+strings.TrimSpace(hash)+",mode-entry")
	})
	sc.Step(`^disk instead has different ordinary-file contents for "([^"]*)" and "([^"]*)" and a regenerated "([^"]*)"$`, func(a, b, c string) error {
		for _, p := range []string{a, b, c} {
			if err := w.write(p, "disk-version"); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^ignored "([^"]*)" and "([^"]*)" directories exist only in the source checkout$`, func(a, b string) error {
		for _, p := range []string{a, b} {
			if err := w.write(p+"/ignored", "local"); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^the fixture sets source-repository-local core.filemode to (true|false) and core.symlinks to (true|false)$`, func(a, b string) error {
		if err := w.git("config", "core.filemode", a); err != nil {
			return err
		}
		return w.git("config", "core.symlinks", b)
	})
	sc.Step(`^the probe recorded "([^"]*)" in "([^"]*)" and no "([^"]*)", "([^"]*)" or "([^"]*)"$`, func(text, p, _, _, _ string) error {
		if err := w.probeEquals(0, p, text); err != nil {
			return err
		}
		return w.probeMissing(0, "present")
	})
	sc.Step(`^the probe recorded "([^"]*)" as (.+)$`, func(p, result string) error {
		parts := strings.Split(result, `"`)
		if len(parts) < 2 {
			return fmt.Errorf("invalid observation %s", result)
		}
		if strings.HasPrefix(result, "a symlink") {
			return w.probeEquals(0, p+".link", parts[1]+"\n")
		}
		if err := w.probeEquals(0, p, parts[1]); err != nil {
			return err
		}
		if strings.HasPrefix(result, "an executable") {
			_, err := os.Stat(w.probePath(0, "executable"))
			return err
		}
		return nil
	})
	sc.Step(`^the probe recorded snapshot-local core.filemode as (true|false) and core.symlinks as (true|false)$`, func(a, b string) error {
		if err := w.probeEquals(0, "filemode", a+"\n"); err != nil {
			return err
		}
		return w.probeEquals(0, "symlinks", b+"\n")
	})
	sc.Step(`^the fixture sets GOCACHE to an existing absolute directory outside the repository$`, func() error {
		p := filepath.Join(w.support, "go-cache")
		if err := os.MkdirAll(p, 0o755); err != nil {
			return err
		}
		w.vars = append(w.vars, "GOCACHE="+p)
		return nil
	})
	sc.Step(`^(?:both probes recorded the same|the probe recorded a) disposable checkout under the source git common directory's "([^"]*)", not the source checkout$`, func(p string) error { return w.disposableProbe(p) })
	sc.Step(`^the second probe recorded the first probe's generated "([^"]*)"$`, func(p string) error { return w.probeEquals(1, filepath.Base(p), "generated") })
	sc.Step(`^both probes recorded GOCACHE unchanged from the fixture's value$`, func() error {
		for i := 0; i < 2; i++ {
			if err := w.probeEquals(i, "cache", filepath.Join(w.support, "go-cache")); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^the recorded snapshot directory(?:, its private Git metadata, generated artifact)? and check-output temporary files no longer exist$`, w.snapshotClean)
	sc.Step(`^the fixture writes a regular file at the git common dir's "([^"]*)"$`, func(p string) error {
		p = filepath.Join(w.snapFixture.common, p)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		// The folder a run left empty there gives way to the file; a folder
		// with something in it stays, and the write refuses.
		_ = os.Remove(p)
		return os.WriteFile(p, []byte("blocking file"), 0o644)
	})
	sc.Step(`^the index contains a gitlink at "([^"]*)" pointing to HEAD$`, func(p string) error {
		head, err := w.head()
		if err != nil {
			return err
		}
		return w.git("update-index", "--add", "--cacheinfo", "160000,"+head+","+p)
	})
	sc.Step(`^its diagnostic contains "([^"]*)" and identifies (.+)$`, func(reason, location string) error {
		parts := strings.Split(location, `"`)
		if len(parts) < 2 {
			return fmt.Errorf("invalid location %s", location)
		}
		out := strings.ReplaceAll(w.output(), `\`, "/")
		if !strings.Contains(out, reason) || !strings.Contains(out, parts[1]) {
			return fmt.Errorf("diagnostic lacks %q / %q: %s", reason, parts[1], out)
		}
		return nil
	})
	sc.Step(`^no snapshot probe command ran$`, func() error {
		entries, err := os.ReadDir(filepath.Join(w.support, "snapshot-observations"))
		if err != nil {
			return err
		}
		if len(entries) != 0 {
			return fmt.Errorf("probe ran: %v", entries)
		}
		return nil
	})
	sc.Step(`^no partially created snapshot or check-output temporary file remains$`, w.snapshotClean)
	sc.Step(`^the snapshot-location blocking file and source state are unchanged$`, func() error {
		b, err := os.ReadFile(filepath.Join(w.snapFixture.common, "itos/snapshots"))
		if err != nil {
			return err
		}
		if string(b) != "blocking file" {
			return fmt.Errorf("blocking file changed")
		}
		return w.snapshotUnchanged()
	})
	sc.Step(`^its output (omits|includes) "snapshot-artifact-output".*$`, func(mode string) error {
		for _, text := range []string{"snapshot-artifact-output", "failing T-001", "snapshot-probe leave-artifacts 7"} {
			if strings.Contains(w.output(), text) != (mode == "includes") {
				return fmt.Errorf("output must %s %q: %s", mode, text, w.report())
			}
		}
		return nil
	})
}

func (w *world) probePath(i int, p string) string {
	return filepath.Join(w.support, "snapshot-observations", fmt.Sprintf("run-%d", i), p)
}
func (w *world) probeEquals(i int, p, text string) error {
	b, err := os.ReadFile(w.probePath(i, p))
	if err != nil {
		return err
	}
	if string(b) != text {
		return fmt.Errorf("probe %d %s = %q, want %q", i, p, b, text)
	}
	return nil
}
func (w *world) probeMissing(i int, p string) error {
	_, err := os.Lstat(w.probePath(i, p))
	if !os.IsNotExist(err) {
		return fmt.Errorf("unexpected observation %s: %v", p, err)
	}
	return nil
}
func (w *world) disposableProbe(p string) error {
	entries, err := os.ReadDir(filepath.Join(w.support, "snapshot-observations"))
	if err != nil {
		return err
	}
	var first string
	for i := range entries {
		b, err := os.ReadFile(w.probePath(i, "cwd"))
		if err != nil {
			return err
		}
		cwd := filepath.FromSlash(strings.TrimSpace(string(b)))
		base := filepath.Join(w.snapFixture.common, p)
		rel, err := filepath.Rel(base, cwd)
		if err != nil || strings.HasPrefix(rel, "..") || rel == "." || filepath.Base(cwd) != "tree" {
			return fmt.Errorf("probe ran in %q, want disposable tree under %q", cwd, base)
		}
		if i > 0 && cwd != first {
			return fmt.Errorf("checks used different snapshots")
		}
		first = cwd
	}
	if len(entries) == 0 {
		return fmt.Errorf("no probes recorded")
	}
	return nil
}
func (w *world) snapshotClean() error {
	for _, p := range []string{filepath.Join(w.snapFixture.common, "itos/snapshots"), w.snapFixture.temporary} {
		info, err := os.Stat(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			continue
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			return fmt.Errorf("resources remain in %s: %v", p, entries)
		}
	}
	return nil
}
func snapshotFiles(root string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if rel == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		var b []byte
		if info.Mode()&os.ModeSymlink != 0 {
			s, e := os.Readlink(p)
			b, err = []byte(s), e
		} else {
			b, err = os.ReadFile(p)
		}
		if err != nil {
			return err
		}
		files[rel] = fmt.Sprintf("%v:%s", info.Mode(), b)
		return nil
	})
	return files, err
}
func (w *world) snapshotGit() (map[string]string, error) {
	state := map[string]string{}
	for _, args := range [][]string{{"ls-files", "--stage"}, {"rev-parse", "HEAD"}, {"show-ref"}, {"worktree", "list", "--porcelain"}, {"reflog", "show", "--all"}} {
		out, err := w.gitOutput(args...)
		if err != nil {
			return nil, err
		}
		state[strings.Join(args, " ")] = out
	}
	return state, nil
}
func (w *world) snapshotUnchanged() error {
	files, err := snapshotFiles(w.dir)
	if err != nil {
		return err
	}
	git, err := w.snapshotGit()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(files, w.snapFixture.files) {
		return fmt.Errorf("source files or modes changed: before %v, after %v", w.snapFixture.files, files)
	}
	if !reflect.DeepEqual(git, w.snapFixture.git) {
		return fmt.Errorf("source git state changed: before %v, after %v", w.snapFixture.git, git)
	}
	return nil
}
