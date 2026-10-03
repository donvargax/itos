// Package release is where itos's releases are and how one is asked for:
// the base they are fetched from (ITOS_RELEASES, the GitHub releases of itos
// by default), a version's assets at <base>/download/v<version>/<asset>, the
// newest release's at <base>/latest/download/<asset>, as GitHub serves its
// latest release, and its notes at <base>/tag/v<version>. A release's
// checksums.txt lists its archives, whose names carry its version, and its
// SHA-256 is what a pin holds (pin.checksums). The launcher (internal/launch)
// fetches and runs releases through it, and itos pin (internal/cli) reads one
// to move a pin.
package release

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// Env is the variable naming the base the releases are fetched from.
const Env = "ITOS_RELEASES"

// Default is where releases come from when ITOS_RELEASES is not set.
const Default = "https://github.com/donvargax/itos/releases"

// Base is the base the releases are fetched from: ITOS_RELEASES, else the
// GitHub releases of itos.
func Base() string {
	if base := strings.TrimRight(os.Getenv(Env), "/"); base != "" {
		return base
	}
	return Default
}

// URL is where the version's asset is fetched from.
func URL(v, asset string) string {
	return Base() + "/download/v" + v + "/" + asset
}

// LatestURL is where the newest release's asset is fetched from.
func LatestURL(asset string) string {
	return Base() + "/latest/download/" + asset
}

// NotesURL is the page of the version's release, its notes.
func NotesURL(v string) string {
	return Base() + "/tag/v" + v
}

// Get is the body at the address, or why it cannot be had within timeout.
func Get(url string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	return body, nil
}

// SHA256 is the bytes' SHA-256, in lowercase hex, as pin.checksums and a
// checksums.txt line write it.
func SHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// archiveLine is an archive's name in a release's checksums.txt, its version
// first.
var archiveLine = regexp.MustCompile(`^itos-(\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?)-[0-9a-z]+-[0-9a-z]+\.(?:tar\.gz|zip)$`)

// VersionOf is the version of the release whose checksums.txt this is, read
// from its archives' names, "" when it names none.
func VersionOf(sums []byte) string {
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if m := archiveLine.FindStringSubmatch(strings.TrimPrefix(fields[1], "*")); m != nil {
			return m[1]
		}
	}
	return ""
}
