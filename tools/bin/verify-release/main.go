// Command verify-release holds the newest release to what it publishes, with no artifact
// downloaded (T-127): GitHub publishes a sha256 digest on every release asset, so one API call
// for the release and one for its manifest (checksums.txt, 564 bytes) are enough to prove every
// asset the manifest names is on the release, is not empty, and carries the digest the manifest
// gives it.
//
//	go run ./tools/bin/verify-release [-repo <owner/repo>] [-manifest <name>] [-api <base>]
//
// The release is the repository's latest: /repos/<repo>/releases/latest, which is neither a
// pre-release nor a draft, as `gh release view` reads it. The manifest is that release's asset
// of the -manifest name, fetched by id with an octet-stream Accept header, and its own bytes are
// held to the digest the same call published for it, so the manifest cannot vouch for a tampered
// asset while itself being tampered. The token is GH_TOKEN or GITHUB_TOKEN when either is set
// (the nightly's step hands it github.token), else none, which is enough for a public repository.
//
// It imports nothing but the standard library, as the other gate programs do, and reads nothing
// but the two responses.
//
// Exit status: 0 every asset the manifest names is there, not empty and on its digest, 1 when
// one is missing, empty or off its digest, 2 a usage error or a request GitHub did not answer.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// defaultRepo is the repository whose newest release this holds, when -repo names none.
const defaultRepo = "donvargax/itos"

// defaultManifest is the asset that lists every other asset's digest, when -manifest names none.
const defaultManifest = "checksums.txt"

// defaultAPI is GitHub's API, where a GitHub Enterprise host's is given by -api or
// ITOS_GITHUB_API.
const defaultAPI = "https://api.github.com"

// asset is what the API says of one release asset, as much as is read.
type asset struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}

// release is what the API says of a release, as much as is read.
type release struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

// entry is one line of a manifest: the digest GitHub published for the asset of that name, and
// its name.
type entry struct {
	Name   string
	Digest string
}

func main() {
	os.Exit(run())
}

func run() int {
	repo := flag.String("repo", defaultRepo, "the owner/repo whose latest release is verified")
	manifest := flag.String("manifest", defaultManifest, "the release asset that lists every other asset's digest")
	api := flag.String("api", env("ITOS_GITHUB_API", defaultAPI), "the GitHub API's base")
	flag.Parse()
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "verify-release: unexpected argument %q\n", flag.Arg(0))
		return 2
	}
	if *repo == "" || *manifest == "" || *api == "" {
		fmt.Fprintln(os.Stderr, "verify-release: -repo, -manifest and -api each name something")
		return 2
	}

	client := &http.Client{Timeout: 60 * time.Second}
	rel, err := fetchRelease(client, strings.TrimSuffix(*api, "/"), *repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify-release: %v\n", err)
		return 2
	}

	held, ok := find(rel.Assets, *manifest)
	if !ok {
		fmt.Fprintf(os.Stderr, "verify-release: %s's %s has no %s\n", rel.TagName, *repo, *manifest)
		return 1
	}
	if held.Size == 0 {
		fmt.Fprintf(os.Stderr, "verify-release: %s's %s is empty\n", rel.TagName, *manifest)
		return 1
	}
	body, err := fetchAsset(client, strings.TrimSuffix(*api, "/"), *repo, held)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify-release: %v\n", err)
		return 2
	}

	problems := verify(rel, held, body)
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "verify-release: %s's %s\n", rel.TagName, p)
		}
		fmt.Fprintf(os.Stderr, "verify-release: %d problem(s); %s is not what the manifest says\n",
			len(problems), rel.TagName)
		return 1
	}

	fmt.Printf("verify-release: %s publishes %d assets, every one named by %s present, not empty "+
		"and on its digest (%d bytes of manifest, no artifact downloaded)\n",
		rel.TagName, len(rel.Assets), *manifest, held.Size)
	return 0
}

