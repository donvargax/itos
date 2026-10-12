// Package release holds the self-test of the Go release (T-040, a Go test since T-143), built as
// the release workflow builds it: the archives, the config's JSON Schema and checksums.txt
// GoReleaser writes (.goreleaser.yaml) are what the release publishes and the launcher, a pin and
// a consumer's pinned install script download
// (docs/decisions/0022-itos-is-distributed-as-release-archives-with-checksums-installed-pinned.md),
// so they are proven before anyone relies on them.
//
// It builds the release from the checkout as a GoReleaser snapshot (tools/bin/pinned goreleaser
// release --snapshot --clean, stamped with tools/bin/dev-version's version, as
// tools/bin/build-go --release builds it) into a scratch folder, then reads the folder with
// Go's own archive and hash readers, not the code that wrote it:
//
//   - the folder holds one archive per platform of platforms below, named as the release names
//     them, itos.schema.json and checksums.txt, and nothing else;
//   - every archive lists exactly its binary (itos, or itos.exe), LICENSE and README.md, and its
//     binary is an executable of its platform's format and architecture;
//   - itos.schema.json is JSON (the go-schema self-test proves what it says);
//   - checksums.txt names every archive and the schema once, and each one's SHA-256 is the one
//     it gives;
//   - this machine's archive, unpacked, says itos <version> for itos version.
//
// The version is the one the archives are named with, which every archive must share. The
// Claude Code plugin's version is its own since T-069, so its manifest is not compared with
// itos's. The nightly runs it, so the release build cannot rot between releases.
package release

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
)

// platforms is what a release publishes for each platform (decision 22): the expectation,
// written here rather than read from the build's config, so a platform dropped there fails here.
var platforms = []string{"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"}

// schemaFile is the config's JSON Schema, beside the archives.
const schemaFile = "itos.schema.json"

func archiveOf(version, platform string) string {
	if strings.HasPrefix(platform, "windows") {
		return "itos-" + version + "-" + platform + ".zip"
	}
	return "itos-" + version + "-" + platform + ".tar.gz"
}

func binaryOf(platform string) string {
	if strings.HasPrefix(platform, "windows") {
		return "itos.exe"
	}
	return "itos"
}

// listed is an archive's files and the bytes of the one named binary.
func listed(archive, binary string) ([]string, []byte, error) {
	var names []string
	var data []byte
	if strings.HasSuffix(archive, ".zip") {
		z, err := zip.OpenReader(archive)
		if err != nil {
			return nil, nil, err
		}
		defer z.Close()
		for _, f := range z.File {
			names = append(names, f.Name)
			if f.Name == binary {
				rc, err := f.Open()
				if err != nil {
					return nil, nil, err
				}
				data, err = io.ReadAll(rc)
				rc.Close()
				if err != nil {
					return nil, nil, err
				}
			}
		}
		slices.Sort(names)
		return names, data, nil
	}
	f, err := os.Open(archive)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		names = append(names, h.Name)
		if h.Name == binary {
			if data, err = io.ReadAll(tr); err != nil {
				return nil, nil, err
			}
		}
	}
	slices.Sort(names)
	return names, data, nil
}

