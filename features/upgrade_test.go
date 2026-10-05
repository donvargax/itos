// The steps of itos upgrade (upgrade-command.feature, slice 75): a release
// server whose releases each publish an upgrading.json (T-091), naming the
// one before them, with what each asks; the config's schema line; and the
// install script, tools/bin/install-itos, written for a release of the
// server: its version= line and its platform's sum= hash, from that
// release's checksums.txt.
package features

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cucumber/godog"
)

// installScript is where a project keeps its install script (v2.0.0's
// Upgrading).
const installScript = "tools/bin/install-itos"

// upgradingAsset is a release's upgrading.json, as tools/bin/release-notes
// -json writes it.
type upgradingAsset struct {
	Schema    int              `json:"schema"`
	Version   string           `json:"version"`
	Previous  string           `json:"previous"`
	Breaking  []upgradingEntry `json:"breaking"`
	Upgrading []upgradingEntry `json:"upgrading"`
	Changes   []upgradingEntry `json:"changes"`
	Config    []upgradingKey   `json:"config"`
}

type upgradingEntry struct {
	Commit string `json:"commit"`
	Header string `json:"header"`
	Text   string `json:"text"`
}

type upgradingKey struct {
	Key    string `json:"key"`
	Change string `json:"change"`
}

func initializeUpgradeSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a release server offering the versions "([^"]*)", "([^"]*)" and "([^"]*)", each with an upgrading\.json naming the one before it$`,
		w.releasesWithUpgrading)
	sc.Step(`^the release "([^"]*)" asks "([^"]*)"$`, func(version, text string) error {
		return w.editUpgrading(version, func(a *upgradingAsset) {
			a.Upgrading = append(a.Upgrading, upgradingEntryOf(version, "feat: what "+version+" asks", text))
		})
	})
	sc.Step(`^the release "([^"]*)" breaks with "([^"]*)"$`, func(version, text string) error {
		return w.editUpgrading(version, func(a *upgradingAsset) {
			a.Breaking = append(a.Breaking, upgradingEntryOf(version, "feat!: what "+version+" breaks", text))
		})
	})
	sc.Step(`^the release "([^"]*)" changes "([^"]*)" on purpose$`, func(version, id string) error {
		return w.editUpgrading(version, func(a *upgradingAsset) {
			a.Changes = append(a.Changes, upgradingEntryOf(version, "fix: what "+version+" fixes", id))
		})
	})
	sc.Step(`^the release "([^"]*)" removes the config key "([^"]*)"$`, func(version, key string) error {
		return w.editUpgrading(version, func(a *upgradingAsset) {
			a.Config = append(a.Config, upgradingKey{Key: key, Change: "key removed"})
		})
	})
	sc.Step(`^the release "([^"]*)" has no upgrading\.json$`, w.noUpgrading)
	sc.Step(`^the config's first line is the schema line of the version "([^"]*)" of the release server$`, w.schemaLineIs)
	sc.Step(`^the repository has the install script of the version "([^"]*)" of the release server$`, w.hasInstallScript)

	sc.Step(`^the config's first line is now the schema line of the version "([^"]*)" of the release server$`, w.configStartsWithSchemaLine)
	sc.Step(`^the install script is the one of the version "([^"]*)" of the release server$`, w.installScriptIs)
	sc.Step(`^nothing was committed$`, w.noCommitMade)
}