// verify is what the release and its manifest's bytes say of each other: the manifest's own
// digest, then every asset it names present, not empty and on the digest it gives.
func verify(rel release, manifest asset, body []byte) []string {
	var problems []string
	got := "sha256:" + digest(body)
	if manifest.Digest != got {
		problems = append(problems, fmt.Sprintf("%s is %s, and GitHub's digest for it is %s",
			manifest.Name, got, orNone(manifest.Digest)))
	}

	entries, bad := parse(manifest.Name, string(body))
	problems = append(problems, bad...)
	if len(entries) == 0 && len(bad) == 0 {
		problems = append(problems, fmt.Sprintf("%s names no asset", manifest.Name))
		return problems
	}

	byName := make(map[string]asset, len(rel.Assets))
	for _, a := range rel.Assets {
		byName[a.Name] = a
	}
	for _, e := range entries {
		a, ok := byName[e.Name]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s names %s, which the release does not publish", manifest.Name, e.Name))
		case a.Size == 0:
			problems = append(problems, fmt.Sprintf("%s is empty", e.Name))
		case a.Digest != "sha256:"+e.Digest:
			problems = append(problems, fmt.Sprintf("%s is %s, and %s gives it %s",
				e.Name, orNone(a.Digest), manifest.Name, e.Digest))
		}
	}
	return problems
}

// parse is a manifest's lines as entries, and the lines that are not one. A line is sha256sum's
// format, `<digest>  <name>` or `<digest> *<name>`, a `#` comment or nothing; anything else is
// named, since a manifest this cannot read vouches for nothing.
func parse(name, manifest string) ([]entry, []string) {
	var entries []entry
	var bad []string
	for i, line := range strings.Split(manifest, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			bad = append(bad, fmt.Sprintf("%s's line %d (%q) is not \"<digest>  <name>\"", name, i+1, line))
			continue
		}
		asset := strings.TrimPrefix(fields[1], "*")
		if !isDigest(fields[0]) || asset == "" {
			bad = append(bad, fmt.Sprintf("%s's line %d (%q) is not \"<digest>  <name>\"", name, i+1, line))
			continue
		}
		entries = append(entries, entry{Name: asset, Digest: fields[0]})
	}
	return entries, bad
}

// fetchRelease is the repository's latest release, or why GitHub did not give it.
func fetchRelease(client *http.Client, api, repo string) (release, error) {
	var rel release
	body, err := get(client, fmt.Sprintf("%s/repos/%s/releases/latest", api, repo), "")
	if err != nil {
		return rel, err
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return rel, fmt.Errorf("the API's answer for %s's latest release is not JSON: %w", repo, err)
	}
	if rel.TagName == "" {
		return rel, fmt.Errorf("the API's answer for %s's latest release names no tag", repo)
	}
	if len(rel.Assets) == 0 {
		return rel, fmt.Errorf("%s's latest release %s publishes no asset", repo, rel.TagName)
	}
	return rel, nil
}

// fetchAsset is the bytes of one asset, or why GitHub did not give them.
func fetchAsset(client *http.Client, api, repo string, a asset) ([]byte, error) {
	return get(client, fmt.Sprintf("%s/repos/%s/releases/assets/%d", api, repo, a.ID), "application/octet-stream")
}

// get is one GET, with the token when there is one, or why it failed.
func get(client *http.Client, url, accept string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot ask for %s: %w", url, err)
	}
	req.Header.Set("User-Agent", "itos-verify-release")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if token := token(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch %s: %w", url, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answers %s", url, res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", url, err)
	}
	return body, nil
}

// token is the token to ask GitHub with: GH_TOKEN, else GITHUB_TOKEN, else none, which is what a
// public repository is read with.
func token() string {
	if t := os.Getenv("GH_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("GITHUB_TOKEN")
}

// env is the environment's value of a name, or the fallback when it holds nothing.
func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// find is the asset of that name, and whether the release publishes one.
func find(assets []asset, name string) (asset, bool) {
	for _, a := range assets {
		if a.Name == name {
			return a, true
		}
	}
	return asset{}, false
}

// digest is a sha256 of bytes, as hex.
func digest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// isDigest is a lowercase hex sha256, the form GitHub and sha256sum write.
func isDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && strings.ToLower(s) == s
}

// orNone is what to say of a field GitHub left out.
func orNone(s string) string {
	if s == "" {
		return "no digest"
	}
	return s
}
