package ask

import (
	"context"
	"errors"
	"testing"

	"github.com/donvargax/itos/v8/internal/store/tracker/port"
	trackerfake "github.com/donvargax/itos/v8/internal/store/tracker/port/porttest"
)

// unloadable is a tracker whose Load fails.
type unloadable struct{ trackerfake.Tracker }

func (*unloadable) Load(context.Context, []string) ([]port.Doc, error) {
	return nil, errors.New("cannot load")
}

// Load is the file's questions and its text when it is there, none and ""
// when it is not; a file Parse refuses is its error, the text read with it;
// a tracker that cannot load is its error.
func TestLoad(t *testing.T) {
	var tr trackerfake.Tracker
	f, text, there, err := Load(t.Context(), &tr, "asks.yaml")
	if err != nil || there || text != "" || len(f.Questions) != 0 {
		t.Errorf("no file: %+v %q %v %v", f, text, there, err)
	}

	const written = "questions:\n  - { id: q-1, item: p1-a, question: \"Why?\" }\n"
	tr.Edit("tasks/asks.yaml", written)
	f, text, there, err = Load(t.Context(), &tr, "tasks/asks.yaml")
	if err != nil || !there || text != written || len(f.Questions) != 1 || f.Questions[0].ID != "q-1" || f.Questions[0].Item != "p1-a" {
		t.Errorf("a file: %+v %q %v %v", f, text, there, err)
	}

	const twice = "questions:\n  - { id: q-1, question: \"Why?\" }\n  - { id: q-1, question: \"How?\" }\n"
	tr.Edit("tasks/asks.yaml", twice)
	_, text, there, err = Load(t.Context(), &tr, "tasks/asks.yaml")
	if err == nil || !there || text != twice {
		t.Errorf("a file Parse refuses: %q %v %v", text, there, err)
	}

	f, text, there, err = Load(t.Context(), &unloadable{}, "tasks/asks.yaml")
	if err == nil || err.Error() != "cannot load" || there || text != "" || len(f.Questions) != 0 {
		t.Errorf("a tracker that cannot load: %+v %q %v %v", f, text, there, err)
	}
}
