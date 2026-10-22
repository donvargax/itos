package ledger

import (
	"reflect"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
)

// The label's flag follows GroupNames when it is not one of them, and
// GroupNames itself never grows.
func TestGroupFlagsAreGroupNamesAndTheLabelsOwn(t *testing.T) {
	for _, c := range []struct {
		label string
		want  []string
	}{
		{"phase", []string{"--group", "--phase"}},
		{"group", []string{"--group", "--phase"}},
		{"milestone", []string{"--group", "--phase", "--milestone"}},
		{"sprint", []string{"--group", "--phase", "--sprint"}},
	} {
		cfg := &config.Loaded{}
		cfg.Ledger.Group.Label = c.label
		if got := GroupFlags(cfg); !reflect.DeepEqual(got, c.want) {
			t.Errorf("GroupFlags under the label %s = %q, want %q", c.label, got, c.want)
		}
	}
	if !reflect.DeepEqual(GroupNames, []string{"--group", "--phase"}) {
		t.Fatalf("GroupNames became %q", GroupNames)
	}
}
