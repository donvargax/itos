package porttest

import (
	"errors"
	"testing"
)

// fixture is the fake as TrackerContract asks for it.
func fixture(stealth bool) Fixture {
	s := &Tracker{Stealth: stealth}
	return Fixture{
		Tracker: s,
		Edit:    s.Edit,
		Refuse:  func() { s.Refuse = true },
		Unready: func() { s.Unready = errors.New("the hooks are not ready") },
	}
}

// The fake keeps the tracker as the adapter keeps it on disk and in git.
func TestTrackerKeepsTheContract(t *testing.T) {
	TrackerContract(t, func(_ *testing.T, stealth bool) Fixture { return fixture(stealth) })
}

// A writer's place given back twice is an error the second time.
func TestHeldReleasedTwice(t *testing.T) {
	held, err := (&Tracker{Stealth: true}).Hold(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := held.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := held.Release(t.Context()); err == nil {
		t.Error("a place given back twice")
	}
}
