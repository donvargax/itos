package cli

// itos status (slice 67, features/status.feature): where the work stands,
// for the person a session works for, from what already records it, in
// place of a handoff file rewritten by hand. The remote branch's head is
// asked of the remote (git ls-remote), else read as last fetched; its CI run
// is looked at once through ci.watch's provider, never waited for, a run
// still going reported as going; then (slice 70) the newest release, the
// highest tag vX.Y.Z by the release cut's rule, asked of the remote as the
// head is, else as last fetched, and the commits since it the next release would carry, the feat,
// fix and breaking ones, read from the commits as fetched here; after the
// head's CI run (slice 72), the last nightly's, looked at once through the
// same provider when it names a nightly, and left out when it names none;
// before it, while the head's run is going or has not passed (slice 73), the
// commit main last proved, as ci.range's provider names it, read once; then the
// person's items in progress, the next ones they can start in the queue's
// order (a handful), and the open questions of itos ask. It reads, never writes. What cannot be reached is
// one line naming it, and the rest still prints, exit 0; a status that
// cannot be computed at all (no config, no registry, one with problems) is
// exit 2 or 1, as itos work's. itos go prints it after the guides
// (guide.go), and there skips one that cannot be computed with a line.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/git"
	"github.com/donvargax/itos/v5/internal/out"
	"github.com/donvargax/itos/v5/internal/providers"
	"github.com/donvargax/itos/v5/internal/release"
	"github.com/donvargax/itos/v5/internal/value"
	"github.com/donvargax/itos/v5/internal/work"
)

// statusHeading is the status's first line, which itos go's reader looks for.
const statusHeading = "# Where things stand"

// questionWidth is how much of a question the status prints, in characters,
// the rest left to itos ask show.
const questionWidth = 80

// clipped is text cut to n characters, an ellipsis ending it when cut.
func clipped(text string, n int) string {
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return strings.TrimRight(string(runes[:n-1]), " ") + "…"
}

// handful is how many of the next items the status lists; itos work lists
// them all.
const handful = 5

// lsRemoteTimeout is how long the status waits for the remote to say its
// head, before reading it as last fetched.
var lsRemoteTimeout = 20 * time.Second

// standing is where things stand, as --json gives it.
type standing struct {
	Person any         `json:"person"`
	Every  bool        `json:"every_item,omitempty"`
	Head   *remoteHead `json:"head"`
	CI     *ciStanding `json:"ci"`
	// LastGreen is the commit main last proved; nil when the head's run
	// passed or was not read, ci.range's provider is none, it names no
	// commit, or it cannot be read.
	LastGreen *lastGreen `json:"last_green"`
	// Nightly is the last nightly's run; nil when ci.watch names no
	// nightly, it has no run yet, or it cannot be read.
	Nightly *providers.Run `json:"nightly"`
	Release *newest        `json:"release"`
	// Unreleased are the headers of the commits since the release the next
	// one would carry, oldest first; nil when there is no release or they
	// cannot be listed.
	Unreleased []string     `json:"unreleased"`
	Doing      []any        `json:"doing"`
	Next       []any        `json:"next"`
	More       int          `json:"more"`
	Questions  []askEntry   `json:"questions"`
	Unread     []string     `json:"unread"`
	unowned    map[any]bool // the next items nobody owns
	start      []string     // the lines of the head and CI, in order
	released   []string     // the lines of the release section, in order
}

// newest is the newest release: its tag, the commit it names, and whether
// it was read as last fetched, the remote not answering.
type newest struct {
	Tag         string `json:"tag"`
	Commit      string `json:"commit"`
	LastFetched bool   `json:"last_fetched"`
}

// remoteHead is the remote branch's head: its commit and header ("" when
// the commit is not fetched here), and whether it was read as last fetched,
// the remote not answering.
type remoteHead struct {
	Remote      string `json:"remote"`
	Branch      string `json:"branch"`
	Commit      string `json:"commit"`
	Header      string `json:"header"`
	LastFetched bool   `json:"last_fetched"`
}

