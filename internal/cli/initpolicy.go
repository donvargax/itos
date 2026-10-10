package cli

// itos init --policy <file> (slice 108, issue #33, decision q-49): a
// project made from a template takes the template's reviewed policy, never
// its work. The file is an ordinary, complete itos config, read once from
// where the person stood and copied into the new project as its own config,
// never followed afterwards: not --config, and no inheritance.
//
// Everything is judged before anything is written, git init included: the
// policy, as config check judges a config; the config made of it; whether
// the fresh data can be written (the adoption task T-1 that ledger.id,
// commits.types and the ledger's group layout must be able to hold, every
// file inside the project's root, or the stealth folder, once symbolic links
// are followed); that nothing of a project is there already (a config, a
// file of the ledger, the registry or a smoke set, even an empty one); and
// each smoke set, derived from the list of the project's own tests. A
// refusal writes nothing; there is no merge and no --force.
//
// The config is the policy's text with its comments, edited in place
// (value.Doc): commits.since set to the project's HEAD, or dropped where it
// has no commit, and every footer's since dropped, since the template's
// commits are not the project's. The policy's pin is kept; with none, init
// pins the newest release as it does for the starter. Where the policy has
// no ledger, the starter's layout is added, and nothing else of the
// starter's. Then the fresh data at the paths the config names: the ledger's
// group 1 holding the adoption task (none under --stealth, as for the
// starter), a registry with no item and that group owned by nobody under
// work.groups_key, and for each kind with a smoke set one naming each
// file's first live test of the project's own tests. A command adapter's
// list (slice 110) runs once for that, as itos tests list runs it in the
// project, before anything is written; a list that fails, or whose output
// the protocol refuses, refuses init with its error. Nothing else of the
// template's (its tasks, registry, questions, decisions, smoke sets, proof
// results or mutation caches) is read, and nothing else it names runs.
// Then the hooks and the offers, as init's starter has them. A failure
// after the first write reports what this run wrote, and removes nothing.

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/git"
	"github.com/donvargax/itos/v7/internal/ledger"
	"github.com/donvargax/itos/v7/internal/out"
	"github.com/donvargax/itos/v7/internal/release"
	"github.com/donvargax/itos/v7/internal/tests"
	"github.com/donvargax/itos/v7/internal/value"
)

// The adoption task init --policy writes in a project, its type and its
// group.
const (
	adoptionTask  = "T-1"
	adoptionType  = "chore"
	adoptionGroup = "1"
)

// policyFile is a file init --policy writes: the config key that names
// where it goes ("" for the config itself), its path and its text.
type policyFile struct {
	key, path, text string
}

// policyPlan is what init --policy would write, judged before it writes any
// of it: the config, whether it is the stealth one, commits.since ("" in a
// repository with no commit), whether the folder is a repository already,
// the files, the config first, and the pin (nil for none), whether the
// policy gave it, and why there is none.
type policyPlan struct {
	file, since     string
	stealth, inRepo bool
	files           []policyFile
	pin             *[2]string
	kept            bool
	pinErr          error
}

// policyInit is init --policy: the policy judged and the project checked
// before anything is written, then the config made of it, the fresh data,
// the hooks and the offers.
func policyInit(policy string, stealth bool, offer pluginOffer, shimOffer shimOffer, rulesOffer rulesOffer,
	log io.Writer, o Out) (int, error) {
	if named := os.Getenv("ITOS_CONFIG"); named != "" {
		return 0, usage("init --policy writes the project's own config, itos.yaml at its top or with --stealth in "+
			"the git folder, not the %s that --config or ITOS_CONFIG names", named)
	}
	plan, refusal, err := policyPlanned(policy, stealth)
	if err != nil {
		return 0, err
	}
	if refusal != nil {
		return policyRefused(plan.file, refusal, o)
	}
	initialized := !plan.inRepo
	if initialized {
		if err := gitInit(log); err != nil {
			return 0, err
		}
	}
	var written []writtenFile
	for _, f := range plan.files {
		if err := writeNew(f.path, f.text); err != nil {
			return 0, partial(err, initialized, written)
		}
		written = append(written, writtenFile{filepath.ToSlash(f.path), "wrote"})
	}
	if !o.JSON {
		policySaid(plan, policy, written, log)
	}
	hooksCode, hooks, err := initHooks(o)
	if err != nil {
		return 0, partial(err, initialized, written)
	}
	offer.log, shimOffer.log, rulesOffer.log = log, log, log
	if o.JSON {
		offer.log, shimOffer.log, rulesOffer.log = io.Discard, io.Discard, io.Discard
	}
	plugin, pluginCode := offer.run()
	shim, shimCode := shimOffer.run()
	rulesOffer.file = plan.file
	rules, rulesCode := rulesOffer.run()
	code := hooksCode
	if code == 0 {
		code = max(pluginCode, shimCode, rulesCode)
	}
	if o.JSON {
		var since, pin any
		if plan.since != "" {
			since = plan.since
		}
		if plan.pin != nil {
			pin = map[string]string{"version": plan.pin[0], "checksums": plan.pin[1]}
		}
		fields := []out.Field{{Key: "config", Value: filepath.ToSlash(plan.file)}, {Key: "action", Value: "initialized"},
			{Key: "policy", Value: filepath.ToSlash(policy)}, {Key: "git_init", Value: initialized},
			{Key: "since", Value: since}, {Key: "files", Value: written}, {Key: "pin", Value: pin}}
		if plan.pinErr != nil {
			fields = append(fields, out.Field{Key: "pin_problem", Value: plan.pinErr.Error()})
		}
		fields = append(fields, out.Field{Key: "hooks", Value: hooks}, out.Field{Key: "plugin", Value: plugin},
			out.Field{Key: "git_shim", Value: shim}, out.Field{Key: "agent_rules", Value: rules})
		return code, out.Emit(o.Stdout, fields...)
	}
	if hooksCode == 0 {
		policyNext(plan, written, plugin, rules, log)
	}
	return code, nil
}

