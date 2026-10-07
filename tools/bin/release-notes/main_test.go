package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// upgrading.json holds what the Upgrading section quotes, in its order, with
// previous the range's start as X.Y.Z and every list empty rather than null.
func TestAsset(t *testing.T) {
	n := notes{
		Version: "3.1.0",
		From:    "v3.0.0",
		Commits: []commit{
			{SHA: "a1", Header: "feat!: drop work.adr", Breaking: "feat!: drop work.adr"},
			{SHA: "b2", Header: "fix: a bug"},
			{SHA: "c3", Header: "feat: a thing", Breaking: "Set work.decisions.\nThen delete docs/adr."},
		},
		Upgrading: []footer{{SHA: "c3", Subject: "feat: a thing", Text: "Move the records."}},
		Changes:   []footer{{SHA: "b2", Subject: "fix: a bug", Text: "ask.yaml: ask record writes"}},
		Config:    []finding{{Path: "work.adr", Change: "key removed", Breaking: true}},
	}
	got, err := json.Marshal(n.asset())
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema":1,"version":"3.1.0","previous":"3.0.0",` +
		`"breaking":[{"commit":"a1","header":"feat!: drop work.adr","text":"feat!: drop work.adr"},` +
		`{"commit":"c3","header":"feat: a thing","text":"Set work.decisions.\nThen delete docs/adr."}],` +
		`"upgrading":[{"commit":"c3","header":"feat: a thing","text":"Move the records."}],` +
		`"changes":[{"commit":"b2","header":"fix: a bug","text":"ask.yaml: ask record writes"}],` +
		`"config":[{"key":"work.adr","change":"key removed"}]}`
	if string(got) != want {
		t.Errorf("asset:\n got %s\nwant %s", got, want)
	}

	empty, err := json.Marshal(notes{Version: "0.1.0"}.asset())
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"schema":1,"version":"0.1.0","previous":"","breaking":[],"upgrading":[],"changes":[],"config":[]}`; string(empty) != want {
		t.Errorf("a first release's asset:\n got %s\nwant %s", empty, want)
	}
}

// A release candidate's notes say it is a pre-release, of which release, and
// how to pin it; a release's say nothing of the kind. The version taken is
// X.Y.Z or a candidate of it, X.Y.Z-rc.N (T-118).
func TestPrereleaseNotes(t *testing.T) {
	for _, v := range []string{"7.0.0", "7.0.0-rc.1", "7.0.0-rc.12"} {
		if !semver.MatchString(v) {
			t.Errorf("-version %s is refused", v)
		}
	}
	for _, v := range []string{"7.0.0-beta.1", "7.0.0-rc", "7.0.0-rc.1+b", "v7.0.0"} {
		if semver.MatchString(v) {
			t.Errorf("-version %s is taken", v)
		}
	}
	hashes := map[string]string{}
	for _, p := range platforms {
		hashes[p] = "00"
	}
	rc := notes{Version: "7.0.0-rc.2", Candidate: "7.0.0", From: "v6.5.0", Hashes: hashes, Module: modulePath("7.0.0-rc.2")}
	text, err := rc.render()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# itos 7.0.0-rc.2\n", "**A pre-release**, a candidate for 7.0.0",
		"never as latest", "`itos pin 7.0.0-rc.2` pins this one", "run from v6.5.0, the last\nstable release",
		"go install github.com/donvargax/itos/v7/cmd/itos@v7.0.0-rc.2"} {
		if !strings.Contains(text, want) {
			t.Errorf("an rc's notes lack %q:\n%s", want, text)
		}
	}
	final := notes{Version: "7.0.0", From: "v6.5.0", Hashes: hashes, Module: modulePath("7.0.0")}
	text, err = final.render()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "pre-release") {
		t.Errorf("a release's notes say pre-release:\n%s", text)
	}
}
