package idcounter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/lock"
	"github.com/donvargax/itos/v8/internal/value"
	vcs "github.com/donvargax/itos/v8/internal/vcs/port"
	"go.yaml.in/yaml/v3"
)

const (
	remoteRef = "refs/itos/ids"
	// fetchedCounter names the locally fetched commit without relying on FETCH_HEAD.
	fetchedCounter = "refs/itos/fetched-ids"
	localFile      = "ids.yaml"
)

// Store says where a reservation is made. Root is the repository's folder,
// "" when none was located, and VCS the repository, which reserves on the
// remote. Remote is the configured git remote name, empty when the counter
// must be local. CommonDir is the git common dir, resolved to an absolute
// path. OS is the OS itos runs on, as GOOS names it, which the local
// counter's lock waits by.
type Store struct {
	Root, CommonDir, Remote, OS string
	VCS                         vcs.VCS
	LocalOnly                   bool
}

// Mint reserves one number greater than both repositoryHighest and the
// counter already stored. The reservation happens before returning and is
// never rolled back.
func Mint(store Store, category string, repositoryHighest int) (int, error) {
	return Claim(store, category, repositoryHighest, 1)
}

// Claim reserves count numbers in a row, the first greater than both
// repositoryHighest and the counter already stored, and gives the first. The
// counter is raised past the last in one reservation (one write, or one
// counter commit pushed), so no other claim lands inside the run. A count
// below 1 is refused before anything is read, and a run whose last number
// would pass the largest int before anything is written.
func Claim(store Store, category string, repositoryHighest, count int) (int, error) {
	if !validCategory(category) {
		return 0, fmt.Errorf("invalid id counter category %q", category)
	}
	if count < 1 {
		return 0, fmt.Errorf("cannot claim %d id numbers", count)
	}
	if store.LocalOnly || store.Remote == "" {
		return mintLocal(store, category, repositoryHighest, count)
	}
	return mintRemote(store, category, repositoryHighest, count)
}

func validCategory(s string) bool {
	if s == "" || strings.ContainsAny(s, "/\\\x00") || s == "." || s == ".." {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

func mintLocal(store Store, category string, highest, count int) (int, error) {
	if store.CommonDir == "" {
		return 0, kind.Wrap(kind.Missing, errors.New("cannot locate the git common directory for the id counter"))
	}
	dir := filepath.Join(store.CommonDir, "itos")
	file := filepath.Join(dir, localFile)
	held, err := lock.Hold(filepath.Join(dir, "ids"), store.OS)
	if err != nil {
		return 0, err
	}
	defer held.Release()
	counters, err := readCounters(file)
	if err != nil {
		return 0, err
	}
	first, err := firstAfter(category, max(highest, counters[category]), count)
	if err != nil {
		return 0, err
	}
	counters[category] = first + count - 1
	if err := writeCounters(file, counters); err != nil {
		return 0, err
	}
	return first, nil
}

// maxCounter is the largest counter read: the largest whole number
// value.Parse's numbers hold exactly, or the largest int where that is smaller.
var maxCounter = min(float64(1<<53-1), float64(math.MaxInt))

// readCounters is the counters in the file, read through value.Parse as every
// other YAML file itos reads is. A file Parse refuses, a top or counters that
// is no mapping, or a counter that is no whole number from 0 to maxCounter is
// refused: a counter read lower than it was written would mint a number
// already handed out.
func readCounters(file string) (map[string]int, error) {
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]int{}, nil
	}
	if err != nil {
		return nil, err
	}
	counters, err := parseCounters(string(data))
	if err != nil {
		return nil, kind.Wrap(kind.Usage, fmt.Errorf("read id counter %s: %w", file, err))
	}
	return counters, nil
}

func parseCounters(text string) (map[string]int, error) {
	counters := map[string]int{}
	parsed, err := value.Parse(text)
	if err != nil || parsed == nil {
		return counters, err
	}
	top, ok := parsed.(*value.Map)
	if !ok {
		return nil, fmt.Errorf("the top is %s, not a mapping", value.TypeOf(parsed))
	}
	section := top.At("counters")
	if section == nil || section == value.Undefined {
		return counters, nil
	}
	m, ok := section.(*value.Map)
	if !ok {
		return nil, fmt.Errorf("counters is %s, not a mapping", value.TypeOf(section))
	}
	for _, category := range m.Keys() {
		n, ok := m.At(category).(float64)
		if !ok || n != math.Trunc(n) || n < 0 || n > maxCounter {
			return nil, fmt.Errorf("the counter %s is %s, not a whole number from 0 to %s", category, value.JSON(m.At(category)), value.Number(maxCounter))
		}
		counters[category] = int(n)
	}
	return counters, nil
}

func writeCounters(file string, counters map[string]int) error {
	return writeCountersThrough(file, counters, func(dir string) (tempFile, error) {
		return os.CreateTemp(dir, ".ids-*.tmp")
	})
}

