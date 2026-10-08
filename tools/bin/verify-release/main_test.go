package main

import (
	"strings"
	"testing"
)

// digestOf is a sha256 of a string as the manifest and the API write it.
func digestOf(s string) string { return digest([]byte(s)) }

func held(name, body string) asset {
	return asset{Name: name, Size: int64(len(body)), Digest: "sha256:" + digestOf(body)}
}

func TestVerifyAcceptsAReleaseItsManifestDescribes(t *testing.T) {
	manifest := strings.Join([]string{
		digestOf("archive") + "  itos-7.0.0-linux-amd64.tar.gz",
		digestOf("schema") + "  itos.schema.json",
		"",
	}, "\n")
	rel := release{
		TagName: "v7.0.0",
		Assets: []asset{
			held("checksums.txt", manifest),
			held("itos-7.0.0-linux-amd64.tar.gz", "archive"),
			held("itos.schema.json", "schema"),
			held("upgrading.json", "{}"),
		},
	}
	if problems := verify(rel, rel.Assets[0], []byte(manifest)); len(problems) > 0 {
		t.Fatalf("a release its manifest describes: %v", problems)
	}
}

func TestVerifyNamesEveryWayItCanFail(t *testing.T) {
	manifest := strings.Join([]string{
		digestOf("archive") + "  itos-7.0.0-linux-amd64.tar.gz",
		digestOf("schema") + "  itos.schema.json",
		digestOf("gone") + "  itos-7.0.0-linux-arm64.tar.gz",
		digestOf("empty") + "  itos-7.0.0-darwin-amd64.tar.gz",
		digestOf("other") + "  itos-7.0.0-darwin-arm64.tar.gz",
		"",
	}, "\n")
	rel := release{
		TagName: "v7.0.0",
		Assets: []asset{
			held("checksums.txt", manifest),
			held("itos-7.0.0-linux-amd64.tar.gz", "a tampered archive"),
			held("itos.schema.json", "schema"),
			{Name: "itos-7.0.0-darwin-amd64.tar.gz", Size: 0},
			held("itos-7.0.0-darwin-arm64.tar.gz", "other bytes"),
			held("upgrading.json", "{}"),
		},
	}
	problems := verify(rel, rel.Assets[0], []byte(manifest))
	joined := strings.Join(problems, "\n")
	for _, want := range []string{
		"itos-7.0.0-linux-amd64.tar.gz is sha256:",               // off its digest
		"names itos-7.0.0-linux-arm64.tar.gz, which the release", // named, not there
		"itos-7.0.0-darwin-amd64.tar.gz is empty",                // there, nothing in it
		"checksums.txt gives it " + digestOf("other"),            // there, other bytes
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("no problem says %q in:\n%s", want, joined)
		}
	}
}

func TestVerifyHoldsTheManifestToItsOwnDigest(t *testing.T) {
	manifest := digestOf("archive") + "  itos-7.0.0-linux-amd64.tar.gz\n"
	rel := release{
		TagName: "v7.0.0",
		Assets: []asset{
			{Name: "checksums.txt", Size: int64(len(manifest)), Digest: "sha256:" + digestOf("another manifest")},
			held("itos-7.0.0-linux-amd64.tar.gz", "archive"),
		},
	}
	problems := strings.Join(verify(rel, rel.Assets[0], []byte(manifest)), "\n")
	if !strings.Contains(problems, "GitHub's digest for it is sha256:"+digestOf("another manifest")) {
		t.Errorf("a manifest GitHub's digest does not match is not named: %s", problems)
	}
}

func TestVerifyNamesAManifestItCannotRead(t *testing.T) {
	rel := release{TagName: "v7.0.0", Assets: []asset{held("checksums.txt", "not a manifest")}}
	problems := strings.Join(verify(rel, rel.Assets[0], []byte("not a manifest")), "\n")
	if !strings.Contains(problems, `is not "<digest>  <name>"`) {
		t.Errorf("a line that is not an entry is not named: %s", problems)
	}
}

func TestParseReadsBothOfSha256sumsFormats(t *testing.T) {
	entries, bad := parse("checksums.txt", strings.Join([]string{
		"# a comment",
		"",
		digestOf("a") + "  itos-7.0.0-linux-amd64.tar.gz",
		digestOf("b") + " *itos.schema.json",
	}, "\n"))
	if len(bad) > 0 {
		t.Fatalf("sha256sum's two formats: %v", bad)
	}
	if len(entries) != 2 || entries[0].Name != "itos-7.0.0-linux-amd64.tar.gz" ||
		entries[1].Name != "itos.schema.json" || entries[1].Digest != digestOf("b") {
		t.Fatalf("sha256sum's two formats read as %v", entries)
	}
}
