package status

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/draft"
	"github.com/donvargax/itos/v8/internal/engine/agentrules"
	"github.com/donvargax/itos/v8/internal/engine/ciwatch"
	"github.com/donvargax/itos/v8/internal/engine/pin"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/release"
	"github.com/donvargax/itos/v8/internal/value"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	"github.com/donvargax/itos/v8/internal/version"
	"github.com/donvargax/itos/v8/internal/work"
)

// heading is the status's first line, which itos go's reader looks for.
const heading = "# Where things stand"

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

// fetchTimeout is how long the status waits for the fetch of the branch it
// reads, before reading what it has.
var fetchTimeout = 20 * time.Second

// In is what the status reads beside the config: Identity, who the session
// works for when --as does not say and the people cannot (work.Whoami);
// Stderr, where it is said when that is not among the people; Git, the
// repository the newest release and the rules for agents are read from (the
// head is read through the config's VCS); Setup, what ci.watch's and
// ci.range's providers read, the remote's address set here; Questions, the
// questions of itos decision; and Drafts, the drafts waiting, there false
// outside a repository, where none are listed. Questions and Drafts are
// called once each, in that order, after the items.
type In struct {
	Identity  providers.Identity
	Stderr    io.Writer
	Git       vcsport.VCS
	Setup     providers.WatchSetup
	Questions func() (ask.File, error)
	Drafts    func() (drafts draft.File, there bool, err error)
}

// Standing is where things stand, as --json gives it.
type Standing struct {
	Person any   `json:"person"`
	Every  bool  `json:"every_item,omitempty"`
	Head   *Head `json:"head"`
	CI     *CI   `json:"ci"`
	// LastGreen is the commit main last proved; nil when the head's run
	// passed or was not read, ci.range's provider is none, it names no
	// commit, or it cannot be read.
	LastGreen *LastGreen `json:"last_green"`
	// Nightly is the last nightly's run; nil when ci.watch names no
	// nightly, it has no run yet, or it cannot be read.
	Nightly *providers.Run `json:"nightly"`
	Release *Release       `json:"release"`
	// Behind is how many commits of the head HEAD lacks, after a fetch that
	// worked; nil when none did, the head is not read, or they cannot be
	// counted.
	Behind *int `json:"behind,omitempty"`
	// Unreleased are the headers of the commits since the release the next
	// one would carry, oldest first; nil when there is no release or they
	// cannot be listed.
	Unreleased []string `json:"unreleased"`
	// Itos is the itos the repository runs against the newest release; nil
	// when it runs none it names, the release server is not asked or does
	// not answer.
	Itos *Itos `json:"itos"`
	// AgentRules are the files holding a block of itos's rules for agents
	// other than the one this itos writes; nil when none does.
	AgentRules *AgentRules `json:"agent_rules"`
	Doing      []any       `json:"doing"`
	Next       []any       `json:"next"`
	More       int         `json:"more"`
	Questions  []ask.Entry `json:"questions"`
	// Drafts are the drafts waiting for itos draft promote, in its order.
	Drafts   []draft.Entry `json:"drafts"`
	Unread   []string      `json:"unread"`
	unowned  map[any]bool  // the next items nobody owns
	drafts   []draft.Draft // the drafts waiting, as the list prints them
	start    []string      // the lines of the head and CI, in order
	pull     string        // the line telling to pull before reading the work, when HEAD is behind
	released []string      // the lines of the release section, in order
}

// Release is the newest release: its tag, the commit it names, and whether
// it was read as last fetched, the remote not answering.
type Release struct {
	Tag         string `json:"tag"`
	Commit      string `json:"commit"`
	LastFetched bool   `json:"last_fetched"`
}

// Itos is the version of itos the repository runs, the newest stable
// release of itos, and whether that is a newer major than the one run.
type Itos struct {
	Runs     string `json:"runs"`
	Newest   string `json:"newest"`
	NewMajor bool   `json:"new_major"`
}

// AgentRules are the files holding a block of itos's rules for agents
// other than the one this itos writes, and whether the config is the
// stealth one, which its line says.
type AgentRules struct {
	Stale   []string `json:"stale"`
	stealth bool
}

