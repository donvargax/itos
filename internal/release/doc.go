// Package release is where itos's releases are and how one is asked for:
// the base they are fetched from (ITOS_RELEASES, the GitHub releases of itos
// by default), a version's assets at <base>/download/v<version>/<asset>
// (URL), the newest release's at <base>/latest/download/<asset> (LatestURL),
// as GitHub serves its latest release, and its notes at <base>/tag/v<version>
// (NotesURL). A release's checksums.txt lists its archives, whose names carry
// its version (VersionOf; Listed is a file's line), and its SHA-256 is what
// a pin holds (pin.checksums). The launcher (internal/launch) fetches and
// runs releases through it, itos pin reads one to move a pin, and itos
// upgrade each release's upgrading.json.
//
// Get makes a server it cannot reach a kind.Temporary error (exit 75), and
// reports an answer other than 200 OK as a StatusError, so NotFound tells a
// 404 apart: itos upgrade reads it as a release cut before it had an
// upgrading.json.
//
// The rules a release is cut by live here too, shared with
// tools/bin/release-version and itos status: Newest picks the highest
// vX.Y.Z tag, three numbers and nothing else ordered numerically (itos never
// cuts a prerelease, so a prerelease or build-metadata tag is never the
// newest release); Releasable is whether a commit's message releases
// anything, a feat, a fix, or a breaking change of any type (Breaking: a !
// before the header's colon, or a BREAKING-CHANGE footer).
package release
