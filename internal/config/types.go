package config

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Config is the config as the tools read it, typed: the file laid over the
// table of defaults (Load), or the file alone (the cross-checks). A key with
// a default always has a value; one without is a nil pointer, a nil list or
// an empty Ordered when the file leaves it out.
type Config struct {
	Version  float64  `json:"version"`
	Requires *string  `json:"requires"`
	Pin      *Pin     `json:"pin"`
	Shell    []string `json:"shell"`
	Ledger   Ledger   `json:"ledger"`
	Commits  Commits  `json:"commits"`
	// Tests are the kinds of named tests, each with the per-kind defaults
	// under it.
	Tests Ordered[Kind] `json:"tests"`
	CI    CI            `json:"ci"`
	Work  Work          `json:"work"`
	Hooks Hooks         `json:"hooks"`
}

// Pin is the one release a repository runs: its version and the SHA-256 of
// its checksums.txt (internal/launch).
type Pin struct {
	Version   string `json:"version"`
	Checksums string `json:"checksums"`
}

// Ledger is the ledger's layout: its files, the group in their names, the ID
// pattern and the checks' timeout.
type Ledger struct {
	Files string `json:"files"`
	Group struct {
		Label   string `json:"label"`
		Pattern string `json:"pattern"`
		Numeric bool   `json:"numeric"`
	} `json:"group"`
	ID    *string `json:"id"`
	Check struct {
		Timeout float64 `json:"timeout"`
	} `json:"check"`
}

// Commits are the commit rules.
type Commits struct {
	Types         []string          `json:"types"`
	HeaderLint    HeaderLint        `json:"header_lint"`
	Footers       Ordered[Footer]   `json:"footers"`
	PathSets      Ordered[[]string] `json:"path_sets"`
	Scopes        Ordered[Scope]    `json:"scopes"`
	RejectMessage string            `json:"reject_message"`
	// The commit where verification starts: verify and the range checks
	// leave it and its ancestors out.
	Since *string `json:"since"`
}

// HeaderLint is commits.header_lint: itos's own lint (Use builtin), or the
// delegate, the message file through Hook and a message on stdin through
// Stdin (Use command, or no Use).
type HeaderLint struct {
	Use   *string `json:"use"`
	Hook  *string `json:"hook"`
	Stdin *string `json:"stdin"`
}

// Builtin is whether itos lints the header itself rather than through a
// delegate.
func (h HeaderLint) Builtin() bool { return h.Use != nil && *h.Use == "builtin" }

// Footer is one footer of commits.footers.
type Footer struct {
	Source      FooterSource `json:"source"`
	StripPrefix *string      `json:"strip_prefix"`
	RequiredFor *Types       `json:"required_for"`
	ValidateFor *Types       `json:"validate_for"`
	MustBeLive  *bool        `json:"must_be_live"`
	ReadAt      *string      `json:"read_at"`
	// The commit after which the footer is required: verify leaves it and its
	// ancestors out of required_for, so a footer added to a project's rules
	// does not fail the history written before it.
	Since *string `json:"since"`
}

// Text is whether the footer is free text (source: text) rather than IDs.
func (f Footer) Text() bool { return f.Source.IsName && f.Source.Name == "text" }

// Live is whether every ID the footer names must be live (must_be_live).
func (f Footer) Live() bool { return f.MustBeLive != nil && *f.MustBeLive }

// FooterSource is a footer's source: the ledger, a kind of tests, or free
// text. Name is the source as written when it is a word (IsName: ledger,
// text, or any other word the cross-checks refuse); Tests is the kind when
// it is { tests: <kind> }.
type FooterSource struct {
	Name   string
	IsName bool
	Tests  string
}

func (s *FooterSource) UnmarshalJSON(data []byte) error {
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte(`"`)) {
		s.IsName = true
		return json.Unmarshal(data, &s.Name)
	}
	var kind struct {
		Tests string `json:"tests"`
	}
	err := json.Unmarshal(data, &kind)
	s.Tests = kind.Tests
	return err
}

// Types are a footer's required_for or validate_for: a list of commit types,
// or one word (all, or another the cross-checks refuse).
type Types struct {
	Word string
	List []string
	// IsList is whether the value is a list rather than a word.
	IsList bool
}

func (t *Types) UnmarshalJSON(data []byte) error {
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte(`"`)) {
		return json.Unmarshal(data, &t.Word)
	}
	t.IsList = true
	return json.Unmarshal(data, &t.List)
}

// Scope is one commit type's path rules.
type Scope struct {
	Only      []string `json:"only"`
	Never     []string `json:"never"`
	MustTouch []string `json:"must_touch"`
}

// Kind is one kind of named tests, its defaults under it.
type Kind struct {
	Adapter   Adapter `json:"adapter"`
	Root      *string `json:"root"`
	ID        *string `json:"id"`
	TagPrefix string  `json:"tag_prefix"`
	WipTag    string  `json:"wip_tag"`
	Run       struct {
		Whole      *string `json:"whole"`
		Select     *string `json:"select"`
		IDsPattern *string `json:"ids_pattern"`
		Join       struct {
			Each string `json:"each"`
			Sep  string `json:"sep"`
		} `json:"join"`
	} `json:"run"`
	Recognize []Recognize `json:"recognize"`
	Smoke     struct {
		File      *string `json:"file"`
		EveryFile bool    `json:"every_file"`
		AddHint   *string `json:"add_hint"`
	} `json:"smoke"`
	RangeChecks []RangeCheck `json:"range_checks"`
}