// agentRulesOf is how the rules for agents stand, as v locates them, nil
// when no block is stale.
func agentRulesOf(ctx context.Context, v vcsport.VCS, stealth bool) *AgentRules {
	stale := agentrules.Stale(ctx, v, stealth)
	if len(stale) == 0 {
		return nil
	}
	return &AgentRules{Stale: stale, stealth: stealth}
}

// Head is the remote branch's head: its commit and header ("" when the
// commit is not fetched here), whether it was read as last fetched, the
// remote not answering, and the repository's visibility, "" when ci.watch's
// provider cannot tell it.
type Head struct {
	Remote      string `json:"remote"`
	Branch      string `json:"branch"`
	Commit      string `json:"commit"`
	Header      string `json:"header"`
	LastFetched bool   `json:"last_fetched"`
	Visibility  string `json:"visibility,omitempty"`
}

// LastGreen is the commit ci.range's provider names as main's last green
// one, and its header ("" when the commit is not fetched here).
type LastGreen struct {
	Commit string `json:"commit"`
	Header string `json:"header"`
}

// CI is the head's CI run as one look saw it: its result, success, failure
// (or another conclusion), going while it runs, or none when there is no
// run of the commit yet.
type CI struct {
	Result string         `json:"result"`
	Run    *providers.Run `json:"run,omitempty"`
}

// Of is where things stand for the person --as names (as, "" when not
// given), else who the people make it, as itos work proposes (Whoami: the
// one person, nobody, or the identity provider's among several). A registry
// that is not there or not sound is its problems; missing is true when --as
// is not among the people, said on in.Stderr.
func Of(ctx context.Context, cfg *config.Loaded, as string, in In) (st *Standing, found []out.Problem, missing bool, err error) {
	found, err = work.ProblemsAt(cfg, cfg.Work.Registry, false)
	if err != nil {
		return nil, nil, false, err
	}
	if len(found) > 0 {
		return nil, found, false, nil
	}
	registry, err := work.Load(cfg, cfg.Work.Registry)
	if err != nil {
		return nil, nil, false, err
	}
	var proposal work.Proposal
	if cfg.Stealth && as == "" {
		proposal = work.ProposeEvery(registry)
	} else {
		listedIn := cfg.Work.People.File
		who := work.Whoami(registry, listedIn, as, in.Identity)
		switch {
		case who.Problem != "":
			fmt.Fprintln(in.Stderr, who.Problem)
			if as != "" {
				return nil, nil, true, nil
			}
		case who.Handle != "" && !who.Listed:
			fmt.Fprintf(in.Stderr, "%s is not in %s: nothing is theirs yet\n", who.Handle, listedIn)
		}
		proposal = work.Propose(registry, who.Handle)
	}
	st = &Standing{Every: proposal.Every, Doing: []any{}, Next: []any{}, Questions: []ask.Entry{}, Unread: []string{},
		unowned: map[any]bool{}}
	if proposal.Person != "" {
		st.Person = proposal.Person
	}
	st.readHead(ctx, cfg, in)
	st.Itos = newestItos(itosRuns(cfg), in.Setup.Env)
	st.AgentRules = agentRulesOf(ctx, in.Git, cfg.Stealth)
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
	asks, err := in.Questions()
	if err != nil {
		st.unread(fmt.Sprintf("The questions cannot be read: %s", err))
	}
	for _, q := range asks.Questions {
		if q.Answer == "" {
			st.Questions = append(st.Questions, ask.EntryOf(q))
		}
	}
	st.Drafts = []draft.Entry{}
	if drafts, there, err := in.Drafts(); there {
		if err != nil {
			st.unread(fmt.Sprintf("The drafts cannot be read: %s", err))
		}
		st.drafts = drafts.Drafts
		for _, d := range drafts.Drafts {
			st.Drafts = append(st.Drafts, draft.EntryOf(d))
		}
	}
	return st, nil, false, nil
}

// unread records what could not be reached, as a line of its own.
func (st *Standing) unread(line string) {
	st.Unread = append(st.Unread, line)
	st.start = append(st.start, line)
}

