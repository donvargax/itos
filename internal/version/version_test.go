package version

import "testing"

func TestSatisfies(t *testing.T) {
	cases := []struct {
		version, rng string
		want         bool
	}{
		{"0.6.0", ">=0.1.0", true},
		{"0.6.0", ">=0.0.9 <999", true},
		{"0.6.0", ">=0.0.1 <0.6.0", false},
		{"0.6.0", "0.6.0", true},
		{"0.6.0", "0", false},        // a bare version's missing parts are zero
		{"0.0.0", "0", true},         // so 0 is 0.0.0
		{"0.6.0", "=v0.6.0", true},   // a v prefix and = are allowed
		{"0.6.0", "~0.1", false},     // a comparator it cannot read never holds
		{"0.6.0", ">0.6.0", false},   // greater than is strict
		{"0.6.0", "<=0.6", true},     // 0.6 is 0.6.0
		{"1.2.3", " >=1  <2 ", true}, // spaces around and between
		{"1.2.3", "", true},
		{"(devel)", ">=0.1.0", false},
	}
	for _, c := range cases {
		if got := Satisfies(c.version, c.rng); got != c.want {
			t.Errorf("Satisfies(%q, %q) = %v, want %v", c.version, c.rng, got, c.want)
		}
	}
}

func TestFromModule(t *testing.T) {
	for in, want := range map[string]string{"v0.6.0": "0.6.0", "(devel)": Unstamped, "": Unstamped} {
		if got := fromModule(in); got != want {
			t.Errorf("fromModule(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLeadingInt(t *testing.T) {
	for in, want := range map[string]int{"12": 12, "3-rc": 3, "x": 0, "": 0, "-4": -4, " 7": 7} {
		if got := leadingInt(in); got != want {
			t.Errorf("leadingInt(%q) = %d, want %d", in, got, want)
		}
	}
}