// policyPlanned reads and judges the policy, from where the person stood,
// and plans what init would write of it, then finds what of a project is
// there already and derives the smoke sets: a refusal with nothing written,
// git init included.
func policyPlanned(policy string, stealth bool) (policyPlan, *policyRefusal, error) {
	plan := policyPlan{file: "itos.yaml", stealth: stealth}
	text, err := os.ReadFile(typed(policy))
	if err != nil {
		return plan, nil, &config.Error{File: policy, Problems: []out.Problem{{Rule: "config-unreadable",
			Message: "cannot be read: " + err.Error(), Fix: "name the itos config whose policy the project takes"}}}
	}
	src, err := config.FromText(policy, string(text), false)
	if err != nil {
		return plan, nil, err
	}
	if plan.inRepo, err = toTop(); err != nil {
		return plan, nil, err
	}
	found, common := "", ".git"
	if plan.inRepo {
		found = config.Path()
		if out, err := git.Output("rev-parse", "--git-common-dir"); err == nil {
			common = value.Trim(out)
		}
		if head, err := git.Output("rev-parse", "--verify", "--quiet", "HEAD^{commit}"); err == nil {
			plan.since = value.Trim(head)
		}
	}
	if stealth {
		plan.file = filepath.Join(common, config.StealthFolder, "itos.yaml")
	}
	if plan.kept = src.Pin != nil; plan.kept {
		plan.pin = &[2]string{src.Pin.Version, src.Pin.Checksums}
	} else if version, sums, err := pinned(""); err == nil {
		plan.pin = &[2]string{version, release.SHA256(sums)}
	} else {
		plan.pinErr = err
	}
	cfg, made, err := policyConfig(policy, string(text), src, plan)
	if err != nil {
		return plan, nil, err
	}
	files, problems := policyData(cfg, stealth)
	problems = append(problems, policyPaths(filepath.Dir(plan.file), files)...)
	if len(problems) > 0 {
		return plan, nil, &config.Error{File: policy, Problems: problems}
	}
	if there := policyThere(cfg, plan.file, found, files); there != nil {
		return plan, there, nil
	}
	plan.files = append([]policyFile{{path: plan.file, text: made}}, files...)
	refusal, err := policySmokeSets(cfg, plan.files)
	return plan, refusal, err
}

// policySaid is what init --policy wrote, from where, from when and pinning
// which itos.
func policySaid(plan policyPlan, policy string, written []writtenFile, log io.Writer) {
	say := func(format string, a ...any) { fmt.Fprintf(log, format+"\n", a...) }
	for _, f := range written {
		say("%s %s", f.Action, f.Path)
	}
	say("%s holds the policy of %s, made fresh for this project: no task, owner, status, commit or result of "+
		"the policy's own came with it.", filepath.ToSlash(plan.file), filepath.ToSlash(policy))
	if plan.since != "" {
		say("commits.since is HEAD, %s: no commit before it is judged.", plan.since[:7])
	} else {
		say("The repository has no commit, so the config has no commits.since: every commit is judged.")
	}
	switch {
	case plan.kept:
		say("Pinned itos %s, as the policy does.", plan.pin[0])
	case plan.pin != nil:
		say("Pinned itos %s, the newest release.", plan.pin[0])
	default:
		say("Pinned no itos: %s. Run itos pin when the release server can be reached.", plan.pinErr)
	}
}