// readHead reads the remote branch's head and its CI run, then the last
// nightly's run on that branch: the branch's upstream (origin and the
// branch's own name when it has none set), or origin's HEAD from a detached
// HEAD.
func (st *Standing) readHead(ctx context.Context, cfg *config.Loaded, in In) {
	remote, ref := "origin", "HEAD"
	if branch := cfg.VCS.Branch(ctx); branch != "" {
		remote, ref, _ = cfg.VCS.Upstream(ctx, branch)
	}
	branch := strings.TrimPrefix(ref, "refs/heads/")
	st.readBranch(ctx, cfg, remote, ref, branch, in)
	st.readNightly(ctx, cfg, remote, branch, in)
}

// readBranch reads the remote branch's head, its CI run and the newest
// release.
func (st *Standing) readBranch(ctx context.Context, cfg *config.Loaded, remote, ref, branch string, in In) {
	if !hasRemote(ctx, cfg.VCS, remote) {
		st.unread(fmt.Sprintf("No remote %s, so neither its head nor its CI run is read.", remote))
		return
	}
	head := &Head{Remote: remote, Branch: branch}
	sha, reached := lsRemote(ctx, cfg.VCS, remote, ref)
	if reached && sha == "" {
		st.unread(fmt.Sprintf("%s has no branch %s yet, so there is no CI run to read.", remote, branch))
		return
	}
	fetched := false
	if !cfg.Stealth {
		var tip string
		tip, fetched = st.fetch(ctx, cfg.VCS, remote, ref, branch)
		sha = cmp.Or(tip, sha)
	}
	if !reached {
		tracked, err := cfg.VCS.Resolve(ctx, "refs/remotes/"+remote+"/"+branch)
		if err != nil {
			st.unread(fmt.Sprintf("%s cannot be reached and %s/%s was never fetched, so neither its head nor its CI run is read.",
				remote, remote, branch))
			return
		}
		sha = tracked
		head.LastFetched = true
		st.unread(fmt.Sprintf("%s cannot be reached, so its %s is as last fetched.", remote, branch))
	}
	head.Commit = sha
	if shown, err := cfg.VCS.Show(ctx, sha); err == nil {
		head.Header = shown.Subject
	}
	st.Head = head
	wr, ok, err := ciwatch.Watcher(ctx, cfg, cfg.VCS, remote, in.Setup)
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
		st.readLastGreen(ctx, cfg, remote, branch, in)
	}
	if fetched {
		st.readBehind(ctx, cfg.VCS, remote, ref, branch, sha)
	}
	st.readRelease(ctx, in.Git, remote, branch, head)
}

// fetch fetches the one remote branch the status reads into its
// remote-tracking ref, quietly, never prompting for credentials, within
// fetchTimeout, and from a detached HEAD (ref HEAD) the remote's own
// refspecs; it is the commit the branch was fetched at ("" from a detached
// HEAD) and whether that worked, a fetch that failed said in a line naming
// why.
func (st *Standing) fetch(ctx context.Context, v vcsport.VCS, remote, ref, branch string) (tip string, worked bool) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	refspec := ""
	if ref != "HEAD" {
		refspec = "+" + ref + ":refs/remotes/" + remote + "/" + branch
	}
	tip, err := v.Fetch(ctx, remote, refspec, vcsport.Talk{NoPrompt: true})
	if err == nil {
		return tip, true
	}
	st.unread(fmt.Sprintf("%s/%s cannot be fetched: %s", remote, branch, whyNot(ctx, err)))
	return "", false
}

// whyNot is why a fetch failed: git's first line as git writes it, or that
// there was no answer within the time.
func whyNot(ctx context.Context, err error) string {
	if ctx.Err() != nil {
		return fmt.Sprintf("no answer within %d s", int(fetchTimeout/time.Second))
	}
	words := err.Error()
	var failed *vcsport.Failed
	if errors.As(err, &failed) && strings.TrimSpace(failed.Words) != "" {
		words = failed.Words
	}
	first, _, _ := strings.Cut(strings.TrimSpace(words), "\n")
	return strings.TrimSpace(first)
}

