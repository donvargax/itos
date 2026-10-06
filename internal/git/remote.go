package git

import (
	"strings"

	"github.com/donvargax/itos/v5/internal/kind"
)

// A fetch or a push that fails says why only in git's own words on stderr,
// and exits 128 whatever the reason, a code that is none of itos's (slice 90,
// decision 36's rule 31). RemoteFailure and Refused read those words, so that
// itos push exits by the kind of the failure: a remote out of reach is worth
// trying again (75), a remote that is no repository is missing (3), and a push
// the remote or a hook refused is a check that said no (1). The matching is
// narrow and on the messages git and its transports have printed for years,
// in English, as git prints them whatever the platform: a path in a message
// is never read, so a Windows path is matched as a POSIX one is.

// unreachable are the words of a remote git cannot reach: a host that does
// not resolve (curl's "Could not resolve host", ssh's "Could not resolve
// hostname"), a connection refused or timed out (ssh, and git:// as
// errno=Connection refused), and curl's "Failed to connect to", which every
// version of it prints before the reason.
var unreachable = []string{
	"Could not resolve host",
	"Connection refused",
	"Connection timed out",
	"Failed to connect to",
}

// missing are the words of a remote that is no repository, or that git
// cannot find: a local path that holds none, and a host's answer that it has
// no such repository (GitHub's "remote: Repository not found." over https and
// "ERROR: Repository not found." over ssh).
var missing = []string{
	"does not appear to be a git repository",
	"Repository not found",
}

// RemoteFailure is the kind of a fetch's or a push's failure, read from
// git's stderr: kind.Temporary for a remote out of reach, kind.Missing for
// one that is no repository or cannot be found, and kind.Unknown for any
// other failure, a refusal among them (Refused).
func RemoteFailure(stderr string) kind.Kind {
	if Refused(stderr) {
		return kind.Unknown
	}
	for _, words := range unreachable {
		if strings.Contains(stderr, words) {
			return kind.Temporary
		}
	}
	for _, words := range missing {
		if strings.Contains(stderr, words) {
			return kind.Missing
		}
	}
	for _, line := range strings.Split(stderr, "\n") {
		// A local path git cannot open, as git names it on a fatal line;
		// only there, for a hook or a tool can print the same words.
		if strings.HasPrefix(line, "fatal: ") && strings.Contains(line, "No such file or directory") {
			return kind.Missing
		}
	}
	return kind.Unknown
}

// Refused is whether git's stderr says the push reached the remote and was
// refused: a ref rejected, by the remote or its hooks, or the pre-push hook
// failing. git ends each with "failed to push some refs", and a pre-push
// hook's own output, which may say anything, comes only with it: git runs the
// hook once it has reached the remote.
func Refused(stderr string) bool {
	return strings.Contains(stderr, "failed to push some refs")
}
