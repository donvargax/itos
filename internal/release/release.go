// Package release is where itos's releases are and how one is asked for:
// the base they are fetched from (ITOS_RELEASES, the GitHub releases of itos
// by default), a version's assets at <base>/download/v<version>/<asset>, the
// newest release's at <base>/latest/download/<asset>, as GitHub serves its
// latest release, and its notes at <base>/tag/v<version>. A release's
// checksums.txt lists its archives, whose names carry its version, and its
// SHA-256 is what a pin holds (pin.checksums). The launcher (internal/launch)
// fetches and runs releases through it, and itos pin (internal/cli) reads one
// to move a pin, and itos upgrade each release's upgrading.json (slice 75).
package release

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/donvargax/itos/v5/internal/kind"
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

// Get is the body at the address, or why it cannot be had within timeout,
// with its kind (slice 86): a server that cannot be reached, does not answer
// in time, fails (5xx) or limits the rate (429) is kind.Temporary, since
// asking again may answer; any other answer, as a 404 for a release the
// server does not have, is kind.Missing.
func Get(url string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, kind.Wrap(kind.Temporary, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		status := &StatusError{URL: url, Status: resp.Status, Code: resp.StatusCode}
		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			return nil, kind.Wrap(kind.Temporary, status)
		}
		return nil, kind.Wrap(kind.Missing, status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, kind.Wrap(kind.Temporary, fmt.Errorf("%s: %w", url, err))
	}
	return body, nil
}

// StatusError is an answer other than 200 OK: its address and its status, so
// a caller can tell an asset the release does not have (404) from a server
// that failed.
type StatusError struct {
	URL, Status string
	Code        int
}

func (e *StatusError) Error() string { return fmt.Sprintf("%s: %s", e.URL, e.Status) }

// NotFound is whether the error is an answer saying the address has nothing.
func NotFound(err error) bool {
	var s *StatusError
	return errors.As(err, &s) && s.Code == http.StatusNotFound
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

// Listed is the SHA-256 checksums.txt gives the asset, as sha256sum writes a
// line ("<hex>  <name>", or "<hex> *<name>" in binary mode), in lowercase.
func Listed(sums []byte, asset string) (string, bool) {
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == asset {
			return strings.ToLower(fields[0]), true
		}
	}
	return "", false
}