// lastGreen is the commit ci.range's provider names as main's last green
// one, and its header ("" when the commit is not fetched here).
type lastGreen struct {
	Commit string `json:"commit"`
	Header string `json:"header"`
}

// ciStanding is the head's CI run as one look saw it: its result, success,
// failure (or another conclusion), going while it runs, or none when there
// is no run of the commit yet.
type ciStanding struct {
	Result string         `json:"result"`
	Run    *providers.Run `json:"run,omitempty"`
}

// statusCommand is `status [--as <handle>]`.
func statusCommand(args []string, o Out) (int, error) {
	pos, flags, _, err := subArgs("status", "", args, []string{"--as"}, nil)
	if err != nil {
		return 0, err
	}
	if len(pos) > 0 {
		return 0, usage("status takes no arguments: %s", strings.Join(pos, " "))
	}
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	st, found, code, err := statusOf(cfg, flags["--as"], o)
	if err != nil || code != 0 {
		if len(found) > 0 {
			return refuseWork(found, code, o)
		}
		return code, err
	}
	if o.JSON {
		return 0, out.Emit(o.Stdout, st.fields()...)
	}
	st.print(o.Stdout)
	return 0, nil
}

// statusOf is where things stand for the person --as names (as, "" when not
// given), else who the people make it, as itos work proposes (Whoami: the
// one person, nobody, or the identity provider's among several). A registry
// that is not there or not sound is its problems, exit 1; --as not among the
// people is exit 3, said on stderr.
func statusOf(cfg *config.Loaded, as string, o Out) (*standing, []out.Problem, int, error) {
	found, err := work.ProblemsAt(cfg, cfg.Work.Registry, false)
	if err != nil {
		return nil, nil, 0, err
	}
	if len(found) > 0 {
		return nil, found, ExitPolicy, nil
	}
	registry, err := work.Load(cfg, cfg.Work.Registry)
	if err != nil {
		return nil, nil, 0, err
	}
	var proposal work.Proposal
	if cfg.Stealth && as == "" {
		proposal = work.ProposeEvery(registry)
	} else {
		listedIn := cfg.Work.People.File
		who := work.Whoami(registry, listedIn, as, providers.IdentityProvider(cfg))
		switch {
		case who.Problem != "":
			fmt.Fprintln(o.Stderr, who.Problem)
			if as != "" {
				return nil, nil, ExitMissing, nil
			}
		case who.Handle != "" && !who.Listed:
			fmt.Fprintf(o.Stderr, "%s is not in %s: nothing is theirs yet\n", who.Handle, listedIn)
		}
		proposal = work.Propose(registry, who.Handle)
	}
	st := &standing{Every: proposal.Every, Doing: []any{}, Next: []any{}, Questions: []askEntry{}, Unread: []string{},
		unowned: map[any]bool{}}
	if proposal.Person != "" {
		st.Person = proposal.Person
	}
	st.readHead(cfg, o)
	for _, item := range proposal.Doing {
		st.Doing = append(st.Doing, item)
	}
	for _, item := range proposal.Unowned {
		st.unowned[item] = true
	}
	next := work.Startable(registry, proposal)
	for _, item := range next[:min(handful, len(next))] {
		st.Next = append(st.Next, item)
	}
	st.More = len(next) - len(st.Next)
	asks, _, _, err := loadAsks(cfg)
	if err != nil {
		st.unread(fmt.Sprintf("The questions cannot be read: %s", err))
	}
	for _, q := range asks.Questions {
		if q.Answer == "" {
			st.Questions = append(st.Questions, entryOf(q))
		}
	}
	return st, nil, 0, nil
}

// unread records what could not be reached, as a line of its own.
func (st *standing) unread(line string) {
	st.Unread = append(st.Unread, line)
	st.start = append(st.start, line)
}

