package evidence

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Folder is the store's folder in itos's folder of the git common dir, and
// the names in it: a record is the folder <key> holding Manifest, Result and
// the Complete marker, written last; its lock is <key>.lock; a record is
// written in a folder named .tmp-<key>-<unique>, which no reader reads, and
// renamed into place.
const (
	Folder     = "evidence"
	manifest   = "manifest.json"
	resultFile = "result.json"
	complete   = "complete"
)

// Stored is the manifest file: the key and everything it binds.
type Stored struct {
	Key string `json:"key"`
	Manifest
}

// Result is the result file: how the check ended, the tail of what it
// printed, when, and where it ran.
type Result struct {
	Exit       int               `json:"exit"`
	Output     string            `json:"output"`
	Truncated  bool              `json:"truncated,omitempty"`
	Started    string            `json:"started"`
	Ended      string            `json:"ended"`
	Provenance map[string]string `json:"provenance"`
}

// record is a record read back whole.
type record struct {
	stored Stored
	result Result
}

// dir is the store's folder.
func (e *Engine) dir(ctx context.Context) (string, error) { return e.Store.Path(ctx, Folder) }

// lookup reads the record of a key, and whether there is one to trust. A
// record is trusted only whole: its marker there, its manifest a manifest of
// this schema that hashes to the key its folder is named by, its result a
// result. Anything less is no record, as a folder that does not exist is
// none. Lookup takes no lock.
func (e *Engine) lookup(ctx context.Context, key string) (record, bool) {
	root, err := e.dir(ctx)
	if err != nil {
		return record{}, false
	}
	at := filepath.Join(root, key)
	if _, err := os.Stat(filepath.Join(at, complete)); err != nil {
		return record{}, false
	}
	var r record
	if !readJSON(filepath.Join(at, manifest), &r.stored) || !readJSON(filepath.Join(at, resultFile), &r.result) {
		return record{}, false
	}
	if r.stored.Schema != Schema || r.stored.Key != key || r.stored.Manifest.Key() != key {
		return record{}, false
	}
	return r, true
}

// readJSON reads a file into v, and whether it could.
func readJSON(name string, v any) bool {
	text, err := os.ReadFile(name)
	return err == nil && json.Unmarshal(text, v) == nil
}

// mkdirTemp is os.MkdirTemp and rename os.Rename, which a test swaps for a
// folder that refuses a file and a rename that fails.
var (
	mkdirTemp = os.MkdirTemp
	rename    = os.Rename
)

// The reasons a record found broken is renamed aside under.
const (
	quarantineMarkerless = "markerless"
	quarantineCorrupt    = "corrupt"
)

// write makes the record of a key: in a temporary folder of the store, its
// files synced and the marker last, then renamed to the key's folder, which
// is never overwritten. What is at the key is judged before the rename, since
// an OS may let a rename replace a file, and again when the rename fails,
// since another writer may have got there between: a whole record is kept as
// it is and this one is dropped (placed false, kept the record that is
// there), and anything else is quarantined, after which the rename is tried
// once more. What cannot be judged or moved aside is an error, and no record
// is made.
func (e *Engine) write(ctx context.Context, key string, b bound, res Result) (placed bool, kept record, err error) {
	root, err := e.dir(ctx)
	if err != nil {
		return false, record{}, err
	}
	if err = os.MkdirAll(root, 0o700); err != nil {
		return false, record{}, err
	}
	tmp, err := mkdirTemp(root, ".tmp-"+key+"-")
	if err != nil {
		return false, record{}, err
	}
	defer func() {
		if !placed {
			_ = os.RemoveAll(tmp)
		}
	}()
	// Marshalling these types cannot fail.
	stored, _ := json.MarshalIndent(Stored{Key: key, Manifest: b.Manifest}, "", "  ")
	result, _ := json.MarshalIndent(res, "", "  ")
	for _, f := range []struct{ name, text string }{{manifest, string(stored) + "\n"}, {resultFile, string(result) + "\n"}, {complete, "complete\n"}} {
		if err = writeSynced(filepath.Join(tmp, f.name), []byte(f.text)); err != nil {
			return false, record{}, err
		}
	}
	final := filepath.Join(root, key)
	if _, taken := os.Lstat(final); taken == nil {
		if kept, whole, err := e.settle(ctx, root, key); whole || err != nil {
			return false, kept, err
		}
	}
	if rename(tmp, final) == nil {
		return true, record{}, nil
	}
	if kept, whole, err := e.settle(ctx, root, key); whole || err != nil {
		return false, kept, err
	}
	if err = rename(tmp, final); err != nil {
		return false, record{}, err
	}
	return true, record{}, nil
}

// settle judges what is at the key's place: a whole record is returned as it
// is, and anything else is quarantined, which is an error when it cannot be.
func (e *Engine) settle(ctx context.Context, root, key string) (kept record, whole bool, err error) {
	if rec, ok := e.lookup(ctx, key); ok {
		return rec, true, nil
	}
	return record{}, false, e.quarantine(root, key)
}

// quarantine renames what is at the key's place aside, to
// <key>.<reason>-<UTC timestamp>: markerless for a folder without its
// completion marker, corrupt for anything else that is not a whole record.
func (e *Engine) quarantine(root, key string) error {
	at := filepath.Join(root, key)
	info, err := os.Lstat(at)
	if err != nil {
		return err
	}
	reason := quarantineCorrupt
	if info.IsDir() {
		if _, err := os.Stat(filepath.Join(at, complete)); err != nil {
			reason = quarantineMarkerless
		}
	}
	return rename(at, filepath.Join(root, key+"."+reason+"-"+e.now().Format("20060102T150405.000000")+"Z"))
}

// writeSynced writes a file readable by its owner alone and syncs it.
func writeSynced(name string, text []byte) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, written := f.Write(text)
	return errors.Join(written, f.Sync(), f.Close())
}
