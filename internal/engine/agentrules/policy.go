package agentrules

import (
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
)

// Code is a value from the config as a Markdown code span, on one line (a
// line break made a space), fenced with more backticks than it holds.
func Code(s string) string {
	s = strings.TrimSpace(strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s))
	fence := "`"
	for strings.Contains(s, fence) {
		fence += "`"
	}
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		return fence + " " + s + " " + fence
	}
	return fence + s + fence
}

// codes is values as code spans, joined as a list in prose: a, b and c (or
// "or" for join).
func codes(values []string, join string) string {
	spans := make([]string, len(values))
	for i, v := range values {
		spans[i] = Code(v)
	}
	return Prose(spans, join)
}

// Prose is words joined as a list in prose: a, b and c (or "or" for join).
func Prose(words []string, join string) string {
	switch len(words) {
	case 0:
		return ""
	case 1:
		return words[0]
	}
	return strings.Join(words[:len(words)-1], ", ") + " " + join + " " + words[len(words)-1]
}

// typesOf are the commit types a footer's required_for or in_place_of
// names: its list, every type for all, none for nil.
func typesOf(cfg *config.Loaded, t *config.Types) []string {
	switch {
	case t == nil:
		return nil
	case t.IsList:
		return t.List
	case t.Word == "all":
		return cfg.Commits.Types
	}
	return nil
}

// CommitPolicy are the sections on commits the config sets, as itos guide
// commit prints them (slice 119): the commit types, the footers each type
// needs and the paths each may touch; data is the path of a file of itos's
// own data the config names, in Markdown.
func CommitPolicy(cfg *config.Loaded, data func(string) string) []string {
	var paras []string
	add := func(lines ...string) { paras = append(paras, strings.Join(lines, "\n")) }
	c := cfg.Commits
	add("### Commit types")
	header := ""
	switch {
	case c.HeaderLint.Builtin():
		header = "The header is held to commitlint's config-conventional rules by itos's own lint: " +
			Code("type: subject") + " or " + Code("type(scope): subject") + "."
	case c.HeaderLint.Hook != nil:
		header = "The header is linted by " + Code(*c.HeaderLint.Hook) + "."
	case c.HeaderLint.Stdin != nil:
		header = "The header is linted by " + Code(*c.HeaderLint.Stdin) + "."
	}
	if len(c.Types) > 0 {
		add(strings.TrimSpace("A commit's type is one of " + codes(c.Types, "or") + ". " + header))
	} else if header != "" {
		add(header)
	}

	if len(c.Footers.Keys) > 0 {
		add("### Footers")
		var items []string
		for _, key := range c.Footers.Keys {
			f := c.Footers.Values[key]
			what := "is free text"
			switch {
			case f.Source.IsName && f.Source.Name == "ledger":
				what = "names tasks of the ledger, " + data(cfg.Ledger.Files)
			case f.Registry():
				what = "names items of the work registry, " + data(cfg.Work.Registry)
			case f.Source.Tests != "":
				what = "names " + Code(f.Source.Tests) + " tests by their IDs"
				if f.Live() {
					wip := "@wip"
					if k, ok := cfg.Tests.Get(f.Source.Tests); ok && k.WipTag != "" {
						wip = k.WipTag
					}
					what += ", live ones only (not " + Code(wip) + ")"
				}
			}
			items = append(items, "- "+Code(key+":")+" "+what+".")
		}
		add(items...)
		add("The footers each type needs:")
		items = nil
		for _, typ := range c.Types {
			var needs []string
			for _, key := range c.Footers.Keys {
				if !slices.Contains(typesOf(cfg, c.Footers.Values[key].RequiredFor), typ) {
					continue
				}
				alternatives := []string{key + ":"}
				for _, other := range c.Footers.Keys {
					in, ok := c.Footers.Values[other].InPlaceOf.Get(key)
					if ok && slices.Contains(typesOf(cfg, &in), typ) {
						alternatives = append(alternatives, other+":")
					}
				}
				needs = append(needs, codes(alternatives, "or"))
			}
			need := "none"
			if len(needs) > 0 {
				need = Prose(needs, "and")
			}
			items = append(items, "- "+Code(typ)+": "+need+".")
		}
		if len(items) > 0 {
			add(items...)
		}
	}

	add("### The paths each type may touch")
	var items []string
	for _, typ := range c.Scopes.Keys {
		s := c.Scopes.Values[typ]
		var rules []string
		if len(s.Only) > 0 {
			rules = append(rules, "may touch only "+codes(s.Only, "and"))
		}
		if len(s.Never) > 0 {
			never := "may not touch " + codes(s.Never, "or")
			if len(s.Except) > 0 {
				never += ", except " + codes(s.Except, "or")
			}
			rules = append(rules, never)
		}
		if len(s.MustTouch) > 0 {
			rules = append(rules, "must touch at least one of "+codes(s.MustTouch, "or"))
		}
		if len(rules) == 0 {
			rules = []string{"may touch anything"}
		}
		items = append(items, "- "+Code(typ)+" "+strings.Join(rules, "; it ")+".")
	}
	var free []string
	for _, typ := range c.Types {
		if _, ok := c.Scopes.Get(typ); !ok {
			free = append(free, typ)
		}
	}
	switch {
	case len(items) == 0:
		add("Every type may touch anything.")
	case len(free) > 0:
		items = append(items, "- "+codes(free, "and")+" may touch anything.")
	}
	if len(items) > 0 {
		add(items...)
	}
	return paras
}
