// Command build-go is the Go build of itos, the one way a check builds it from a checkout:
//
//	go run ./tools/bin/build-go <out dir>
//
// builds ./cmd/itos into <out dir>/itos (itos.exe on windows) for this machine and prints its
// path. The binary is stamped (-ldflags -X) with the version tools/bin/dev-version gives this
// checkout from git describe, as tools/bin/itos stamps it: the release's at its tag,
// 2.3.1-dev.5.g1234abc five commits after v2.3.0 (T-069; the tag is the version, and nothing in
// the tree holds one). A binary built any other way says the module version Go records instead
// (internal/version). CGO_ENABLED=0, so the binary needs no C library, and -trimpath, so it holds
// no path of the machine that built it.
//
// The release build:
//
//	go run ./tools/bin/build-go --release <out dir>
//
// is GoReleaser's (.goreleaser.yaml, run by tools/bin/pinned), as a snapshot that publishes
// nothing, stamped with that same version, or ITOS_SNAPSHOT_VERSION when it is set (a self-test
// building the release a version would publish): it copies into <out dir> what a release
// publishes, one archive per platform
// (docs/decisions/0022-itos-is-distributed-as-release-archives-with-checksums-installed-pinned.md),
// the config's JSON Schema and checksums.txt, then prints their paths, checksums.txt last. An
// archive is itos-<version>-<os>-<arch>.tar.gz, or .zip for windows, holding at its top level the
// binary (itos, or itos.exe), LICENSE and README.md. The schema is itos.schema.json, written by
// tools/bin/config-schema from the Go config's table, so an editor can check an itos.yaml against
// the release's (`# yaml-language-server: $schema=<its URL>`). checksums.txt is the SHA-256 of
// each archive and of the schema in sha256sum's format, so `sha256sum -c checksums.txt` checks
// them all. The release workflow runs the same configuration at its tag
// (.github/workflows/release.yml), so what this builds is what a release publishes.
//
// The paths it prints are absolute. Exit 0 when it built, 1 when a build failed (its output on
// stderr), 2 for a usage error. It was tools/bin/build-go.ts until T-146.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	repoprogram "github.com/donvargax/itos/v8/tools/bin/repo-program"
)

const module = "github.com/donvargax/itos/v8"

// schema is the config's JSON Schema, as the release publishes it.
const schema = "itos.schema.json"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	release := len(args) > 0 && args[0] == "--release"
	if release {
		args = args[1:]
	}
	if len(args) == 0 || args[0] == "" {
		fmt.Fprintln(stderr, "usage: go run ./tools/bin/build-go [--release] <out dir>")
		return 2
	}
	root, err := top()
	if err != nil {
		fmt.Fprintf(stderr, "build-go: %v\n", err)
		return 1
	}
	var built []string
	if release {
		built, err = buildRelease(root, args[0], stderr)
	} else {
		var binary string
		binary, err = buildNative(root, args[0], stderr)
		built = []string{binary}
	}
	if err != nil {
		fmt.Fprintf(stderr, "build-go: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, strings.Join(built, "\n"))
	return 0
}

// top is the repository's top folder: the nearest folder above the working one that holds go.mod.
func top() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod above the working folder")
		}
		dir = parent
	}
}

// devVersion is the version this checkout's builds say they are (tools/bin/dev-version).
func devVersion(root string) (string, error) {
	name, args := repoprogram.Command(filepath.Join(root, "tools", "bin", "dev-version"), root)
	var out, errs bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = &out, &errs
	err := cmd.Run()
	version := strings.TrimSpace(out.String())
	if err != nil || version == "" {
		said := errs.String()
		if said == "" && err != nil {
			said = err.Error()
		}
		return "", fmt.Errorf("tools/bin/dev-version says no version: %s", said)
	}
	return version, nil
}

// buildNative builds ./cmd/itos into dir for this machine, stamped with the checkout's version,
// and returns the binary's path. go build's output goes to stderr.
func buildNative(root, dir string, stderr io.Writer) (string, error) {
	name := "itos"
	if runtime.GOOS == "windows" {
		name = "itos.exe"
	}
	binary, err := filepath.Abs(filepath.Join(dir, name))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		return "", err
	}
	version, err := devVersion(root)
	if err != nil {
		return "", err
	}
	args := []string{"build", "-trimpath", "-ldflags", "-s -w -X " + module + "/internal/version.stamp=" + version, "-o", binary, "./cmd/itos"}
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stdout, cmd.Stderr = stderr, stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go %s failed (%v)", strings.Join(args, " "), err)
	}
	return binary, nil
}

// buildRelease builds the release as a GoReleaser snapshot stamped with the checkout's version,
// or ITOS_SNAPSHOT_VERSION when it is set, copies what a release publishes into dir, and returns
// their paths, checksums.txt last. GoReleaser's log goes to stderr.
func buildRelease(root, dir string, stderr io.Writer) ([]string, error) {
	version := os.Getenv("ITOS_SNAPSHOT_VERSION")
	if version == "" {
		var err error
		if version, err = devVersion(root); err != nil {
			return nil, err
		}
	}
	out, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, err
	}
	args := []string{"goreleaser", "release", "--snapshot", "--clean"}
	name, argv := repoprogram.Command(filepath.Join(root, "tools", "bin", "pinned"), args...)
	cmd := exec.Command(name, argv...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "ITOS_SNAPSHOT_VERSION="+version)
	cmd.Stdout, cmd.Stderr = stderr, stderr // GoReleaser's log, off this command's stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("tools/bin/pinned %s failed (%v)", strings.Join(args, " "), err)
	}
	dist := filepath.Join(root, "dist", "goreleaser")
	names, err := published(dist)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, name := range names {
		from := filepath.Join(dist, name)
		if name == schema {
			from = filepath.Join(root, ".tools", "release", schema)
		}
		to := filepath.Join(out, name)
		if err := copyFile(from, to); err != nil {
			return nil, err
		}
		paths = append(paths, to)
	}
	return paths, nil
}

// checksumLine splits a line of checksums.txt into its fields, as sha256sum writes it: the
// checksum, then the file's name after white space and a binary mode's asterisk.
var checksumLine = regexp.MustCompile(`\s+\*?`)

// published is what a release publishes, by name: the archives and the schema, as checksums.txt
// lists them, then checksums.txt.
func published(dist string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(dist, "checksums.txt"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		fields := checksumLine.Split(line, -1)
		if len(fields) < 2 {
			return nil, fmt.Errorf("checksums.txt: %q names no file", line)
		}
		names = append(names, fields[1])
	}
	return append(names, "checksums.txt"), nil
}

// copyFile copies the file from to the path to.
func copyFile(from, to string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, data, 0o644)
}
