package git

import (
	"testing"

	"github.com/donvargax/itos/v7/internal/kind"
)

// What git 2.56 printed for each failing fetch or push, recorded, and what
// older curls and ssh print in the same places: the kind each is read as.
func TestRemoteFailure(t *testing.T) {
	const accessRights = "fatal: Could not read from remote repository.\n\n" +
		"Please make sure you have the correct access rights\nand the repository exists.\n"
	cases := []struct {
		name, stderr string
		want         kind.Kind
	}{
		{"https, connection refused (curl 8)",
			"fatal: unable to access 'https://127.0.0.1:1/itos.git/': Failed to connect to 127.0.0.1:1 after 0 ms: Could not connect to server\n",
			kind.Temporary},
		{"https, connection refused (older curl)",
			"fatal: unable to access 'https://example.com/itos.git/': Failed to connect to example.com port 443: Connection refused\n",
			kind.Temporary},
		{"https, connection timed out",
			"fatal: unable to access 'https://example.com/itos.git/': Failed to connect to example.com port 443 after 134512 ms: Connection timed out\n",
			kind.Temporary},
		{"https, host that does not resolve",
			"fatal: unable to access 'https://nosuchhost.invalid/x.git/': Could not resolve host: nosuchhost.invalid\n",
			kind.Temporary},
		{"ssh, connection refused",
			"ssh: connect to host 127.0.0.1 port 1: Connection refused\n" + accessRights,
			kind.Temporary},
		{"ssh, connection timed out",
			"ssh: connect to host example.com port 22: Connection timed out\n" + accessRights,
			kind.Temporary},
		{"ssh, host that does not resolve",
			"ssh: Could not resolve hostname nosuchhost.invalid: Name or service not known\n" + accessRights,
			kind.Temporary},
		{"git://, connection refused",
			"fatal: unable to connect to 127.0.0.1:\n127.0.0.1[0: 127.0.0.1]: errno=Connection refused\n\n",
			kind.Temporary},
		{"a local folder that is no repository",
			"fatal: '../notrepo' does not appear to be a git repository\n" + accessRights,
			kind.Missing},
		{"a file:// URL to no repository",
			"fatal: '/nonexistent' does not appear to be a git repository\n" + accessRights,
			kind.Missing},
		{"a Windows path to no repository",
			"fatal: 'C:\\Users\\ana\\remotes\\itos' does not appear to be a git repository\n" + accessRights,
			kind.Missing},
		{"a Windows path git cannot open",
			"fatal: 'D:/remotes/itos.git': No such file or directory\n" + accessRights,
			kind.Missing},
		{"GitHub over ssh, no such repository",
			"ERROR: Repository not found.\n" + accessRights,
			kind.Missing},
		{"GitHub over https, no such repository",
			"remote: Repository not found.\nfatal: repository 'https://github.com/ana/nothing.git/' not found\n",
			kind.Missing},
		{"ssh, no key the host takes",
			"git@github.com: Permission denied (publickey).\n" + accessRights,
			kind.Unknown},
		{"a rejected ref",
			"To ../origin.git\n ! [rejected]        main -> main (fetch first)\n" +
				"error: failed to push some refs to '../origin.git'\n",
			kind.Unknown},
		{"a pre-push hook that says the words of a remote out of reach",
			"dial tcp 127.0.0.1:8080: connect: Connection refused\n" +
				"error: failed to push some refs to '../origin.git'\n",
			kind.Unknown},
		{"a hook's missing file, not on a fatal line",
			"open testdata/x: No such file or directory\n",
			kind.Unknown},
		{"nothing said", "", kind.Unknown},
	}
	for _, c := range cases {
		if got := RemoteFailure(c.stderr); got != c.want {
			t.Errorf("%s: kind %d, want %d", c.name, got, c.want)
		}
	}
}

// A push the remote or the pre-push hook refused says so; a remote out of
// reach was never reached to refuse anything.
func TestRefused(t *testing.T) {
	if !Refused(" ! [remote rejected] main -> main (pre-receive hook declined)\nerror: failed to push some refs to 'origin'\n") {
		t.Error("a ref the remote rejected is not read as refused")
	}
	if Refused("ssh: connect to host 127.0.0.1 port 1: Connection refused\nfatal: Could not read from remote repository.\n") {
		t.Error("a remote out of reach is read as refused")
	}
}