// platformOf is an executable's format and architecture, or "" for none known.
func platformOf(binary []byte) string {
	r := bytes.NewReader(binary)
	if f, err := elf.NewFile(r); err == nil {
		return map[elf.Machine]string{elf.EM_X86_64: "linux-amd64", elf.EM_AARCH64: "linux-arm64"}[f.Machine]
	}
	if f, err := macho.NewFile(r); err == nil {
		return map[macho.Cpu]string{macho.CpuAmd64: "darwin-amd64", macho.CpuArm64: "darwin-arm64"}[f.Cpu]
	}
	if f, err := pe.NewFile(r); err == nil {
		return map[uint16]string{pe.IMAGE_FILE_MACHINE_AMD64: "windows-amd64", pe.IMAGE_FILE_MACHINE_ARM64: "windows-arm64"}[f.Machine]
	}
	return ""
}

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	root := scratch.Root(t)
	out := t.TempDir()
	env := scratch.OutsideEnv()

	// The release build: GoReleaser's snapshot, stamped with the checkout's version, and what a
	// release publishes copied out of it (the schema from .tools/release, where its before hook
	// writes it), as checksums.txt lists them.
	devName, devArgs := scratch.Program(filepath.Join(root, "tools", "bin", "dev-version"), root)
	stamp := scratch.Run(root, env, "", devName, devArgs...)
	if stamp.Code != 0 || strings.TrimSpace(stamp.Stdout) == "" {
		t.Fatalf("tools/bin/dev-version says no version:\n%s", stamp.Output())
	}
	name, argv := scratch.Program(filepath.Join(root, "tools", "bin", "pinned"), "goreleaser", "release", "--snapshot", "--clean")
	build := scratch.Run(root, scratch.With(env, "ITOS_SNAPSHOT_VERSION="+strings.TrimSpace(stamp.Stdout)), "", name, argv...)
	if build.Code != 0 {
		t.Fatalf("tools/bin/pinned goreleaser release --snapshot --clean (exit %d):\n%s", build.Code, build.Output())
	}
	dist := filepath.Join(root, "dist", "goreleaser")
	sums, err := os.ReadFile(filepath.Join(dist, "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(sums)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("checksums.txt holds %q", line)
		}
		file := strings.TrimPrefix(fields[1], "*")
		from := filepath.Join(dist, file)
		if file == schemaFile {
			from = filepath.Join(root, ".tools", "release", schemaFile)
		}
		data, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, file), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(out, "checksums.txt"), sums, 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	var held []string
	for _, e := range entries {
		held = append(held, e.Name())
	}
	slices.Sort(held)
	// The version the folder's archives are named with: linux-amd64's name read back, which
	// every other archive must then match.
	version := ""
	named := regexp.MustCompile(`^itos-(.+)-linux-amd64\.tar\.gz$`)
	for _, h := range held {
		if m := named.FindStringSubmatch(h); m != nil {
			version = m[1]
		}
	}
	if version == "" {
		t.Fatalf("no archive is named itos-<version>-linux-amd64.tar.gz: %v", held)
	}
	var archives []string
	for _, p := range platforms {
		archives = append(archives, archiveOf(version, p))
	}

	t.Run("the release holds an archive per platform, the schema and checksums.txt, and nothing else", func(t *testing.T) {
		expected := append(append([]string{}, archives...), schemaFile, "checksums.txt")
		slices.Sort(expected)
		if !slices.Equal(held, expected) {
			t.Errorf("the release holds %v, not %v", held, expected)
		}
	})

	for _, platform := range platforms {
		t.Run(platform+"'s archive holds its binary, LICENSE and README.md, built for it", func(t *testing.T) {
			archive := archiveOf(version, platform)
			names, binary, err := listed(filepath.Join(out, archive), binaryOf(platform))
			if err != nil {
				t.Fatalf("%s: %v", archive, err)
			}
			expected := []string{binaryOf(platform), "LICENSE", "README.md"}
			slices.Sort(expected)
			if !slices.Equal(names, expected) {
				t.Errorf("%s holds %v, not %v", archive, names, expected)
			}
			if got := platformOf(binary); got != platform {
				t.Errorf("%s's binary is built for %q", archive, got)
			}
		})
	}

	t.Run("the schema is JSON", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(out, schemaFile))
		if err != nil {
			t.Fatal(err)
		}
		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			t.Errorf("%s is not JSON: %v", schemaFile, err)
		}
	})

	t.Run("checksums.txt names every archive and the schema once, each on its digest", func(t *testing.T) {
		var names []string
		for _, line := range strings.Split(strings.TrimSpace(string(sums)), "\n") {
			fields := strings.Fields(line)
			file := strings.TrimPrefix(fields[1], "*")
			names = append(names, file)
			data, err := os.ReadFile(filepath.Join(out, file))
			if err != nil {
				t.Errorf("checksums.txt names %s, which the release lacks", file)
				continue
			}
			sum := sha256.Sum256(data)
			if hex.EncodeToString(sum[:]) != fields[0] {
				t.Errorf("%s's SHA-256 is not the one checksums.txt gives", file)
			}
		}
		slices.Sort(names)
		expected := append(append([]string{}, archives...), schemaFile)
		slices.Sort(expected)
		if !slices.Equal(names, expected) {
			t.Errorf("checksums.txt names %v, not %v", names, expected)
		}
	})

	// This machine's archive, unpacked, says the version its name gives, run from the checkout,
	// whose itos.yaml itos version reads.
	t.Run("this machine's archive says its version", func(t *testing.T) {
		platform := runtime.GOOS + "-" + runtime.GOARCH
		if !slices.Contains(platforms, platform) {
			t.Fatalf("no archive is for this machine (%s)", platform)
		}
		_, binary, err := listed(filepath.Join(out, archiveOf(version, platform)), binaryOf(platform))
		if err != nil {
			t.Fatal(err)
		}
		exe := filepath.Join(t.TempDir(), binaryOf(platform))
		if err := os.WriteFile(exe, binary, 0o755); err != nil {
			t.Fatal(err)
		}
		r := scratch.Run(root, env, "", exe, "version")
		if r.Code != 0 || strings.TrimSpace(r.Stdout) != "itos "+version {
			t.Errorf("%s's itos says %q (exit %d), not itos %s", archiveOf(version, platform), strings.TrimSpace(r.Stdout), r.Code, version)
		}
	})
}
