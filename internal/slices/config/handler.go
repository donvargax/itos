package config

import (
	"context"
	"strings"

	itosconfig "github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/configcheck"
	"github.com/donvargax/itos/v8/internal/out"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/tests"
	"github.com/donvargax/itos/v8/internal/value"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Query is what the config command asks: Sub, check or get; check's
// Defaults, under --print-defaults, and its Ledger, the one ledger file
// --ledger names, "" for every one; get's Key.
type Query struct {
	Sub      string
	Defaults bool
	Ledger   string
	Key      string
}

// Ports are what the config command reaches the world through, which
// internal/cli binds: the repository, which locates the stealth config;
// Trees, the trees itos reads its data from, and Worktree, the working
// tree, where the config and its data are read; and TaskFlags, the flags of
// itos task, its subcommands' and the global ones, that name no group, which
// internal/cli's flag spec holds.
type Ports interface {
	vcsport.VCS
	Trees(ctx context.Context) treeport.Trees
	Worktree(ctx context.Context) treeport.Source
	TaskFlags(ctx context.Context) []string
}

// Handler is the config command's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Out is what Handle is told of how its answer is printed: JSON, which
// check's answer names the config's path in; and Tests, where the test
// listings warn.
type Out struct {
	JSON  bool
	Tests *tests.Warnings
}

// Answer is what Handle found: check's Code, its Found problems, its
// Warnings, the Lines it prints when it finds none and, under JSON, the
// config's Path; --print-defaults' Defaults; get's Value.
type Answer struct {
	Code            int
	Found, Warnings []configcheck.Found
	Lines           []string
	Path            string
	Defaults        *value.Map
	Value           any
}

// Handle answers q.
func (h Handler) Handle(ctx context.Context, q Query, o Out) (Answer, error) {
	switch {
	case q.Sub == "get":
		return h.get(ctx, q.Key)
	case q.Defaults:
		return h.defaults(ctx), nil
	}
	return h.check(ctx, q.Ledger, o)
}

// path is the config's path, as the working tree has it.
func (h Handler) path(ctx context.Context) string {
	return itosconfig.Path(h.ports, h.ports.Worktree(ctx))
}

// load is the config at file, it and its data read from the working tree.
func (h Handler) load(ctx context.Context, file string) (*itosconfig.Loaded, error) {
	return itosconfig.Load(h.ports, h.ports.Trees(ctx), h.ports.Worktree(ctx), file)
}

// check is `config check [--ledger <file>]`: the config, then the
// ledger, the work registry, each kind's smoke set and the decision records, every problem with its
// rule id and, where one exists, a fix; then its warnings, printed as WARN
// and never failing it.
func (h Handler) check(ctx context.Context, ledgerFile string, o Out) (Answer, error) {
	code, found, warnings, lines, err := configcheck.Findings(ctx, h.ports, h.ports.Trees(ctx), h.ports.Worktree(ctx), ledgerFile, h.ports.TaskFlags(ctx), o.Tests)
	if err != nil {
		return Answer{}, err
	}
	a := Answer{Code: code, Found: found, Warnings: warnings, Lines: lines}
	if o.JSON {
		a.Path = h.path(ctx)
	}
	return a, nil
}

// defaults is `config check --print-defaults`: the one table of
// defaults the loader applies, as YAML or JSON, as it applies to this
// config's ledger and to a stealth config. It checks nothing, so a config that does not load gets the
// table's own values.
func (h Handler) defaults(ctx context.Context) Answer {
	path := h.path(ctx)
	var file *value.Map
	if cfg, err := h.load(ctx, path); err == nil {
		file = cfg.File()
	}
	return Answer{Defaults: itosconfig.DefaultsFor(file, itosconfig.IsStealth(h.ports, path))}
}

// get is `config get <key>`: the value at a dotted key path as the
// tools read it, the config's own or its default, from the config every
// command finds. A key the schema does not have is a usage error, before
// any config is read; a config that cannot be loaded fails as it does for
// every command.
func (h Handler) get(ctx context.Context, key string) (Answer, error) {
	if known, keys := itosconfig.KnownKey(key); !known {
		if fix, ok := itosconfig.RemovedKey(key); ok {
			return Answer{}, out.Usage("config get: %s was removed in v5; %s", key, fix)
		}
		if len(keys) > 0 {
			return Answer{}, out.Usage("config get: unknown key %s; the keys there are %s", key, strings.Join(keys, ", "))
		}
		return Answer{}, out.Usage("config get: unknown key %s", key)
	}
	cfg, err := h.load(ctx, h.path(ctx))
	if err != nil {
		return Answer{}, err
	}
	return Answer{Value: cfg.Get(key)}, nil
}