// readHead reads the remote branch's head and its CI run, then the last
// nightly's run on that branch: the branch's upstream (origin and the
// branch's own name when it has none set), or origin's HEAD from a detached
// HEAD.
func (st *standing) readHead(cfg *config.Loaded, o Out) {
	remote, ref := "origin", "HEAD"
	if branch := git.Branch(); branch != "" {
		remote, ref, _ = git.Upstream(branch)
	}
	branch := strings.TrimPrefix(ref, "refs/heads/")
	st.readBranch(cfg, remote, ref, branch, o)
	st.readNightly(cfg, remote, branch, o)
}

// readBranch reads the remote branch's head, its CI run and the newest
// release.
func (st *standing) readBranch(cfg *config.Loaded, remote, ref, branch string, o Out) {
	if _, err := git.Output("remote", "get-url", remote); err != nil {
		st.unread(fmt.Sprintf("No remote %s, so neither its head nor its CI run is read.", remote))
		return
	}
	head := &remoteHead{Remote: remote, Branch: branch}
	sha, reached := lsRemote(remote, ref)
	switch {
	case reached && sha == "":
		st.unread(fmt.Sprintf("%s has no branch %s yet, so there is no CI run to read.", remote, branch))
		return
	case !reached:
		fetched, err := git.Output("rev-parse", "--verify", "--quiet", "refs/remotes/"+remote+"/"+branch+"^{commit}")
		if err != nil || strings.TrimSpace(fetched) == "" {
			st.unread(fmt.Sprintf("%s cannot be reached and %s/%s was never fetched, so neither its head nor its CI run is read.",
				remote, remote, branch))
			return
		}
		sha = strings.TrimSpace(fetched)
		head.LastFetched = true
		st.unread(fmt.Sprintf("%s cannot be reached, so its %s is as last fetched.", remote, branch))
	}
	head.Commit = sha
	if git.HasCommit(sha) {
		header, _ := git.Output("log", "-1", "--format=%s", sha)
		head.Header = strings.TrimSpace(header)
	}
	st.Head = head
	line := fmt.Sprintf("%s/%s: %s", remote, branch, short(sha))
	if head.Header != "" {
		line += " " + head.Header
	} else {
		line += " (not fetched here)"
	}
	st.start = append(st.start, line)
	st.readCI(cfg, remote, sha, o)
	if st.CI != nil && st.CI.Result != "success" {
		st.readLastGreen(cfg, remote, branch, o)
	}
	st.readRelease(remote, branch, head)
}

// lsRemote is the commit the remote's ref names, "" when it has no such
// ref; reached is false when the remote did not answer in time, or at all.
// It never prompts for credentials.
func lsRemote(remote, ref string) (sha string, reached bool) {
	refs, reached := askRemote(remote, ref)
	return refs[ref], reached
}

// askRemote is git ls-remote's refs, each name the commit or object it
// names; reached is false when the remote did not answer in time, or at all.
// It never prompts for credentials.
func askRemote(args ...string) (refs map[string]string, reached bool) {
	ctx, cancel := context.WithTimeout(context.Background(), lsRemoteTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, git.Bin(), append([]string{"ls-remote"}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, false
	}
	refs = map[string]string{}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 {
			refs[fields[1]] = fields[0]
		}
	}
	return refs, true
}

// releaseTags are the tags the remote has, each the commit it names, an
// annotated tag peeled to its commit; reached is false when the remote did
// not answer. Asked of the remote alone when it answered for the head, so
// a remote that did not is not waited for twice.
func releaseTags(remote string, ask bool) (tags map[string]string, reached bool) {
	if !ask {
		return nil, false
	}
	refs, reached := askRemote("--tags", remote)
	if !reached {
		return nil, false
	}
	return peeled(refs, func(ref string) (string, bool) { return strings.CutPrefix(ref, "refs/tags/") }), true
}