// policyNext is what to do next: commit what init wrote with the adoption
// task, or under --stealth nothing at all.
func policyNext(plan policyPlan, written []writtenFile, plugin pluginOutcome, rules rulesOutcome, log io.Writer) {
	if plan.stealth {
		fmt.Fprintln(log, "Nothing the project tracks changed: commit as ever, with itos commit, and push with itos push.")
		return
	}
	var paths []string
	for _, f := range written {
		paths = append(paths, f.Path)
	}
	// The plugin installed for the project is declared in its settings,
	// which the commit carries.
	if plugin.Action == "installed" && plugin.Scope == "project" {
		paths = append(paths, projectSettings)
	}
	for _, f := range rules.Files {
		if f.Action != "kept" {
			paths = append(paths, f.Path)
		}
	}
	fmt.Fprintf(log, "Commit what init wrote with the task that adopts itos: git add %s, then itos commit --task %s -m "+
		"'chore: adopt itos'\n", strings.Join(paths, " "), adoptionTask)
}

// policyConfig is the config made of the policy's text, and the text:
// commits.since the project's HEAD, or none where it has no commit, no
// footer's since, the starter's ledger layout where the policy has no
// ledger, and the pin where the policy has none, every other byte as the
// policy has it.
func policyConfig(policy, text string, src *config.Loaded, plan policyPlan) (*config.Loaded, string, error) {
	doc, err := value.OpenDoc(text)
	if err != nil {
		return nil, "", policyUnedited(policy, err)
	}
	var edits []error
	for _, name := range src.Commits.Footers.Keys {
		if src.HasSection("commits.footers." + name + ".since") {
			edits = append(edits, doc.Drop([]any{"commits", "footers", name, "since"}))
		}
	}
	at := []any{"commits", "since"}
	switch had := src.HasSection("commits.since"); {
	case had && plan.since != "":
		edits = append(edits, doc.Set(at, plan.since))
	case had && src.File().At("commits").(*value.Map).Len() == 1:
		// commits holds only its since: with it gone, commits goes too,
		// rather than be left holding nothing, which would read as null.
		edits = append(edits, doc.Drop(at[:1]))
	case had:
		edits = append(edits, doc.Drop(at))
	case plan.since != "" && src.HasSection("commits"):
		edits = append(edits, doc.Add(at, plan.since))
	}
	made, err := doc.Text()
	if err := errors.Join(append(edits, err)...); err != nil {
		return nil, "", policyUnedited(policy, err)
	}
	var tail []string
	if plan.since != "" && !src.HasSection("commits") {
		tail = append(tail, "# Where verification starts: this commit and the ones before it, written\n"+
			"# before itos, are never judged.\ncommits:\n  since: "+value.Quote(plan.since)+"\n")
	}
	if !src.HasSection("ledger") {
		tail = append(tail, "# The tasks, as itos init lays them out: one file per phase.\n"+
			"ledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\n")
	}
	if plan.pin != nil && !plan.kept {
		tail = append(tail, fmt.Sprintf("# The itos release this repository runs, the newest when itos init ran:\n"+
			"# itos pin moves it.\npin:\n  version: %s\n  checksums: %s\n", value.Quote(plan.pin[0]), value.Quote(plan.pin[1])))
	}
	nl := "\n"
	if strings.Contains(made, "\r\n") {
		nl = "\r\n"
	}
	if len(tail) > 0 && !strings.HasSuffix(made, "\n") {
		made += nl
	}
	for _, section := range tail {
		made += nl + strings.ReplaceAll(section, "\n", nl)
	}
	cfg, err := config.FromText(plan.file, made, plan.stealth)
	return cfg, made, err
}

// policyUnedited is a policy whose text init --policy cannot edit in place.
func policyUnedited(policy string, err error) error {
	return &config.Error{File: policy, Problems: []out.Problem{{Rule: "init-policy-edit",
		Message: "init --policy cannot make the project's config of it: " + err.Error(),
		Fix:     "write the policy as a block mapping, with no anchor or tag on commits or its footers"}}}
}

