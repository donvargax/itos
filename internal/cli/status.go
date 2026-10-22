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
// order (a handful), the open questions of itos decision, and (slice 96) the
// drafts waiting for itos draft promote. After the release (slice 116), the
// version of itos the repository runs, its pin else its install script's as
// itos upgrade reads it, and the newest stable release of itos, asked of the
// release server once, naming itos upgrade only for a newer major: only a new
// major is something to act on. Beside it (slice 124), a line naming itos
// init --agent-rules when AGENTS.md, or under a stealth config the copies
// init writes there, holds a block of itos's rules for agents other than the
// one this itos writes, as init's rerun decides it (staleRules,
// initrules.go): a project crossing majors with an older itos on its PATH
// gets no block rewritten by itself. The head's line names the repository's
// visibility (slice 135), public, private or internal, asked once of
// ci.watch's provider, and nothing of it when the provider cannot tell. It
// reads, never writes. What cannot be reached is
// one line naming it, and the rest still prints, exit 0; a status that
// cannot be computed at all (no config, no registry, one with problems) is
// exit 2 or 1, as itos work's. itos go prints it after the guides
// (guide.go), and there skips one that cannot be computed with a line.

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/draft"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/release"
	"github.com/donvargax/itos/v8/internal/value"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	"github.com/donvargax/itos/v8/internal/version"
	"github.com/donvargax/itos/v8/internal/work"
)

// statusHeading is the status's first line, which itos go's reader looks for.
const statusHeading = "# Where things stand"

// questionWidth is how much of a question the status prints, in characters,
// the rest left to itos decision show.
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

// newestItosTimeout is how long the status waits for the release server to
// name the newest itos, before leaving the itos line out.
var newestItosTimeout = 5 * time.Second

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
	Unreleased []string `json:"unreleased"`
	// Itos is the itos the repository runs against the newest release; nil
	// when it runs none it names, the release server is not asked or does
	// not answer.
	Itos *itosStanding `json:"itos"`
	// AgentRules are the files holding a block of itos's rules for agents
	// other than the one this itos writes; nil when none does.
	AgentRules *agentRules `json:"agent_rules"`
	Doing      []any       `json:"doing"`
	Next       []any       `json:"next"`
	More       int         `json:"more"`
	Questions  []askEntry  `json:"questions"`
	// Drafts are the drafts waiting for itos draft promote, in its order.
	Drafts   []draftEntry  `json:"drafts"`
	Unread   []string      `json:"unread"`
	unowned  map[any]bool  // the next items nobody owns
	drafts   []draft.Draft // the drafts waiting, as the list prints them
	start    []string      // the lines of the head and CI, in order
	released []string      // the lines of the release section, in order
}

// newest is the newest release: its tag, the commit it names, and whether
// it was read as last fetched, the remote not answering.
type newest struct {
	Tag         string `json:"tag"`
	Commit      string `json:"commit"`
	LastFetched bool   `json:"last_fetched"`
}

// itosStanding is the version of itos the repository runs, the newest stable
// release of itos, and whether that is a newer major than the one run.
type itosStanding struct {
	Runs     string `json:"runs"`
	Newest   string `json:"newest"`
	NewMajor bool   `json:"new_major"`
}

// agentRules are the files holding a block of itos's rules for agents
// other than the one this itos writes, and whether the config is the
// stealth one, which its line says.
type agentRules struct {
	Stale   []string `json:"stale"`
	stealth bool
}

// agentRulesOf is how the rules for agents stand, nil when no block is
// stale.
func agentRulesOf(stealth bool) *agentRules {
	stale := staleRules(stealth)
	if len(stale) == 0 {
		return nil
	}
	return &agentRules{Stale: stale, stealth: stealth}
}