// fetchedTags are the tags this clone has, as last fetched, each the commit
// it names.
func fetchedTags() map[string]string {
	out, err := git.Output("for-each-ref", "--format=%(objectname) %(refname) %(*objectname)", "refs/tags/")
	if err != nil {
		return map[string]string{}
	}
	refs := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			refs[fields[1]] = fields[0]
		}
		if len(fields) == 3 {
			refs[fields[1]+"^{}"] = fields[2]
		}
	}
	return peeled(refs, func(ref string) (string, bool) { return strings.CutPrefix(ref, "refs/tags/") })
}

// peeled are the refs' tag names, each the commit it names: its peeled ^{}
// entry where it has one, an annotated tag's commit, else its own object.
func peeled(refs map[string]string, name func(string) (string, bool)) map[string]string {
	tags := map[string]string{}
	for ref, sha := range refs {
		if strings.HasSuffix(ref, "^{}") {
			continue
		}
		if tag, ok := name(ref); ok {
			tags[tag] = sha
			if commit, ok := refs[ref+"^{}"]; ok {
				tags[tag] = commit
			}
		}
	}
	return tags
}

// readRelease reads the newest release, the highest tag vX.Y.Z by the
// release cut's rule (release.Newest, bug 20; a prerelease is never one),
// asked of the remote when it answered for the head, else as last fetched,
// and the commits since it the next release would carry: the feat, fix and
// breaking ones (release.Releasable, as the release cut counts them) from
// the release to the remote's head, read from the commits as fetched here.
// Where the head is not fetched here, they are listed to the remote branch
// as last fetched, and a line says the list may be behind.
func (st *standing) readRelease(remote, branch string, head *remoteHead) {
	tags, reached := releaseTags(remote, !head.LastFetched)
	if !reached {
		if !head.LastFetched {
			st.releaseUnread(fmt.Sprintf("%s's tags cannot be read, so its newest release is as last fetched.", remote))
		}
		tags = fetchedTags()
	}
	names := make([]string, 0, len(tags))
	for name := range tags {
		names = append(names, name)
	}
	tag := release.Newest(names)
	if tag == "" {
		st.released = append(st.released, "Newest release: no release yet.")
		return
	}
	st.Release = &newest{Tag: tag, Commit: tags[tag], LastFetched: !reached}
	line := "Newest release: " + tag
	if !reached {
		line += " (as last fetched)"
	}
	st.released = append(st.released, line)
	tip, asFetched := head.Commit, ""
	if !git.HasCommit(tip) {
		asFetched = ", as fetched here"
		fetched, err := git.Output("rev-parse", "--verify", "--quiet", "refs/remotes/"+remote+"/"+branch+"^{commit}")
		tip = strings.TrimSpace(fetched)
		if err != nil || tip == "" {
			st.releaseUnread(fmt.Sprintf("%s/%s is not fetched here, so the commits since %s are not listed.", remote, branch, tag))
			return
		}
		st.releaseUnread(fmt.Sprintf("%s/%s is not fetched here up to its head, so the commits since %s may be behind.",
			remote, branch, tag))
	}
	if !git.HasCommit(st.Release.Commit) {
		st.releaseUnread(fmt.Sprintf("%s's commit is not fetched here, so the commits since it are not listed.", tag))
		return
	}
	log, err := git.Output("log", "--reverse", "--format=%B%x1e", st.Release.Commit+".."+tip)
	if err != nil {
		st.releaseUnread(fmt.Sprintf("The commits since %s cannot be read.", tag))
		return
	}
	st.Unreleased = []string{}
	for _, record := range strings.Split(log, "\x1e") {
		message := strings.TrimSpace(record)
		if message != "" && release.Releasable(message) {
			header, _, _ := strings.Cut(message, "\n")
			st.Unreleased = append(st.Unreleased, header)
		}
	}
	if len(st.Unreleased) == 0 {
		st.released = append(st.released, fmt.Sprintf("Nothing unreleased since %s%s.", tag, asFetched))
		return
	}
	st.released = append(st.released, fmt.Sprintf("Unreleased since %s%s, oldest first:", tag, asFetched))
	for _, header := range st.Unreleased {
		st.released = append(st.released, "  "+header)
	}
}

