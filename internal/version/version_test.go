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
		// A range reads the version's X.Y.Z alone, its pre-release left out,
		// though Compare orders it below its release (bug 50).
		{"6.5.1-dev.74.gabc", "6.5.1", true},
		{"6.5.1-dev.74.gabc", ">=0.0.1 <6.5.1", false},
		{"9.2.0-rc.1", ">=9.2.0", true},
		{"9.2.0+build.1", "=9.2.0", true},
	}
	for _, c := range cases {
		if got := Satisfies(c.version, c.rng); got != c.want {
			t.Errorf("Satisfies(%q, %q) = %v, want %v", c.version, c.rng, got, c.want)
		}
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int // the sign of Compare(a, b)
	}{
		{"9.2.0-rc.1", "9.2.0", -1},       // a release candidate below its release (bug 50)
		{"9.2.0-rc.2", "9.2.0-rc.10", -1}, // numbers as numbers
		{"9.2.0-rc.1", "9.1.0", 1},        // above the release before
		{"9.2.0-rc.1", "9.2.0-rc.1", 0},
		{"9.2.0", "9.2.0", 0},
		{"9.2.0", "9.1.9", 1}, // with no pre-release, the three numbers as before
		{"1.10.0", "1.9.0", 1},
		{"1.2.3.4", "1.2.3", 0},               // core components after the third are ignored
		{"1.2", "1.2.0", 0},                   // a missing part is 0
		{"9.2.0-rc.1", "9.2.0-rc", 1},         // more identifiers above fewer
		{"9.2.0-1", "9.2.0-rc", -1},           // a number below a word
		{"9.2.0-alpha", "9.2.0-beta", -1},     // words by their bytes
		{"9.2.0-rc.01", "9.2.0-rc.1", 0},      // leading zeros do not count
		{"9.2.0+build.1", "9.2.0+build.2", 0}, // build metadata does not count
		{"6.5.1-dev.73.gabc", "6.5.0", 1},     // a build of a checkout above the last release
		{"6.5.1-dev.73.gabc", "6.5.1", -1},    // and below the next
		{"6.5.1-dev.9.gabc", "6.5.1-dev.73.gabc", -1},
	}
	sign := func(n int) int {
		switch {
		case n < 0:
			return -1
		case n > 0:
			return 1
		}
		return 0
	}
	for _, c := range cases {
		if got := sign(Compare(c.a, c.b)); got != c.want {
			t.Errorf("Compare(%q, %q) has the sign %d, want %d", c.a, c.b, got, c.want)
		}
		if got := sign(Compare(c.b, c.a)); got != -c.want {
			t.Errorf("Compare(%q, %q) has the sign %d, want %d", c.b, c.a, got, -c.want)
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
	for in, want := range map[string]int{"12": 12, "101": 101, "1000": 1000, "3-rc": 3, "x": 0, "": 0, "-4": -4, " 7": 7} {
		if got := leadingInt(in); got != want {
			t.Errorf("leadingInt(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParts(t *testing.T) {
	for in, want := range map[string][3]int{
		"1.2.3.4":   {1, 2, 3},
		"1.2.3.4.5": {1, 2, 3},
	} {
		if got := parts(in); got != want {
			t.Errorf("parts(%q) = %v, want %v", in, got, want)
		}
	}
}
