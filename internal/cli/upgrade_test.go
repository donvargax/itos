package cli

import (
	"strings"
	"testing"
)

const (
	oldSum   = "1111111111111111111111111111111111111111111111111111111111111111"
	linuxSum = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	winSum   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// newSums is a checksums.txt of 9.3.0 for linux-amd64 and windows-amd64.
var newSums = []byte(linuxSum + "  itos-9.3.0-linux-amd64.tar.gz\n" +
	winSum + "  itos-9.3.0-windows-amd64.zip\n" +
	"cccc  itos.schema.json\n")

func script(version, linux, windows string) string {
	return "#!/bin/sh\nset -eu\nversion=" + version + "\n" +
		"case \"$(uname -s)-$(uname -m)\" in\n" +
		"Linux-x86_64) platform=linux-amd64 sum=" + linux + " ;;\n" +
		"MINGW*-x86_64 | MSYS*-x86_64) platform=windows-amd64 sum=" + windows + " ;;\n" +
		"esac\nversion_note=\"version=$version\"\n"
}

func TestMoveInstallScript(t *testing.T) {
	got, err := moveInstallScript(script("9.1.0", oldSum, oldSum), "9.3.0", newSums)
	if err != nil {
		t.Fatal(err)
	}
	if want := script("9.3.0", linuxSum, winSum); got != want {
		t.Errorf("moved script:\n%s\nwant:\n%s", got, want)
	}
}

func TestMoveInstallScriptRefuses(t *testing.T) {
	cases := map[string]string{
		"no version line":   strings.Replace(script("9.1.0", oldSum, oldSum), "version=9.1.0\n", "", 1),
		"two version lines": "version=9.1.0\n" + script("9.1.0", oldSum, oldSum),
		"a platform the release lacks": script("9.1.0", oldSum, oldSum) +
			"Darwin-arm64) platform=darwin-arm64 sum=" + oldSum + " ;;\n",
	}
	for name, text := range cases {
		if got, err := moveInstallScript(text, "9.3.0", newSums); err == nil {
			t.Errorf("%s: moved, to\n%s", name, got)
		}
	}
}

func TestScriptVersion(t *testing.T) {
	if v, err := scriptVersion(script("9.1.0", oldSum, oldSum)); err != nil || v != "9.1.0" {
		t.Errorf("scriptVersion = %q, %v", v, err)
	}
	if v, err := scriptVersion("version=latest\n"); err == nil {
		t.Errorf("scriptVersion of version=latest = %q", v)
	}
}

func TestMoveSchemaLine(t *testing.T) {
	const line = "# yaml-language-server: $schema=https://example.test/r/download/v"
	cases := []struct{ text, want string }{
		{line + "9.1.0/itos.schema.json\nversion: 1\n", line + "9.3.0/itos.schema.json\nversion: 1\n"},
		{line + "9.1.0/itos.schema.json\r\nversion: 1\r\n", line + "9.3.0/itos.schema.json\r\nversion: 1\r\n"},
		{line + "9.1.0/itos.schema.json", line + "9.3.0/itos.schema.json"},
		// Not a release's schema, or not the first line: left alone.
		{"# yaml-language-server: $schema=./itos.schema.json\n", "# yaml-language-server: $schema=./itos.schema.json\n"},
		{"version: 1\n" + line + "9.1.0/itos.schema.json\n", "version: 1\n" + line + "9.1.0/itos.schema.json\n"},
	}
	for _, c := range cases {
		if got := moveSchemaLine(c.text, "9.3.0"); got != c.want {
			t.Errorf("moveSchemaLine(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

// latest is the newest, as no version is; anything else not a version is a
// usage error.
func TestVersionArg(t *testing.T) {
	for args, want := range map[string]string{"": "", "latest": "", "2.5.0": "2.5.0", "v2.5.0": "2.5.0"} {
		if got, err := versionArg("pin", strings.Fields(args)); err != nil || got != want {
			t.Errorf("versionArg(%s) = %q, %v; want %q", args, got, err, want)
		}
	}
	for _, args := range []string{"newest", "Latest", "vlatest", "latest 2.5.0", "--force"} {
		if _, err := versionArg("pin", strings.Fields(args)); err == nil {
			t.Errorf("versionArg(%s) took it", args)
		}
	}
}
