package cli

// itos status (slice 67, features/status.feature): where the work stands,
// for the person a session works for, from what already records it, in
// place of a handoff file rewritten by hand. The remote branch's head is
// asked of the remote (git ls-remote), else read as last fetched; its CI run
// is looked at once through ci.watch's provider, never waited for, a run
// still going reported as going; then the person's items in progress, the
// next ones they can start in the queue's order (a handful), and the open
// questions of itos ask. It reads, never writes. What cannot be reached is
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

	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/git"
	"github.com/donvargax/itos/v2/internal/out"
	"github.com/donvargax/itos/v2/internal/providers"
	"github.com/donvargax/itos/v2/internal/value"
	"github.com/donvargax/itos/v2/internal/work"
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
	Person    any          `json:"person"`
	Every     bool         `json:"every_item,omitempty"`
	Head      *remoteHead  `json:"head"`
	CI        *ciStanding  `json:"ci"`
	Doing     []any        `json:"doing"`
	Next      []any        `json:"next"`
	More      int          `json:"more"`
	Questions []askEntry   `json:"questions"`
	Unread    []string     `json:"unread"`
	unowned   map[any]bool // the next items nobody owns
	start     []string     // the lines of the head and CI, in order
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
// given), else the identity provider's, as itos work proposes. A registry
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
		who := work.Whoami(registry, listedIn, as, providers.IdentityProvider(cfg, o.Stderr))
		switch {
		case who.Problem != "":
			fmt.Fprintln(o.Stderr, who.Problem)
			if as != "" {
				return nil, nil, ExitMissing, nil
			}
		case !who.Listed:
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

// readHead reads the remote branch's head and its CI run: the branch's
// upstream (origin and the branch's own name when it has none set), or
// origin's HEAD from a detached HEAD.
func (st *standing) readHead(cfg *config.Loaded, o Out) {
	remote, ref := "origin", "HEAD"
	if branch := git.Branch(); branch != "" {
		remote, ref, _ = git.Upstream(branch)
	}
	branch := strings.TrimPrefix(ref, "refs/heads/")
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
}

// lsRemote is the commit the remote's ref names, "" when it has no such
// ref; reached is false when the remote did not answer in time, or at all.
// It never prompts for credentials.
func lsRemote(remote, ref string) (sha string, reached bool) {
	ctx, cancel := context.WithTimeout(context.Background(), lsRemoteTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, git.Bin(), "ls-remote", remote, ref)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", false
	}
	for _, line := range strings.Split(stdout.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == ref {
			return fields[0], true
		}
	}
	return "", true
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
		result := "going"
		if run.Done() {
			result = run.Conclusion
		}
		st.CI = &ciStanding{Result: result, Run: &run}
		line := "CI: " + result
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
		st.start = append(st.start, line)
	}
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
		{Key: "doing", Value: st.Doing},
		{Key: "next", Value: st.Next},
		{Key: "more", Value: st.More},
		{Key: "questions", Value: st.Questions},
		{Key: "unread", Value: st.Unread},
	}...)
}

// print writes the status as text: the heading, the head and its CI run
// (or what could not be reached), the items in progress, the next ones and
// the open questions, each a short section.
func (st *standing) print(w io.Writer) {
	fmt.Fprintf(w, "%s\n\n", statusHeading)
	for _, line := range st.start {
		fmt.Fprintln(w, line)
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