// tempFile is the file writeCounters writes the counters to before it moves
// it over the counter: an *os.File, or in the tests one that fails as a full
// or failing disk would, which a real file does not do on demand.
type tempFile interface {
	Name() string
	Chmod(mode os.FileMode) error
	Write(b []byte) (int, error)
	Sync() error
	Close() error
}

// writeCountersThrough is writeCounters with its temporary file made by
// create, in the counter's folder.
func writeCountersThrough(file string, counters map[string]int, create func(dir string) (tempFile, error)) error {
	// A map of strings to ints always marshals: yaml.Marshal fails only on a
	// value it cannot encode, and every string and int encodes.
	text, _ := yaml.Marshal(struct {
		Counters map[string]int `yaml:"counters"`
	}{Counters: counters})
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	f, err := create(filepath.Dir(file))
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(text); err != nil {
		_ = f.Close()
		return err
	}
	if err := errors.Join(f.Sync(), f.Close()); err != nil {
		return err
	}
	if err := replaceFile(tmp, file); err != nil {
		return err
	}
	return nil
}

func mintRemote(store Store, category string, highest, count int) (int, error) {
	if store.Root == "" {
		return 0, kind.Wrap(kind.Missing, errors.New("cannot locate the repository for the id counter"))
	}
	ctx := context.Background()
	parent, err := fetchCounter(ctx, store)
	if err != nil {
		return 0, remoteError("fetch the id counter from the remote", err)
	}
	for attempt := 0; attempt < 16; attempt++ {
		current := 0
		if parent != "" {
			data, ok, err := store.VCS.File(ctx, parent, "counters/"+category)
			switch {
			case err != nil:
				return 0, remoteError("read the id counter on the remote", err)
			case ok:
				current, err = strconv.Atoi(strings.TrimSpace(data))
				if err != nil || current < 0 {
					return 0, fmt.Errorf("remote id counter %s is not a non-negative integer", category)
				}
			}
		}
		first, err := firstAfter(category, max(highest, current), count)
		if err != nil {
			return 0, err
		}
		commit, err := counterCommit(ctx, store.VCS, parent, category, first+count-1)
		if err != nil {
			return 0, fmt.Errorf("build id counter commit: %w", err)
		}
		pushed := store.VCS.Push(ctx, store.Remote, []string{commit + ":" + remoteRef}, vcs.Pushing{NoVerify: true}, vcs.Talk{})
		if pushed == nil {
			return first, nil
		}
		// Whatever git said, a push is a lost race when the remote's counter
		// moved past the commit it was built on; one that left the counter
		// where it was is the remote's refusal.
		moved, err := fetchCounter(ctx, store)
		if err != nil {
			return 0, remoteError("fetch the id counter from the remote", err)
		}
		if moved == parent {
			return 0, remoteError("push the id counter to the remote", pushed)
		}
		parent = moved
	}
	return 0, kind.Wrap(kind.Temporary, errors.New("the remote id counter kept advancing; run the command again"))
}

// firstAfter is the first of count numbers past floor, refused when the
// last would pass the largest int.
func firstAfter(category string, floor, count int) (int, error) {
	if floor > math.MaxInt-count {
		return 0, fmt.Errorf("id counter %s cannot claim %d past %d: the largest number is %d", category, count, floor, math.MaxInt)
	}
	return floor + 1, nil
}

// fetchCounter is the remote's counter commit, fetched, "" when the remote
// has none yet. A concurrent update between asking and fetching is fine:
// the fetched commit, not the earlier answer, is the parent for this
// attempt.
func fetchCounter(ctx context.Context, store Store) (string, error) {
	refs, err := store.VCS.RemoteRefs(ctx, store.Remote, []string{remoteRef}, vcs.Talk{})
	if err != nil || len(refs) == 0 {
		return "", err
	}
	return store.VCS.Fetch(ctx, store.Remote, "+"+remoteRef+":"+fetchedCounter, vcs.Talk{})
}

// counterCommit gives otherwise identical attempts a random message nonce:
// two clones racing from the same parent must not push the same commit object
// and both believe they reserved the number.
func counterCommit(ctx context.Context, v vcs.VCS, parent, category string, n int) (string, error) {
	// crypto/rand.Read never returns an error (since Go 1.24): where the
	// system cannot give random bytes, it stops the program.
	var nonce [16]byte
	_, _ = rand.Read(nonce[:])
	return v.CommitFile(ctx, parent, "counters/"+category, strconv.Itoa(n)+"\n",
		fmt.Sprintf("itos: reserve %s %d (%s)", category, n, hex.EncodeToString(nonce[:])),
		"itos id counter", "itos-id-counter@localhost")
}

func remoteError(action string, err error) error {
	return kind.Wrap(kind.Missing, fmt.Errorf("cannot %s (nothing was written): %w", action, err))
}
