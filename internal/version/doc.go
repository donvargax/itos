// Package version is itos's version, and whether it satisfies a config's
// requires: a list of comparators, each of >=, >, <=, <, = (or none) and a
// version, all of which must hold. Compare orders two versions by semver's
// precedence: a pre-release below its release, and pre-releases of one
// release by their identifiers, numbers as numbers (rc.10 above rc.2).
//
// The version is the release's tag, written nowhere in the tree: a
// release's GoReleaser stamps it into the binary with -ldflags
// "-X github.com/donvargax/itos/v6/internal/version.stamp=<v>", and
// tools/bin/itos and tools/bin/build-go.ts stamp a build of a checkout with
// the version tools/bin/dev-version reads from git describe: the release's
// at its tag, X.Y.(Z+1)-dev.N.g<sha> N commits after the newest vX.Y.Z tag
// (0.0.1-dev.N.g<sha> with none, 0.0.0-dev outside git). That is a
// pre-release of the next patch, so it sorts above the last release and
// below the next, and it carries no + build metadata, since pin.version and
// the launcher refuse a +. A config's requires reads X.Y.Z alone, so no such
// build can be named by one.
//
// A binary built without the stamp says the module version Go records, which
// go install github.com/donvargax/itos/v6/cmd/itos@v<x> sets (from v2 the
// module path ends in its major version, as Go requires of a v2 tag, so the
// path moves before a major release), or with none Unstamped, (devel), which
// runs itself where the launcher finds no config.
package version