// releaseUnread records what of the release could not be read, as a line
// of the release section.
func (st *standing) releaseUnread(line string) {
	st.Unread = append(st.Unread, line)
	st.released = append(st.released, line)
}

// readCI looks once at the commit's CI run through ci.watch's provider.
func (st *standing) readCI(cfg *config.Loaded, remote, sha string, o Out) {
	look, ok, err := watcher(cfg, remote, o)
	switch {
	case err != nil:
		st.unread(fmt.Sprintf("CI cannot be read: %s", err))
		return
	case !ok:
		st.unread("CI is not read: ci.watch.provider is none.")
		return
	}
	run, found, err := look(sha)
	switch {
	case err != nil:
		st.unread(fmt.Sprintf("CI cannot be read: %s", err))
	case !found:
		st.CI = &ciStanding{Result: "none"}
		st.start = append(st.start, fmt.Sprintf("CI: no run of %s yet", short(sha)))
	default:
		if run.Jobs == nil {
			run.Jobs = []providers.Job{}
		}
		result, line := runLine("CI", run)
		st.CI = &ciStanding{Result: result, Run: &run}
		st.start = append(st.start, line)
	}
}

// readLastGreen looks once at the commit main last proved, through
// ci.range's provider, from the head of the branch as fetched (bug 24), and
// prints nothing when the provider is none.
func (st *standing) readLastGreen(cfg *config.Loaded, remote, branch string, o Out) {
	remoteURL, _ := git.Output("remote", "get-url", remote)
	look, ok, err := providers.LastGreenProvider(cfg, providers.WatchSetup{
		Env:       os.Getenv,
		RemoteURL: strings.TrimSpace(remoteURL),
		GhToken:   providers.GhToken,
	})
	switch {
	case err != nil:
		st.unread(fmt.Sprintf("The last green commit cannot be read: %s", err))
		return
	case !ok:
		return
	}
	sha, err := look("refs/remotes/" + remote + "/" + branch)
	switch {
	case err != nil:
		st.unread(fmt.Sprintf("The last green commit cannot be read: %s", err))
	case sha == "":
		st.start = append(st.start, "Last green: none found")
	default:
		green := &lastGreen{Commit: sha}
		line := "Last green: " + short(sha)
		if git.HasCommit(sha) {
			header, _ := git.Output("log", "-1", "--format=%s", sha)
			green.Header = strings.TrimSpace(header)
			line += " " + green.Header
		}
		st.LastGreen = green
		st.start = append(st.start, line)
	}
}

// readNightly looks once at the last nightly's run on the branch through
// ci.watch's provider, and prints nothing when it names no nightly.
func (st *standing) readNightly(cfg *config.Loaded, remote, branch string, o Out) {
	remoteURL, _ := git.Output("remote", "get-url", remote)
	look, ok, err := providers.NightlyProvider(cfg, branch, providers.WatchSetup{
		Env:       os.Getenv,
		RemoteURL: strings.TrimSpace(remoteURL),
		GhToken:   providers.GhToken,
	})
	switch {
	case err != nil:
		st.unread(fmt.Sprintf("The nightly cannot be read: %s", err))
		return
	case !ok:
		return
	}
	run, found, err := look()
	switch {
	case err != nil:
		st.unread(fmt.Sprintf("The nightly cannot be read: %s", err))
	case !found:
		st.start = append(st.start, fmt.Sprintf("Nightly: no run on %s yet", branch))
	default:
		if run.Jobs == nil {
			run.Jobs = []providers.Job{}
		}
		_, line := runLine("Nightly", run)
		st.Nightly = &run
		st.start = append(st.start, line)
	}
}

