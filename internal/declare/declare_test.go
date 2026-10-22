package declare

import "testing"

// Each constructor makes the flag its name says: what it takes and whether
// it may be repeated.
func TestConstructors(t *testing.T) {
	for _, c := range []struct {
		got, want Flag
	}{
		{Switch("--all"), Flag{Name: "--all", Takes: Bare}},
		{Value("--as"), Flag{Name: "--as", Takes: Needs}},
		{Values("--task"), Flag{Name: "--task", Takes: Needs, Repeat: true}},
		{Optional("--plugin"), Flag{Name: "--plugin", Takes: May}},
	} {
		if c.got != c.want {
			t.Errorf("%+v, not %+v", c.got, c.want)
		}
	}
}

// The zero values are the defaults a declaration leaves out: a switch, no
// flags from the config.
func TestZeroValues(t *testing.T) {
	if Bare != 0 || NoMore != 0 {
		t.Errorf("Bare %d, NoMore %d: a flag or spec that names neither must mean them", Bare, NoMore)
	}
}
