package idcounter

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v7/internal/git"
	"github.com/donvargax/itos/v7/internal/kind"
	"github.com/donvargax/itos/v7/internal/lock"
	"go.yaml.in/yaml/v3"
)

const (
	remoteRef = "refs/itos/ids"
	// fetchedCounter names the locally fetched commit without relying on FETCH_HEAD.
	fetchedCounter = "refs/itos/fetched-ids"
	localFile      = "ids.yaml"
)

// Store says where a reservation is made. Remote is the configured git
// remote name, empty when the counter must be local. CommonDir is the result
// of git rev-parse --git-common-dir, resolved to an absolute path.
type Store struct {
	Root, CommonDir, Remote string
	LocalOnly               bool
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
	held, err := lock.Hold(filepath.Join(dir, "ids"))
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

func readCounters(file string) (map[string]int, error) {
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]int{}, nil
	}
	if err != nil {
		return nil, err
	}
	var doc struct {
		Counters map[string]int `yaml:"counters"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("read id counter %s: %w", file, err)
	}
	if doc.Counters == nil {
		doc.Counters = map[string]int{}
	}
	return doc.Counters, nil
}

func writeCounters(file string, counters map[string]int) error {
	text, err := yaml.Marshal(struct {
		Counters map[string]int `yaml:"counters"`
	}{Counters: counters})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(file), ".ids-*.tmp")
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
	for attempt := 0; attempt < 16; attempt++ {
		parent, err := fetchCounter(store)
		if err != nil {
			return 0, remoteError("fetching", err)
		}
		current := 0
		if parent != "" {
			data, err := run(store.Root, nil, "show", parent+":counters/"+category)
			if err == nil {
				current, err = strconv.Atoi(strings.TrimSpace(string(data)))
				if err != nil || current < 0 {
					return 0, fmt.Errorf("remote id counter %s is not a non-negative integer", category)
				}
			} else if !missingPath(err) {
				return 0, remoteError("reading", err)
			}
		}
		first, err := firstAfter(category, max(highest, current), count)
		if err != nil {
			return 0, err
		}
		commit, err := counterCommit(store.Root, parent, category, first+count-1)
		if err != nil {
			return 0, fmt.Errorf("build id counter commit: %w", err)
		}
		_, err = run(store.Root, nil, "push", "--no-verify", store.Remote, commit+":"+remoteRef)
		if err == nil {
			return first, nil
		}
		if !nonFastForward(err) {
			return 0, remoteError("pushing", err)
		}
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

func fetchCounter(store Store) (string, error) {
	data, err := run(store.Root, nil, "ls-remote", store.Remote, remoteRef)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(data))
	if line == "" {
		return "", nil
	}
	refspec := "+" + remoteRef + ":" + fetchedCounter
	if _, err := run(store.Root, nil, "fetch", "--quiet", "--no-tags", store.Remote, refspec); err != nil {
		return "", err
	}
	fetched, err := run(store.Root, nil, "rev-parse", "--verify", "--quiet", fetchedCounter+"^{commit}")
	if err != nil {
		return "", err
	}
	// A concurrent update between ls-remote and fetch is fine; the fetched
	// commit, not the earlier advertisement, is the parent for this attempt.
	return strings.TrimSpace(string(fetched)), nil
}

// counterCommit gives otherwise identical attempts a random message nonce:
// two clones racing from the same parent must not push the same commit object
// and both believe they reserved the number.
func counterCommit(root, parent, category string, n int) (string, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("create unique id counter commit identity: %w", err)
	}
	index, err := os.CreateTemp("", "itos-id-index-")
	if err != nil {
		return "", err
	}
	indexPath := index.Name()
	if err := index.Close(); err != nil {
		_ = os.Remove(indexPath)
		return "", err
	}
	if err := os.Remove(indexPath); err != nil {
		return "", err
	}
	defer os.Remove(indexPath)
	env := []string{"GIT_INDEX_FILE=" + indexPath}
	if parent == "" {
		if _, err := run(root, env, "read-tree", "--empty"); err != nil {
			return "", err
		}
	} else {
		if _, err := run(root, env, "read-tree", parent+"^{tree}"); err != nil {
			return "", err
		}
	}
	blob, err := runWithInput(root, nil, []byte(strconv.Itoa(n)+"\n"), "hash-object", "-w", "--stdin")
	if err != nil {
		return "", err
	}
	entry := "100644," + strings.TrimSpace(string(blob)) + ",counters/" + category
	if _, err := run(root, env, "update-index", "--add", "--cacheinfo", entry); err != nil {
		return "", err
	}
	tree, err := run(root, env, "write-tree")
	if err != nil {
		return "", err
	}
	args := []string{"commit-tree", strings.TrimSpace(string(tree))}
	if parent != "" {
		args = append(args, "-p", parent)
	}
	args = append(args, "-m", fmt.Sprintf("itos: reserve %s %d (%s)", category, n, hex.EncodeToString(nonce[:])))
	identity := []string{
		"GIT_AUTHOR_NAME=itos id counter",
		"GIT_AUTHOR_EMAIL=itos-id-counter@localhost",
		"GIT_COMMITTER_NAME=itos id counter",
		"GIT_COMMITTER_EMAIL=itos-id-counter@localhost",
	}
	commit, err := runWithInput(root, identity, nil, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(commit)), nil
}

func run(root string, extraEnv []string, args ...string) ([]byte, error) {
	return runWithInput(root, extraEnv, nil, args...)
}

func runWithInput(root string, extraEnv []string, input []byte, args ...string) ([]byte, error) {
	cmd := exec.Command(git.Bin(), args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), extraEnv...)
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func missingPath(err error) bool {
	return strings.Contains(err.Error(), "does not exist in") || strings.Contains(err.Error(), "path '")
}

func nonFastForward(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "non-fast-forward") || strings.Contains(s, "fetch first") ||
		strings.Contains(s, "stale info") || strings.Contains(s, "reference already exists")
}

func remoteError(what string, err error) error {
	return kind.Wrap(kind.Missing, fmt.Errorf("cannot %s id counter on remote (nothing was written): %w", what, err))
}
