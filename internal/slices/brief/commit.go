package brief

import (
	"regexp"
	"slices"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/scope"
)

// docs is the type a spec goes in under where the config lets it.
const docs = "docs"

// specCommit is how a drafter commits its spec: the commit Type and the Flag
// of itos commit that goes with it, "" for none, or Stop, no type of the
// config being able to carry it, with the flag it was judged under.
type specCommit struct {
	Type, Flag string
	Stop       bool
}

// command is the itos commit line that writes the flag's footer for the item.
func (c specCommit) command(id string) string {
	if c.Flag == "" {
		return "itos commit"
	}
	return "itos commit " + c.Flag + " " + id
}

// noCommitType is what a drafter is told in place of commitSentences when
// its spec cannot be committed; its command is filled as theirs is.
const noCommitType = "No commit type of this repository's config can carry this spec: none may touch the spec's files and be committed with `" + commitCommand + "`. Stop and report that before writing the spec; do not commit it under another type."

// commitSentences are a drafter template's sentences on the spec's commit and
// its push, on one line.
var commitSentences = regexp.MustCompile("Commit with `.*? Push with `itos push`\\.")

// filesToCommit are the files the spec's commit touches.
func (f facts) filesToCommit() []string {
	if f.unwritten == "" {
		return f.specFiles
	}
	return []string{f.unwritten}
}

// chooseSpecCommit is how the config lets the spec of the item, of the kind
// and in the files, be committed. The flag is the one the config has: --item
// for a footer of the work registry, else --task for a task and a ledger
// footer, else none, as itos commit refuses a flag whose footer the config
// lacks. The type is docs where it may carry the spec, else the first of the
// commit types that may and that the flag's footer is meant for: one the
// registry's footer stands in for, or one the ledger's is required for. A type
// may carry it when it is a commit type, the path rules itos commit check-paths
// judges with let it touch the files, and the hook's footer rules call nothing
// missing from a commit that has the flag's footer and no other. A config with
// no commits section has nothing to judge by, so docs with no flag, files or
// none. An error is a glob the path rules cannot compile.
func chooseSpecCommit(cfg *config.Loaded, id, kind string, files []string) (specCommit, error) {
	if !cfg.HasSection("commits") {
		return specCommit{Type: docs}, nil
	}
	c, meantFor, links := specFlag(cfg, id, kind)
	c.Stop = true
	if len(files) == 0 {
		return c, nil
	}
	// The section is there, the one thing Of refuses.
	rules, _ := scope.Of(cfg)
	types := message.Types(cfg)
	candidates := []string{}
	if slices.Contains(types, docs) {
		candidates = append(candidates, docs)
	}
	for _, typ := range types {
		if meantFor(typ) {
			candidates = append(candidates, typ)
		}
	}
	for _, typ := range candidates {
		found, err := rules.Issues(typ, files)
		if err != nil {
			return specCommit{}, err
		}
		if len(found) == 0 && len(message.Missing(cfg, typ, links, links)) == 0 {
			c.Type, c.Stop = typ, false
			return c, nil
		}
	}
	return c, nil
}

// specFlag is the flag a spec of the item is committed with, whether a type
// other than docs is meant for that flag's footer, and the footers of a commit
// made with it, as message.Missing reads them.
func specFlag(cfg *config.Loaded, id, kind string) (specCommit, func(typ string) bool, string) {
	subject := "specify " + id
	trailed := func(key string) string {
		return message.Trailed(subject, message.FooterLines(key, []string{id}), cfg.CommentChar())
	}
	if key, ok := message.RegistryFooter(cfg); ok {
		return specCommit{Flag: "--item"}, func(typ string) bool {
			return slices.ContainsFunc(cfg.Commits.Footers.Keys, func(other string) bool {
				return slices.Contains(message.StandIns(cfg, other, typ), key)
			})
		}, trailed(key)
	}
	if key, ok := message.LedgerFooter(cfg); ok && kind == "task" {
		return specCommit{Flag: "--task"}, func(typ string) bool {
			return slices.Contains(message.Missing(cfg, typ, "", ""), key)
		}, trailed(key)
	}
	return specCommit{}, func(string) bool { return false }, subject
}