// readBehind counts the commits of the head that HEAD lacks, the items
// being read from the working tree, which the fetch did not move; the line
// telling to pull is set when there are any, a detached HEAD, where git
// refuses a pull, told to switch to a branch first.
func (st *Standing) readBehind(ctx context.Context, v vcsport.VCS, remote, ref, branch, head string) {
	commits, err := v.Commits(ctx, vcsport.Range{From: "HEAD", To: head})
	if err != nil {
		return
	}
	behind := len(commits)
	st.Behind = &behind
	if behind == 0 {
		return
	}
	count := fmt.Sprintf("%d commits", behind)
	if behind == 1 {
		count = "1 commit"
	}
	if ref == "HEAD" {
		st.pull = fmt.Sprintf("HEAD is %s behind %s/%s: switch to a branch and pull before reading the work below.", count, remote, branch)
		return
	}
	st.pull = fmt.Sprintf("%s is %s behind %s/%s: git pull before reading the work below.", v.Branch(ctx), count, remote, branch)
}

// hasRemote is whether the repository has the remote.
func hasRemote(ctx context.Context, v vcsport.VCS, remote string) bool {
	_, err := v.RemoteURL(ctx, remote)
	return err == nil
}

// lsRemote is the commit the remote's ref names, "" when it has no such
// ref; reached is false when the remote did not answer in time, or at all.
// It never prompts for credentials.
func lsRemote(ctx context.Context, v vcsport.VCS, remote, ref string) (sha string, reached bool) {
	refs, reached := askRemote(ctx, v, remote, ref)
	return refs[ref], reached
}

// askRemote is the remote's refs, those given or else all, each name the
// commit or object it names; reached is false when the remote did not
// answer in time, or at all. It never prompts for credentials.
func askRemote(ctx context.Context, v vcsport.VCS, remote string, refs ...string) (map[string]string, bool) {
	ctx, cancel := context.WithTimeout(ctx, lsRemoteTimeout)
	defer cancel()
	found, err := v.RemoteRefs(ctx, remote, refs, vcsport.Talk{NoPrompt: true})
	return found, err == nil
}

// releaseTags are the tags the remote has, each the commit it names, an
// annotated tag peeled to its commit; reached is false when the remote did
// not answer. Asked of the remote alone when it answered for the head, so
// a remote that did not is not waited for twice.
func releaseTags(ctx context.Context, v vcsport.VCS, remote string, ask bool) (tags map[string]string, reached bool) {
	if !ask {
		return nil, false
	}
	refs, reached := askRemote(ctx, v, remote)
	if !reached {
		return nil, false
	}
	return peeled(refs, func(ref string) (string, bool) { return strings.CutPrefix(ref, "refs/tags/") }), true
}

