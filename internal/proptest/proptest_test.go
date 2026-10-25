package proptest

import (
	"flag"
	"os"
	"strconv"
	"testing"

	"pgregory.net/rapid"
)

func rapidFlags(seed, checks string) *flag.FlagSet {
	fs := flag.NewFlagSet("rapid", flag.ContinueOnError)
	fs.String("rapid.seed", seed, "")
	fs.String("rapid.checks", checks, "")
	return fs
}

func none(string) (string, bool) { return "", false }

// A run that names no seed draws the fixed one; a run that names one keeps
// it, whatever it is.
func TestPinSeedFixesTheSeedOnlyWhenTheRunNamedNone(t *testing.T) {
	fs := rapidFlags("0", "100")
	pinSeed(fs)
	if got := fs.Lookup("rapid.seed").Value.String(); got != strconv.FormatUint(FixedSeed, 10) {
		t.Errorf("seed = %s, want %d", got, FixedSeed)
	}
	named := rapidFlags("77", "100")
	pinSeed(named)
	if got := named.Lookup("rapid.seed").Value.String(); got != "77" {
		t.Errorf("a named seed became %s", got)
	}
	pinSeed(flag.NewFlagSet("no rapid", flag.ContinueOnError))
}

// Checks is the property's own count for the length of one property: the
// flag's value inside, its old one after.
func TestChecksSetsTheCountForOnePropertyAndRestoresIt(t *testing.T) {
	fs := rapidFlags("0", "100")
	t.Run("property", func(t *testing.T) {
		Checks(7)(t, fs, none)
		if got := fs.Lookup("rapid.checks").Value.String(); got != "7" {
			t.Errorf("checks = %s inside, want 7", got)
		}
	})
	if got := fs.Lookup("rapid.checks").Value.String(); got != "100" {
		t.Errorf("checks = %s after, want 100", got)
	}
}

// A count the run names (RAPID_CHECKS, as the nightly does, or -rapid.checks)
// is not Checks's to change.
func TestChecksYieldsToACountTheRunNamed(t *testing.T) {
	byEnv := rapidFlags("0", "100")
	Checks(7)(t, byEnv, func(name string) (string, bool) { return "300", name == "RAPID_CHECKS" })
	if got := byEnv.Lookup("rapid.checks").Value.String(); got != "100" {
		t.Errorf("checks = %s with RAPID_CHECKS named, want it left", got)
	}
	byFlag := rapidFlags("0", "100")
	if err := byFlag.Set("rapid.checks", "50"); err != nil {
		t.Fatal(err)
	}
	Checks(7)(t, byFlag, none)
	if got := byFlag.Lookup("rapid.checks").Value.String(); got != "50" {
		t.Errorf("checks = %s with -rapid.checks named, want 50", got)
	}
	Checks(7)(t, flag.NewFlagSet("no rapid", flag.ContinueOnError), none)
}

// Check pins the process's seed, so two runs of a property draw alike, and
// its options reach rapid.
func TestCheckDrawsTheSameSequencesEveryRun(t *testing.T) {
	draws := func() (drawn []int, checks string) {
		Check(t, func(rt *rapid.T) {
			drawn = append(drawn, rapid.Int().Draw(rt, "n"))
			checks = flag.Lookup("rapid.checks").Value.String()
		}, Checks(4))
		return drawn, checks
	}
	first, checks := draws()
	second, _ := draws()
	if len(first) == 0 || len(first) != len(second) {
		t.Fatalf("runs drew %d and %d values", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("draw %d was %d, then %d: the seed is not pinned", i, first[i], second[i])
		}
	}
	if seed := flag.Lookup("rapid.seed").Value.String(); seed == "0" {
		t.Error("rapid's seed is still 0 after Check")
	}
	if checks != "4" && !namedChecks() {
		t.Errorf("checks = %s inside Check(..., Checks(4))", checks)
	}
}

// namedChecks is whether this run named a count of checks, which Checks
// then leaves alone.
func namedChecks() bool {
	found := false
	flag.Visit(func(f *flag.Flag) { found = found || f.Name == "rapid.checks" })
	_, env := os.LookupEnv("RAPID_CHECKS")
	return found || env
}