// remoteHead is the remote branch's head: its commit and header ("" when
// the commit is not fetched here), whether it was read as last fetched, the
// remote not answering, and the repository's visibility, "" when ci.watch's
// provider cannot tell it.
type remoteHead struct {
	Remote      string `json:"remote"`
	Branch      string `json:"branch"`
	Commit      string `json:"commit"`
	Header      string `json:"header"`
	LastFetched bool   `json:"last_fetched"`
	Visibility  string `json:"visibility,omitempty"`
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
	cfg, err := loadConfig()
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
	st.Itos = newestItos(itosRuns(cfg))
	st.AgentRules = agentRulesOf(cfg.Stealth)
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
	st.Drafts = []draftEntry{}
	if file := draftsFile(); file != "" {
		drafts, err := draft.Load(file)
		if err != nil {
			st.unread(fmt.Sprintf("The drafts cannot be read: %s", err))
		}
		st.drafts = drafts.Drafts
		for _, d := range drafts.Drafts {
			st.Drafts = append(st.Drafts, entryOfDraft(d))
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
	if branch := cfg.VCS.Branch(context.Background()); branch != "" {
		remote, ref, _ = cfg.VCS.Upstream(context.Background(), branch)
	}
	branch := strings.TrimPrefix(ref, "refs/heads/")
	st.readBranch(cfg, remote, ref, branch, o)
	st.readNightly(cfg, remote, branch, o)
}

// readBranch reads the remote branch's head, its CI run and the newest
// release.
func (st *standing) readBranch(cfg *config.Loaded, remote, ref, branch string, o Out) {
	if !hasRemote(cfg.VCS, remote) {
		st.unread(fmt.Sprintf("No remote %s, so neither its head nor its CI run is read.", remote))
		return
	}
	head := &remoteHead{Remote: remote, Branch: branch}
	sha, reached := lsRemote(cfg.VCS, remote, ref)
	switch {
	case reached && sha == "":
		st.unread(fmt.Sprintf("%s has no branch %s yet, so there is no CI run to read.", remote, branch))
		return
	case !reached:
		fetched, err := cfg.VCS.Resolve(context.Background(), "refs/remotes/"+remote+"/"+branch)
		if err != nil {
			st.unread(fmt.Sprintf("%s cannot be reached and %s/%s was never fetched, so neither its head nor its CI run is read.",
				remote, remote, branch))
			return
		}
		sha = fetched
		head.LastFetched = true
		st.unread(fmt.Sprintf("%s cannot be reached, so its %s is as last fetched.", remote, branch))
	}
	head.Commit = sha
	if shown, err := cfg.VCS.Show(context.Background(), sha); err == nil {
		head.Header = shown.Subject
	}
	st.Head = head
	wr, ok, err := watcher(cfg, remote, o)
	head.Visibility = visibility(wr, ok, err)
	line := remote + "/" + branch
	if head.Visibility != "" {
		line += " (" + head.Visibility + ")"
	}
	line += ": " + short(sha)
	if head.Header != "" {
		line += " " + head.Header
	} else {
		line += " (not fetched here)"
	}
	st.start = append(st.start, line)
	st.readCI(wr, ok, err, sha)
	if st.CI != nil && st.CI.Result != "success" {
		st.readLastGreen(cfg, remote, branch, o)
	}
	st.readRelease(remote, branch, head)
}

// lsRemote is the commit the remote's ref names, "" when it has no such
// ref; reached is false when the remote did not answer in time, or at all.
// It never prompts for credentials.
func lsRemote(v vcsport.VCS, remote, ref string) (sha string, reached bool) {
	refs, reached := askRemote(v, remote, ref)
	return refs[ref], reached
}

// askRemote is the remote's refs, those given or else all, each name the
// commit or object it names; reached is false when the remote did not
// answer in time, or at all. It never prompts for credentials.
func askRemote(v vcsport.VCS, remote string, refs ...string) (map[string]string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), lsRemoteTimeout)
	defer cancel()
	found, err := v.RemoteRefs(ctx, remote, refs, vcsport.Talk{NoPrompt: true})
	return found, err == nil
}