// policyData is the fresh ledger and registry at the config's paths, and
// why it cannot hold them: a ledger whose files cannot name the group 1, and
// in a project an adoption task ledger.id or commits.types refuses. Never a
// weaker pattern or a guessed ID in their place.
func policyData(cfg *config.Loaded, stealth bool) ([]policyFile, []out.Problem) {
	var problems []out.Problem
	layout, _ := ledger.LayoutOf(cfg)
	files := filepath.ToSlash(cfg.Ledger.Files)
	first := path.Join(layout.Dir, strings.ReplaceAll(path.Base(files), "{group}", adoptionGroup))
	if !strings.Contains(path.Base(files), "{group}") || !layout.File.MatchString(path.Base(first)) {
		problems = append(problems, out.Problem{Rule: "init-policy-ledger",
			Message: fmt.Sprintf("ledger.files (%s) and ledger.group.pattern (%s) cannot name the file of the group %s, "+
				"the fresh ledger's", cfg.Ledger.Files, cfg.Ledger.Group.Pattern, adoptionGroup),
			Fix: "write {group} in ledger.files' file name, and a ledger.group.pattern that matches " + adoptionGroup})
	}
	if !stealth {
		if pattern := ledger.IDPattern(cfg); !pattern.MatchString(adoptionTask) {
			problems = append(problems, out.Problem{Rule: "init-policy-ledger",
				Message: fmt.Sprintf("ledger.id (%s) does not match %s, the task the project's adoption commit names",
					*cfg.Ledger.ID, adoptionTask),
				Fix: "write a ledger.id that " + adoptionTask + " matches, such as T-\\d+"})
		}
		if types := cfg.Commits.Types; types != nil && !value.Includes(types, adoptionType) {
			problems = append(problems, out.Problem{Rule: "init-policy-ledger",
				Message: fmt.Sprintf("commits.types does not list %s, the type of %s, the adoption task", adoptionType, adoptionTask),
				Fix:     "add " + adoptionType + " to commits.types"})
		}
	}
	data := []policyFile{
		{key: "ledger.files", path: filepath.FromSlash(first), text: starter{stealth: stealth}.ledger()},
		{key: "work.registry", path: cfg.Work.Registry, text: freshRegistry(cfg.Work.GroupsKey)},
	}
	for _, name := range cfg.Tests.Keys {
		if file := cfg.Tests.Values[name].Smoke.File; file != nil {
			data = append(data, policyFile{key: "tests." + name + ".smoke.file", path: *file})
		}
	}
	return data, problems
}

// plainKey is a mapping key YAML reads as itself, written bare.
var plainKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

// freshRegistry is the starter's registry with its groups under the key.
func freshRegistry(key string) string {
	if !plainKey.MatchString(key) {
		key = value.Quote(key)
	}
	return strings.Replace(starterRegistryText, "\nphases:\n", "\n"+key+":\n", 1)
}

// policyPaths are the files init would write outside the root (the
// project's top, or the stealth folder), as written or once the symbolic
// links of the part of their path that is there are followed.
func policyPaths(root string, files []policyFile) []out.Problem {
	var problems []out.Problem
	for _, f := range files {
		if !inside(root, f.path) {
			problems = append(problems, out.Problem{Rule: "init-policy-path",
				Message: fmt.Sprintf("%s puts %s outside %s", f.key, filepath.ToSlash(f.path), rootName(root)),
				Fix:     "point " + f.key + " at a path inside it"})
		}
	}
	return problems
}

// rootName is the root as a message names it.
func rootName(root string) string {
	if root == "." {
		return "the project"
	}
	return filepath.ToSlash(root)
}

// inside is whether p stays in root, both resolved by resolved.
func inside(root, p string) bool {
	r, errRoot := resolved(root)
	q, errPath := resolved(filepath.Dir(p))
	rel, err := filepath.Rel(r, filepath.Join(q, filepath.Base(p)))
	return errRoot == nil && errPath == nil && err == nil && filepath.IsLocal(rel)
}

// resolved is p made absolute, the symbolic links of its deepest part that
// is there followed and the rest joined to it as written. Abs fails only
// where the working folder is gone, and then so does EvalSymlinks.
func resolved(p string) (string, error) {
	abs, _ := filepath.Abs(p)
	rest := ""
	for at := abs; ; at = filepath.Dir(at) {
		if _, err := os.Lstat(at); err == nil || filepath.Dir(at) == at {
			real, err := filepath.EvalSymlinks(at)
			return filepath.Join(real, rest), err
		}
		rest = filepath.Join(filepath.Base(at), rest)
	}
}

