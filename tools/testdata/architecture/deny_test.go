package architecture_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// nestedDenies refuses a depguard deny list holding an entry that prefixes
// another. depguard sorts a list and looks an import up as the one entry
// sorting just before it, by prefix, an entry ending in $ matching only
// itself: a deny prefixing a longer one is hidden from every import sorting
// after the longer, so those imports are let by. An entry ending in $
// prefixes nothing, and the longer's trailing $ is left off, as depguard
// leaves it off when it sorts. Taken from itos-cc (its T-167).
func nestedDenies(rule string, denies []string) []string {
	sorted := slices.Sorted(slices.Values(denies))
	var refusals []string
	for _, shorter := range sorted {
		if strings.HasSuffix(shorter, "$") {
			continue
		}
		for _, longer := range sorted {
			trimmed := strings.TrimRight(longer, "$")
			if trimmed != shorter && strings.HasPrefix(trimmed, shorter) {
				refusals = append(refusals, fmt.Sprintf("depguard rule %q denies %q, which prefixes its %q: depguard looks a deny up as the one entry sorting just before the import, so imports sorting after %q are let by; give the longer a rule of its own", rule, shorter, longer, longer))
			}
		}
	}
	return refusals
}

// TestNoDenyEntryPrefixesAnother refuses, with no linter or environment
// variable, a depguard rule of the repository's .golangci.yml whose deny
// list nests, its table proving the refusal first.
func TestNoDenyEntryPrefixesAnother(t *testing.T) {
	module := "github.com/donvargax/itos/v8"
	for _, c := range []struct {
		name     string
		denies   []string
		refusals []string
	}{
		{"nested", []string{module, module + "/internal/slices/alpha/adapters"}, []string{fmt.Sprintf("%q, which prefixes its %q", module, module+"/internal/slices/alpha/adapters")}},
		{"nested exact longer", []string{module + "/internal/slices/", module + "/internal/slices/alpha$"}, []string{fmt.Sprintf("%q, which prefixes its %q", module+"/internal/slices/", module+"/internal/slices/alpha$")}},
		{"exact shorter", []string{module + "/internal/git$", module + "/internal/git/sub"}, nil},
		{"slice-domain-io", []string{"os", "net"}, nil},
		{"mocks", []string{"github.com/golang/mock", "go.uber.org/mock", "github.com/stretchr/testify/mock", "github.com/vektra/mockery", "github.com/maxbrunsfeld/counterfeiter"}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			refusals := nestedDenies(c.name, c.denies)
			if len(refusals) != len(c.refusals) {
				t.Fatalf("refusals: got %q, want ones saying %q", refusals, c.refusals)
			}
			for i, text := range c.refusals {
				for _, said := range []string{fmt.Sprintf("depguard rule %q", c.name), text} {
					if !strings.Contains(refusals[i], said) {
						t.Errorf("the refusal does not say %q: %s", said, refusals[i])
					}
				}
			}
		})
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "..", ".golangci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Linters struct {
			Settings struct {
				Depguard struct {
					Rules map[string]struct {
						Deny []struct {
							Pkg string `yaml:"pkg"`
						} `yaml:"deny"`
					} `yaml:"rules"`
				} `yaml:"depguard"`
			} `yaml:"settings"`
		} `yaml:"linters"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	rules := config.Linters.Settings.Depguard.Rules
	if len(rules) == 0 {
		t.Fatal(".golangci.yml has no depguard rules where this test reads them, linters.settings.depguard.rules")
	}
	denied := 0
	for name, rule := range rules {
		var denies []string
		for _, deny := range rule.Deny {
			denies = append(denies, deny.Pkg)
		}
		denied += len(denies)
		for _, refusal := range nestedDenies(name, denies) {
			t.Error(refusal)
		}
	}
	if denied == 0 {
		t.Fatal(".golangci.yml's depguard rules deny nothing where this test reads them, each rule's deny[].pkg")
	}
}