// fetchedTags are the tags this clone has, as last fetched, each the commit
// it names.
func fetchedTags(ctx context.Context, v vcsport.VCS) map[string]string {
	refs, err := v.Refs(ctx, "refs/tags/")
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
// the release to the remote's head, read from the commits as fetched here,
// through v. Where the head is not fetched here, they are listed to the
// remote branch as last fetched, and a line says the list may be behind.
func (st *Standing) readRelease(ctx context.Context, v vcsport.VCS, remote, branch string, head *Head) {
	tags, reached := releaseTags(ctx, v, remote, !head.LastFetched)
	if !reached {
		if !head.LastFetched {
			st.releaseUnread(fmt.Sprintf("%s's tags cannot be read, so its newest release is as last fetched.", remote))
		}
		tags = fetchedTags(ctx, v)
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
	st.Release = &Release{Tag: tag, Commit: tags[tag], LastFetched: !reached}
	line := "Newest release: " + tag
	if !reached {
		line += " (as last fetched)"
	}
	st.released = append(st.released, line)
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
	script, found, err := pin.ReadInstallScript(cfg.Folder())
	if err != nil || !found {
		return ""
	}
	v, _ := pin.ScriptVersion(script)
	return v
}

// newestItos is the itos the repository runs, runs, against the newest
// stable release, asked of the release server once, as a bare itos pin asks
// (<ITOS_RELEASES>/latest/download/checksums.txt), within newestItosTimeout
// and never again; nil when runs is "", where the launcher asks nothing (CI
// or ITOS_NO_UPDATE set in env, as its notice reads them), and when the
// server does not answer or names no release, or a pre-release, which is
// never the newest.
func newestItos(runs string, env providers.Env) *Itos {
	if runs == "" || env("CI") != "" || env("ITOS_NO_UPDATE") != "" {
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
	return &Itos{Runs: runs, Newest: newest, NewMajor: version.Major(newest) > version.Major(runs)}
}

// line is the itos line of the status: both versions, and itos upgrade with
// the release's notes only for a newer major.
func (s *Itos) line() string {
	if s.NewMajor {
		return fmt.Sprintf("itos: this repository runs %s; the newest release is %s, a new major: "+
			"itos upgrade moves it there (notes: %s)", s.Runs, s.Newest, release.NotesURL(s.Newest))
	}
	return fmt.Sprintf("itos: this repository runs %s; the newest release is %s.", s.Runs, s.Newest)
}

// releaseUnread records what of the release could not be read, as a line
// of the release section.
func (st *Standing) releaseUnread(line string) {
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
func (st *Standing) readCI(wr providers.Watcher, ok bool, err error, sha string) {
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
		st.CI = &CI{Result: "none"}
		st.start = append(st.start, fmt.Sprintf("CI: no run of %s yet", short(sha)))
	default:
		if run.Jobs == nil {
			run.Jobs = []providers.Job{}
		}
		result, line := runLine("CI", run)
		st.CI = &CI{Result: result, Run: &run}
		st.start = append(st.start, line)
	}
}

// readLastGreen looks once at the commit main last proved, through
// ci.range's provider, from the head of the branch as fetched (bug 24), and
// prints nothing when the provider is none.
func (st *Standing) readLastGreen(ctx context.Context, cfg *config.Loaded, remote, branch string, in In) {
	remoteURL, _ := cfg.VCS.RemoteURL(ctx, remote)
	setup := in.Setup
	setup.RemoteURL = strings.TrimSpace(remoteURL)
	look, ok, err := providers.LastGreenProvider(cfg, setup)
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
		green := &LastGreen{Commit: sha}
		line := "Last green: " + short(sha)
		if shown, err := cfg.VCS.Show(ctx, sha); err == nil {
			green.Header = shown.Subject
			line += " " + green.Header
		}
		st.LastGreen = green
		st.start = append(st.start, line)
	}
}

// readNightly looks once at the last nightly's run on the branch through
// ci.watch's provider, and prints nothing when it names no nightly.
func (st *Standing) readNightly(ctx context.Context, cfg *config.Loaded, remote, branch string, in In) {
	remoteURL, _ := cfg.VCS.RemoteURL(ctx, remote)
	setup := in.Setup
	setup.RemoteURL = strings.TrimSpace(remoteURL)
	look, ok, err := providers.NightlyProvider(cfg, branch, setup)
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

// Fields are the status as --json gives it, its keys as Standing's, with
// every_item: true only for a session that owns every item, as itos work's.
func (st *Standing) Fields() []out.Field {
	fields := []out.Field{{Key: "person", Value: st.Person}}
	if st.Every {
		fields = append(fields, out.Field{Key: "every_item", Value: true})
	}
	fields = append(fields, out.Field{Key: "head", Value: st.Head})
	if st.Behind != nil {
		fields = append(fields, out.Field{Key: "behind", Value: *st.Behind})
	}
	return append(fields, []out.Field{
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

// Print writes the status as text: the heading, the head and its CI run
// (or what could not be reached), the newest release and the commits since
// it, the itos the repository runs against the newest itos and whether its
// rules for agents are stale, the items in progress, the next ones and the
// open questions, each a short section.
func (st *Standing) Print(w io.Writer) {
	fmt.Fprintf(w, "%s\n\n", heading)
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
		advice = append(advice, agentrules.StaleLine(st.AgentRules.stealth))
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
	if st.pull != "" {
		fmt.Fprintf(w, "%s\n\n", st.pull)
	}
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
func (st *Standing) printDrafts(w io.Writer) {
	if len(st.drafts) == 0 {
		return
	}
	fmt.Fprintln(w, "Drafts waiting, for itos draft promote once no work is going on:")
	for _, line := range draft.Lines(st.drafts) {
		fmt.Fprintln(w, line)
	}
}

// short is a commit's short name, its first seven characters.
func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// oneLine is text on one line, its runs of white space each one space.
func oneLine(text string) string { return strings.Join(strings.Fields(text), " ") }
