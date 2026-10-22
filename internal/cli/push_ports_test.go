package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// pushPorts bind the push slice to the real code: the config file at its
// path in the working tree, the stealth check of it, its load and ci.watch's
// provider, none by default.
func TestPushPortsBindTheRealCode(t *testing.T) {
	gitConfigRepo(t, "version: 1\nwork: { registry: work-items.yaml }\n")
	var p pushPorts
	if !p.ConfigFound(t.Context()) || p.Stealth(t.Context()) {
		t.Errorf("ConfigFound %v, Stealth %v, want the working tree's config", p.ConfigFound(t.Context()), p.Stealth(t.Context()))
	}
	cfg, err := p.LoadConfig(t.Context())
	if err != nil || cfg.Work.Registry != "work-items.yaml" {
		t.Fatalf("LoadConfig = %+v, %v, want the working tree's config", cfg, err)
	}
	if _, ok, err := p.Watcher(t.Context(), cfg, "origin"); ok || err != nil {
		t.Errorf("Watcher = %v, %v, want the provider none", ok, err)
	}
	if err := os.Remove("itos.yaml"); err != nil {
		t.Fatal(err)
	}
	if p.ConfigFound(t.Context()) {
		t.Errorf("ConfigFound with no config file")
	}
	writeFiles(t, map[string]string{filepath.Join(".git", "itos", "itos.yaml"): "version: 1\n"})
	if !p.ConfigFound(t.Context()) || !p.Stealth(t.Context()) {
		t.Errorf("ConfigFound %v, Stealth %v, want the stealth config", p.ConfigFound(t.Context()), p.Stealth(t.Context()))
	}
}
