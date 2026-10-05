package main

import (
	"encoding/json"
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