// policyThere is what of a project is there already: the config found (or
// the one init would write), every file of the ledger's folder its pattern
// names, and the registry and the smoke sets, an empty file as much as any.
func policyThere(cfg *config.Loaded, file, found string, files []policyFile) *policyRefusal {
	var there []string
	seen := map[string]bool{}
	add := func(p string) {
		p = filepath.ToSlash(filepath.Clean(p))
		if !seen[p] {
			seen[p] = true
			there = append(there, p)
		}
	}
	for _, p := range []string{found, file, "itos.yaml"} {
		if p != "" && lexists(p) {
			add(p)
		}
	}
	layout, _ := ledger.LayoutOf(cfg)
	if entries, err := os.ReadDir(filepath.FromSlash(layout.Dir)); err == nil {
		for _, e := range entries {
			if layout.File.MatchString(e.Name()) {
				add(path.Join(layout.Dir, e.Name()))
			}
		}
	}
	for _, f := range files {
		if lexists(f.path) {
			add(f.path)
		}
	}
	if len(there) == 0 {
		return nil
	}
	refusal := &policyRefusal{lines: there,
		head: "itos: init --policy makes a fresh project and never writes over one, so it wrote nothing; " +
			"these are there already:",
		tail: "Run init --policy where no itos config or data is, or init without --policy to check this project."}
	for _, p := range there {
		refusal.problems = append(refusal.problems, out.Problem{Rule: "init-policy-exists",
			Message: p + " is there already: init --policy makes a fresh project, never over one",
			Fix:     "run init --policy where no itos config or data is, or init without --policy to check this project"})
	}
	return refusal
}

// lexists is whether there is a file, a folder or a symbolic link at p.
func lexists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil || !errors.Is(err, fs.ErrNotExist)
}

// policyRefusal is why init --policy writes nothing of a policy it can use:
// the line that says why, its problems, each as the text lists it, and what
// to do. Exit 1, a policy failure.
type policyRefusal struct {
	head, tail string
	problems   []out.Problem
	lines      []string
}

// policyRefused reports the refusal and writes nothing: exit 1.
func policyRefused(file string, refusal *policyRefusal, o Out) (int, error) {
	if o.JSON {
		return ExitPolicy, out.Emit(o.Stdout, out.Field{Key: "config", Value: filepath.ToSlash(file)},
			out.Field{Key: "action", Value: "refused"}, out.Field{Key: "problems", Value: refusal.problems})
	}
	fmt.Fprintln(o.Stderr, refusal.head)
	for _, line := range refusal.lines {
		fmt.Fprintf(o.Stderr, "  %s\n", strings.ReplaceAll(line, "\n", "\n  "))
	}
	fmt.Fprintln(o.Stderr, refusal.tail)
	return ExitPolicy, nil
}

// policySmokeSets gives each smoke set its text: the first live test of
// each file of the project's own tests, as the starter's names, never the
// policy's selections. A command adapter's list that fails, or whose output
// the protocol refuses, is a refusal with its error.
func policySmokeSets(cfg *config.Loaded, files []policyFile) (*policyRefusal, error) {
	for i, f := range files {
		name, isSmoke := strings.CutPrefix(f.key, "tests.")
		if !isSmoke {
			continue
		}
		name = strings.TrimSuffix(name, ".smoke.file")
		k := cfg.Tests.Values[name]
		list, err := tests.ListTests(cfg, name, "worktree")
		switch {
		case err != nil && k.Adapter.Command != "":
			return policyListRefused(name, err), nil
		case err != nil:
			return nil, err
		}
		files[i].text, _ = smokeSetOf(list, k.TagPrefix)
	}
	return nil, nil
}

// policyListRefused is the refusal of a command adapter's list that failed,
// so the kind's smoke set cannot be derived.
func policyListRefused(name string, err error) *policyRefusal {
	return &policyRefusal{
		head: "itos: init --policy wrote nothing: the list of tests." + name + " failed, so it cannot derive its smoke set:",
		problems: []out.Problem{{Rule: "init-policy-list", Message: err.Error(),
			Fix: "make tests." + name + "'s adapter list the project's tests, then run init --policy again"}},
		lines: []string{err.Error()},
		tail:  "Make tests." + name + "'s adapter list the project's tests, then run init --policy again."}
}

// writeNew writes a file that is not there, its folder made first, and
// fails rather than write over one.
func writeNew(p, text string) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}
	_, err = f.WriteString(text)
	return errors.Join(err, f.Close())
}

// partial is a failure after init --policy began to write: what it did, so
// nothing it did is taken for a project it did not finish, and nothing of
// the person's is removed.
func partial(err error, initialized bool, written []writtenFile) error {
	var did []string
	if initialized {
		did = append(did, "ran git init")
	}
	for _, f := range written {
		did = append(did, "wrote "+f.Path)
	}
	if len(did) == 0 {
		return fmt.Errorf("init --policy did not initialize the project: %w", err)
	}
	return fmt.Errorf("init --policy did not initialize the project: %w; it %s, and left them as they are",
		err, strings.Join(did, ", "))
}