// releaseTags are the tags the remote has, each the commit it names, an
// annotated tag peeled to its commit; reached is false when the remote did
// not answer. Asked of the remote alone when it answered for the head, so
// a remote that did not is not waited for twice.
func releaseTags(v vcsport.VCS, remote string, ask bool) (tags map[string]string, reached bool) {
	if !ask {
		return nil, false
	}
	refs, reached := askRemote(v, remote)
	if !reached {
		return nil, false
	}
	return peeled(refs, func(ref string) (string, bool) { return strings.CutPrefix(ref, "refs/tags/") }), true
}

// fetchedTags are the tags this clone has, as last fetched, each the commit
// it names.
func fetchedTags(v vcsport.VCS) map[string]string {
	refs, err := v.Refs(context.Background(), "refs/tags/")
	if err != nil {
		return map[string]string{}
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
	tags, reached := releaseTags(vcs.Git{}, remote, !head.LastFetched)
	if !reached {
		if !head.LastFetched {
			st.releaseUnread(fmt.Sprintf("%s's tags cannot be read, so its newest release is as last fetched.", remote))
		}
		tags = fetchedTags(vcs.Git{})
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
	ctx, v := context.Background(), vcs.Git{}
	tip, asFetched := head.Commit, ""
	if _, err := v.Resolve(ctx, tip); err != nil {
		asFetched = ", as fetched here"
		if tip, err = v.Resolve(ctx, "refs/remotes/"+remote+"/"+branch); err != nil {
			st.releaseUnread(fmt.Sprintf("%s/%s is not fetched here, so the commits since %s are not listed.", remote, branch, tag))
			return
		}
		st.releaseUnread(fmt.Sprintf("%s/%s is not fetched here up to its head, so the commits since %s may be behind.",
			remote, branch, tag))
	}
	if _, err := v.Resolve(ctx, st.Release.Commit); err != nil {
		st.releaseUnread(fmt.Sprintf("%s's commit is not fetched here, so the commits since it are not listed.", tag))
		return
	}
	log, err := v.Log(ctx, vcsport.Range{From: st.Release.Commit, To: tip})
	if err != nil {
		st.releaseUnread(fmt.Sprintf("The commits since %s cannot be read.", tag))
		return
	}
	st.Unreleased = []string{}
	for _, e := range log {
		message := strings.TrimSpace(e.Message)
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

// itosRuns is the version of itos the repository runs, as itos upgrade reads
// the one it moves from: the config's pin, else the version= line of the
// install script; "" with neither, or one that names no version.
func itosRuns(cfg *config.Loaded) string {
	if cfg.Pin != nil && config.PinVersion.MatchString(cfg.Pin.Version) {
		return cfg.Pin.Version
	}
	script, found, err := readInstallScript()
	if err != nil || !found {
		return ""
	}
	v, _ := scriptVersion(script)
	return v
}

// newestItos is the itos the repository runs, runs, against the newest
// stable release, asked of the release server once, as a bare itos pin asks
// (<ITOS_RELEASES>/latest/download/checksums.txt), within newestItosTimeout
// and never again; nil when runs is "", where the launcher asks nothing (CI
// or ITOS_NO_UPDATE set, as its notice reads them), and when the server does
// not answer or names no release, or a pre-release, which is never the
// newest.
func newestItos(runs string) *itosStanding {
	if runs == "" || os.Getenv("CI") != "" || os.Getenv("ITOS_NO_UPDATE") != "" {
		return nil
	}
	sums, err := release.Get(release.LatestURL("checksums.txt"), newestItosTimeout)
	if err != nil {
		return nil
	}
	newest := release.VersionOf(sums)
	if !release.IsTag("v" + newest) {
		return nil
	}
	return &itosStanding{Runs: runs, Newest: newest, NewMajor: version.Major(newest) > version.Major(runs)}
}

// line is the itos line of the status: both versions, and itos upgrade with
// the release's notes only for a newer major.
func (s *itosStanding) line() string {
	if s.NewMajor {
		return fmt.Sprintf("itos: this repository runs %s; the newest release is %s, a new major: "+
			"itos upgrade moves it there (notes: %s)", s.Runs, s.Newest, release.NotesURL(s.Newest))
	}
	return fmt.Sprintf("itos: this repository runs %s; the newest release is %s.", s.Runs, s.Newest)
}

// releaseUnread records what of the release could not be read, as a line
// of the release section.
func (st *standing) releaseUnread(line string) {
	st.Unread = append(st.Unread, line)
	st.released = append(st.released, line)
}

// visibility is the repository's visibility, asked once of ci.watch's
// provider (slice 135); "" when there is none, it cannot tell it, or the
// asking fails, which status says nothing of.
func visibility(wr providers.Watcher, ok bool, err error) string {
	if err != nil || !ok || wr.Visibility == nil {
		return ""
	}
	v, err := wr.Visibility()
	if err != nil {
		return ""
	}
	return v
}

// readCI looks once at the commit's CI run through ci.watch's provider, wr,
// none when ok is false, or err when it cannot look.
func (st *standing) readCI(wr providers.Watcher, ok bool, err error, sha string) {
	switch {
	case err != nil:
		st.unread(fmt.Sprintf("CI cannot be read: %s", err))
		return
	case !ok:
		st.unread("CI is not read: ci.watch.provider is none.")
		return
	}
	run, found, err := wr.Look(sha)
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
	remoteURL, _ := cfg.VCS.RemoteURL(context.Background(), remote)
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
		if shown, err := cfg.VCS.Show(context.Background(), sha); err == nil {
			green.Header = shown.Subject
			line += " " + green.Header
		}
		st.LastGreen = green
		st.start = append(st.start, line)
	}
}

// readNightly looks once at the last nightly's run on the branch through
// ci.watch's provider, and prints nothing when it names no nightly.
func (st *standing) readNightly(cfg *config.Loaded, remote, branch string, o Out) {
	remoteURL, _ := cfg.VCS.RemoteURL(context.Background(), remote)
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
		{Key: "itos", Value: st.Itos},
		{Key: "agent_rules", Value: st.AgentRules},
		{Key: "doing", Value: st.Doing},
		{Key: "next", Value: st.Next},
		{Key: "more", Value: st.More},
		{Key: "questions", Value: st.Questions},
		{Key: "drafts", Value: st.Drafts},
		{Key: "unread", Value: st.Unread},
	}...)
}

// print writes the status as text: the heading, the head and its CI run
// (or what could not be reached), the newest release and the commits since
// it, the itos the repository runs against the newest itos and whether its
// rules for agents are stale, the items in progress, the next ones and the
// open questions, each a short section.
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
	var advice []string
	if st.Itos != nil {
		advice = append(advice, st.Itos.line())
	}
	if st.AgentRules != nil {
		advice = append(advice, staleRulesLine(st.AgentRules.stealth))
	}
	if len(advice) > 0 {
		fmt.Fprintf(w, "\n%s\n", strings.Join(advice, "\n"))
	}
	whose := ""
	if p, ok := st.Person.(string); ok {
		whose = " for " + p
	}
	itemLine := func(v any) string {
		item := v.(*value.Map)
		s := "  " + work.TaggedID(item) + "  " + value.String(item.At("title"))
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
	defer st.printDrafts(w)
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

// printDrafts writes the drafts waiting, when there are any: what itos
// draft promote would apply, between agents.
func (st *standing) printDrafts(w io.Writer) {
	if len(st.drafts) == 0 {
		return
	}
	fmt.Fprintln(w, "Drafts waiting, for itos draft promote once no work is going on:")
	for _, line := range draftLines(st.drafts) {
		fmt.Fprintln(w, line)
	}
}