// Adapter is a kind's adapter: built in by Name (gherkin), or a Command that
// speaks the adapter protocol. Given is whether the file gives one (in the
// file alone; the loaded config has the default).
type Adapter struct {
	Name       string
	Command    string
	SupportsAt *bool
	Given      bool
}

func (a *Adapter) UnmarshalJSON(data []byte) error {
	a.Given = true
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte(`"`)) {
		return json.Unmarshal(data, &a.Name)
	}
	var command struct {
		Command    string `json:"command"`
		SupportsAt *bool  `json:"supports_at"`
	}
	err := json.Unmarshal(data, &command)
	a.Command, a.SupportsAt = command.Command, command.SupportsAt
	return err
}

// Recognize is a template that reads a task check back as a selection.
type Recognize struct {
	Command string `json:"command"`
	As      string `json:"as"`
}

// RangeCheck is a kind's rule on how its tests may change between two trees:
// commands run on the staged tree and over a range, or the built-in moves
// rule.
type RangeCheck struct {
	Name           string            `json:"name"`
	ExceptTypes    []string          `json:"except_types"`
	Staged         *string           `json:"staged"`
	Range          *string           `json:"range"`
	Builtin        *string           `json:"builtin"`
	AllowedRenames map[string]string `json:"allowed_renames"`
}

// CI is CI's plan: its steps, the prose shortcut, the cost patterns, what
// covers a check, the nightly and the range provider.
type CI struct {
	Env   map[string]string `json:"env"`
	Steps []Step            `json:"steps"`
	Prose *struct {
		Paths []string `json:"paths"`
		Steps []string `json:"steps"`
	} `json:"prose"`
	Cost struct {
		Static           []string `json:"static"`
		KeepWrittenOrder bool     `json:"keep_written_order"`
	} `json:"cost"`
	Covers      []Cover  `json:"covers"`
	NightlyOnly []string `json:"nightly_only"`
	Nightly     *struct {
		Steps []Step `json:"steps"`
	} `json:"nightly"`
	WaitOnStatus       []string `json:"wait_on_status"`
	StopAtFirstFailure bool     `json:"stop_at_first_failure"`
	Range              Range    `json:"range"`
}

// Step is one CI step: a command written as text, or a mapping that runs a
// command, a kind's tests or (in the nightly) the done tasks' checks. JSON is
// the step as JSON.stringify writes it, for the messages that quote it.
type Step struct {
	Text  *string `json:"-"`
	Run   *string `json:"run"`
	Tests *string `json:"tests"`
	Whole bool    `json:"whole"`
	Cost  *string `json:"cost"`
	Tasks *string `json:"tasks"`
	JSON  string  `json:"-"`
}

func (s *Step) UnmarshalJSON(data []byte) error {
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte(`"`)) {
		return json.Unmarshal(data, &s.Text)
	}
	type plain Step
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*s = Step(p)
	s.JSON = string(data)
	return nil
}

// Cover is a ci.covers rule: a check matching Matches is not run again after
// the step By.
type Cover struct {
	By      string `json:"by"`
	Matches string `json:"matches"`
}

// Range is where a push's range starts.
type Range struct {
	Provider string  `json:"provider"`
	Command  *string `json:"command"`
	GitHub   struct {
		Workflow      string   `json:"workflow"`
		Branch        string   `json:"branch"`
		RepositoryEnv string   `json:"repository_env"`
		TokenEnv      []string `json:"token_env"`
	} `json:"github"`
}

// Work is the work registry, its statuses, the people and the identity.
type Work struct {
	Registry  string   `json:"registry"`
	GroupsKey string   `json:"groups_key"`
	Statuses  []string `json:"statuses"`
	People    People   `json:"people"`
	Identity  struct {
		Provider string  `json:"provider"`
		Command  *string `json:"command"`
		Hint     string  `json:"hint"`
	} `json:"identity"`
}

// People is who may own work: the source and the file it reads.
type People struct {
	Source    string  `json:"source"`
	File      string  `json:"file"`
	LoginFrom *string `json:"login_from"`
}

// Hooks are the hook manager, the binary the shims call, the pre-push
// commands and the commit-msg hook's task checks.
type Hooks struct {
	Manager *string `json:"manager"`
	Bin     string  `json:"bin"`
	PrePush *struct {
		PerBase string `json:"per_base"`
		Whole   string `json:"whole"`
	} `json:"pre_push"`
	CommitMsg struct {
		TaskChecks   bool     `json:"task_checks"`
		CheckTimeout *float64 `json:"check_timeout"`
	} `json:"commit_msg"`
}

// Ordered is a mapping whose order matters, as written (JavaScript's order).
type Ordered[T any] struct {
	Keys   []string
	Values map[string]T
}

// Get is a key's value, and whether the mapping has it.
func (o Ordered[T]) Get(key string) (T, bool) {
	v, ok := o.Values[key]
	return v, ok
}

func (o *Ordered[T]) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return fmt.Errorf("not a mapping: %s", data)
	}
	o.Values = map[string]T{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		key := t.(string)
		var v T
		if err := dec.Decode(&v); err != nil {
			return err
		}
		o.Keys = append(o.Keys, key)
		o.Values[key] = v
	}
	return nil
}