// runLine is a run's result, success, failure (or another conclusion), or
// going while it runs, and its line: the label, the result, its address and
// the jobs that failed.
func runLine(label string, run providers.Run) (result, line string) {
	result = "going"
	if run.Done() {
		result = run.Conclusion
	}
	line = label + ": " + result
	if run.URL != "" {
		line += ", " + run.URL
	}
	var failed []string
	for _, j := range run.Jobs {
		if j.Done() && j.Conclusion != "success" && j.Conclusion != "skipped" && j.Conclusion != "neutral" {
			failed = append(failed, j.Name)
		}
	}
	if len(failed) > 0 {
		line += "; failed jobs: " + strings.Join(failed, ", ")
	}
	return result, line
}

// fields are the status as --json gives it, its keys as standing's, with
// every_item: true only for a session that owns every item, as itos work's.
func (st *standing) fields() []out.Field {
	fields := []out.Field{{Key: "person", Value: st.Person}}
	if st.Every {
		fields = append(fields, out.Field{Key: "every_item", Value: true})
	}
	return append(fields, []out.Field{
		{Key: "head", Value: st.Head},
		{Key: "ci", Value: st.CI},
		{Key: "last_green", Value: st.LastGreen},
		{Key: "nightly", Value: st.Nightly},
		{Key: "release", Value: st.Release},
		{Key: "unreleased", Value: st.Unreleased},
		{Key: "doing", Value: st.Doing},
		{Key: "next", Value: st.Next},
		{Key: "more", Value: st.More},
		{Key: "questions", Value: st.Questions},
		{Key: "unread", Value: st.Unread},
	}...)
}

// print writes the status as text: the heading, the head and its CI run
// (or what could not be reached), the newest release and the commits since
// it, the items in progress, the next ones and the open questions, each a
// short section.
func (st *standing) print(w io.Writer) {
	fmt.Fprintf(w, "%s\n\n", statusHeading)
	for _, line := range st.start {
		fmt.Fprintln(w, line)
	}
	if len(st.released) > 0 {
		fmt.Fprintln(w)
		for _, line := range st.released {
			fmt.Fprintln(w, line)
		}
	}
	whose := ""
	if p, ok := st.Person.(string); ok {
		whose = " for " + p
	}
	itemLine := func(v any) string {
		item := v.(*value.Map)
		s := "  " + value.String(item.At("id")) + "  " + value.String(item.At("title"))
		if st.unowned[v] {
			s += "  (unowned)"
		}
		return s
	}
	fmt.Fprintln(w)
	if st.Person == nil && !st.Every {
		fmt.Fprintln(w, "Working for nobody: only the unowned items are listed.")
	} else if len(st.Doing) == 0 {
		fmt.Fprintf(w, "Nothing in progress%s.\n", whose)
	} else {
		fmt.Fprintf(w, "In progress%s:\n", whose)
		for _, item := range st.Doing {
			fmt.Fprintln(w, itemLine(item))
		}
	}
	if len(st.Next) == 0 {
		fmt.Fprintf(w, "Nothing to start%s.\n", whose)
	} else {
		fmt.Fprintf(w, "Next%s, in the queue's order:\n", whose)
		for _, item := range st.Next {
			fmt.Fprintln(w, itemLine(item))
		}
		if st.More > 0 {
			fmt.Fprintf(w, "  and %d more: itos work\n", st.More)
		}
	}
	if len(st.Questions) == 0 {
		fmt.Fprintln(w, "No open questions.")
		return
	}
	fmt.Fprintln(w, "Open questions:")
	width := 0
	for _, q := range st.Questions {
		width = max(width, len(q.ID))
	}
	for _, q := range st.Questions {
		about := ""
		if q.Item != "" {
			about = "  (" + q.Item + ")"
		}
		fmt.Fprintf(w, "  %-*s  %s%s\n", width, q.ID, clipped(oneLine(q.Question), questionWidth), about)
	}
}