// The release server with each version's release, the last the newest, and
// each one's upgrading.json, asking nothing yet, whose previous is the
// version before it ("" for the first).
func (w *world) releasesWithUpgrading(a, b, c string) error {
	versions := []string{a, b, c}
	if err := w.startReleaseServer(versions...); err != nil {
		return err
	}
	for i, version := range versions {
		previous := ""
		if i > 0 {
			previous = versions[i-1]
		}
		asset := upgradingAsset{Schema: 1, Version: version, Previous: previous,
			Breaking: []upgradingEntry{}, Upgrading: []upgradingEntry{}, Changes: []upgradingEntry{}, Config: []upgradingKey{}}
		if err := w.setUpgrading(version, asset); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) setUpgrading(version string, asset upgradingAsset) error {
	body, err := json.MarshalIndent(asset, "", "  ")
	if err != nil {
		return err
	}
	w.releases.set(releasePath(version, "upgrading.json"), append(body, '\n'))
	return nil
}

// The version's upgrading.json, read from the server, changed and served
// again.
func (w *world) editUpgrading(version string, change func(*upgradingAsset)) error {
	if err := w.needReleases(); err != nil {
		return err
	}
	body := w.releases.get(releasePath(version, "upgrading.json"))
	if body == nil {
		return fmt.Errorf("the release server has no upgrading.json for %s", version)
	}
	var asset upgradingAsset
	if err := json.Unmarshal(body, &asset); err != nil {
		return err
	}
	change(&asset)
	return w.setUpgrading(version, asset)
}

// An entry of a commit of the release, its SHA made up from what it holds.
func upgradingEntryOf(version, header, text string) upgradingEntry {
	sha := sha256Hex([]byte(version + header + text))[:40]
	return upgradingEntry{Commit: sha, Header: header, Text: text}
}

func (w *world) noUpgrading(version string) error {
	if err := w.needReleases(); err != nil {
		return err
	}
	w.releases.mu.Lock()
	defer w.releases.mu.Unlock()
	delete(w.releases.files, releasePath(version, "upgrading.json"))
	return nil
}

// The schema line the release notes ask a project to keep at the config's
// top, for the version of the release server.
func (w *world) schemaLine(version string) string {
	return "# yaml-language-server: $schema=" + w.releases.server.URL + releasePath(version, "itos.schema.json")
}

func (w *world) schemaLineIs(version string) error {
	if err := w.needReleases(); err != nil {
		return err
	}
	w.config.schemaLine = w.schemaLine(version)
	return w.writeConfig()
}

func (w *world) configStartsWithSchemaLine(version string) error {
	if err := w.needReleases(); err != nil {
		return err
	}
	text, err := os.ReadFile(w.configPath())
	if err != nil {
		return err
	}
	first, _, _ := strings.Cut(string(text), "\n")
	if want := w.schemaLine(version); first != want {
		return fmt.Errorf("the config's first line is %q, not %q\n%s\n%s", first, want, text, w.report())
	}
	return nil
}

// The install script of the version, as the release notes write it, with
// the line of the platform the tests run on alone, its sum= the hash the
// version's checksums.txt gives that platform's archive.
func (w *world) installScriptOf(version string) (string, error) {
	if err := w.needReleases(); err != nil {
		return "", err
	}
	sums := w.releases.get(releasePath(version, "checksums.txt"))
	if sums == nil {
		return "", fmt.Errorf("the release server has no version %s", version)
	}
	asset := platformArchive(version)
	sum := ""
	for _, line := range strings.Split(string(sums), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 && fields[1] == asset {
			sum = fields[0]
		}
	}
	if sum == "" {
		return "", fmt.Errorf("the checksums.txt of %s lists no %s", version, asset)
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	return `#!/bin/sh
# tools/bin/install-itos: the pinned itos, into .tools/bin/
set -eu
version=` + version + `
cd "$(git rev-parse --show-toplevel)"
case "$(uname -s)-$(uname -m)" in
*) platform=` + platform + ` sum=` + sum + ` ;;
esac
case "$platform" in
windows-*) archive="itos-$version-$platform.zip" bin=.tools/bin/itos.exe ;;
*) archive="itos-$version-$platform.tar.gz" bin=.tools/bin/itos ;;
esac
echo "install-itos: itos $version in $bin"
`, nil
}

func (w *world) hasInstallScript(version string) error {
	script, err := w.installScriptOf(version)
	if err != nil {
		return err
	}
	full := filepath.Join(w.dir, installScript)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(script), 0o755)
}

func (w *world) installScriptIs(version string) error {
	want, err := w.installScriptOf(version)
	if err != nil {
		return err
	}
	text, err := os.ReadFile(filepath.Join(w.dir, installScript))
	if err != nil {
		return err
	}
	if string(text) != want {
		return fmt.Errorf("the install script is not the one of %s; it is:\n%s\nnot:\n%s\n%s", version, text, want, w.report())
	}
	return nil
}
