package initcmd

import (
	"context"
	"fmt"
	"io"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/configcheck"
	"github.com/donvargax/itos/v8/internal/engine/hooks"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/version"
)

// report is init where a config is there: it keeps the config as it is
// and lists what is missing, exit 1 when anything is; then what it notes, never
// counted as missing (slice 50): a people file the config names that the
// repository lacks or cannot be read (config check's warning), and a pin
// behind the newest release, where the release server answers; then makes
// the plugin's offer and the git shim's, which never ask, and install only
// for --plugin and --git-shim; then the offer of the rules for agents, which
// reports a block other than the one this itos writes and writes one only
// for --agent-rules. None of them not in place is counted as missing.
func (h Handler) report(ctx context.Context, file string, offer Offer, o Out) (int, error) {
	_, found, notes, _, err := configcheck.Findings(ctx, h.ports, h.ports.Trees(ctx), h.ports.Worktree(ctx), "",
		h.ports.TaskFlags(ctx), o.Tests)
	if err != nil {
		return 0, err
	}
	if cfg, err := h.loadConfigFile(ctx, file); err == nil {
		found = append(found, configcheck.Tagged("hooks", h.hookProblems(ctx))...)
		if behind := h.pinBehind(ctx, cfg); behind != nil {
			notes = append(notes, configcheck.Found{Problem: *behind, Area: "pin"})
		}
	}
	code := 0
	if len(found) > 0 {
		code = exitPolicy
	}
	if o.JSON {
		if notes == nil {
			notes = []configcheck.Found{}
		}
		offered := h.makeOffers(ctx, offer, io.Discard)
		return max(code, offered.Code), out.Emit(o.Stdout, out.Field{Key: "config", Value: file},
			out.Field{Key: "action", Value: "checked"}, out.Field{Key: "missing", Value: found},
			out.Field{Key: "plugin", Value: offered.Plugin}, out.Field{Key: "notes", Value: notes},
			out.Field{Key: "git_shim", Value: offered.Shim}, out.Field{Key: "agent_rules", Value: offered.Rules})
	}
	list := func(found []configcheck.Found) {
		for _, f := range found {
			line := f.Message
			if f.Fix != "" {
				line += "; " + f.Fix
			}
			fmt.Fprintf(o.Stdout, "  %s\n", line)
		}
	}
	fmt.Fprintf(o.Stdout, "%s is there already; init kept it as it is.\n", file)
	if len(found) == 0 {
		fmt.Fprintln(o.Stdout, "Nothing is missing: the config, its ledger, registry and smoke sets are sound, and the hooks call itos.")
	} else {
		fmt.Fprintln(o.Stdout, "Missing:")
		list(found)
	}
	if len(notes) > 0 {
		fmt.Fprintln(o.Stdout, "Noted, not counted as missing:")
		list(notes)
	}
	offered := h.makeOffers(ctx, offer, o.Stdout)
	return max(code, offered.Code), nil
}

// pinBehind is the note of a pin behind the newest release, nil when the
// config pins none, pins the newest or one newer, or the release server
// cannot say which is the newest.
func (h Handler) pinBehind(ctx context.Context, cfg *config.Loaded) *out.Problem {
	if cfg.Pin == nil || cfg.Pin.Version == "" {
		return nil
	}
	newest, _, err := h.ports.Release(ctx, "")
	if err != nil || version.Compare(cfg.Pin.Version, newest) >= 0 {
		return nil
	}
	return &out.Problem{
		Rule:    "pin-behind",
		Message: fmt.Sprintf("the pin, itos %s, is behind the newest release, itos %s", cfg.Pin.Version, newest),
		Fix:     "itos upgrade moves it there, applying the config changes each release between them declares and naming what is left by hand",
	}
}

// hookProblems are itos's hooks git would not run, as hook install declares
// them in the git config (slice 91): a git that runs no hook its config
// declares, which no entry can mend, else each event whose entry is missing
// or does not run itos.
func (h Handler) hookProblems(ctx context.Context) []out.Problem {
	var problems []out.Problem
	missing := func(event, message, fix string) {
		problems = append(problems, out.Problem{Rule: "hook-missing", Message: "the " + event + " hook: " + message, Fix: fix})
	}
	if !h.ports.ConfigHooksRun(ctx) {
		missing("commit-msg", "git "+h.ports.Version(ctx)+" runs no hook its config declares",
			"upgrade git to "+hooks.Since+" or later, then run itos hook install")
		return problems
	}
	for _, event := range hooks.Events {
		if !hooks.Declared(ctx, h.ports, event) {
			missing(event, "hook."+hooks.Entry(event)+" in the git config does not run itos", "run itos hook install")
		}
	}
	return problems
}
