package config

import (
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/value"
)

// Evidence is the declaration that opts a check in to evidence records (ADR-0060):
// a done_when check or a run: entry of ci.steps that has one is looked up by
// the tree it runs on and recorded; one that has none runs as it always did.
// Presence is the opt-in, so an empty declaration is one.
//
//   - Inputs are paths from the repository's top. A folder covers every file
//     under it. A path git tracks is the index's like the rest of the tree;
//     one it does not is hashed by its content and copied into the checkout the
//     check runs in.
//   - Env are the names of environment variables, hashed by value.
//   - Tools are programs resolved on the check's PATH and hashed by content.
//   - Head is whether the check reads HEAD or its history: HEAD's commit, or
//     that HEAD is unborn, is then in the key beside the tree. A check that
//     leaves it out is keyed by the tree alone, so that a record made at the
//     commit's parent answers once the commit is made.
type Evidence struct {
	Inputs []string `json:"inputs"`
	Env    []string `json:"env"`
	Tools  []string `json:"tools"`
	Head   bool     `json:"head,omitempty"`
}

// EvidenceKeys are the keys a declaration has, in the order the schema
// writes them.
var EvidenceKeys = []string{"inputs", "env", "tools", "head"}

// Normal is the declaration with each list sorted, without a name twice and
// never nil, so that the order a check writes them in does not make it
// another check.
func (e Evidence) Normal() Evidence {
	each := func(list []string) []string {
		return append([]string{}, slices.Compact(slices.Sorted(slices.Values(list)))...)
	}
	return Evidence{Inputs: each(e.Inputs), Env: each(e.Env), Tools: each(e.Tools), Head: e.Head}
}

// Text is the normal declaration as JSON, which check.Key binds.
func (e Evidence) Text() string {
	text, _ := json.Marshal(e.Normal())
	return string(text)
}

// IsGlob is whether a declared path holds a character a glob is written
// with, which no declaration takes: a folder is how a set of files is
// declared.
func IsGlob(p string) bool { return strings.ContainsAny(p, "*?[") }

// IsRepoPath is whether a declared input is a path from the repository's
// top, written with /: not empty, not absolute and not leaving the
// repository once cleaned. Whatever reads or copies an input holds it to
// this, since a ledger is read without config check.
func IsRepoPath(p string) bool {
	clean := path.Clean(p)
	return p != "" && !path.IsAbs(p) && clean != ".." && !strings.HasPrefix(clean, "../") && !strings.Contains(p, `\`)
}

// EvidenceOf reads a declaration as the ledger writes it: a mapping, whose
// lists keep the texts in them. Anything else is no declaration, nil, which
// config check has already refused.
func EvidenceOf(raw any) *Evidence {
	m, ok := raw.(*value.Map)
	if !ok {
		return nil
	}
	texts := func(key string) []string {
		list, _ := m.At(key).([]any)
		var got []string
		for _, item := range list {
			if s, ok := item.(string); ok {
				got = append(got, s)
			}
		}
		return got
	}
	return &Evidence{Inputs: texts("inputs"), Env: texts("env"), Tools: texts("tools"), Head: m.At("head") == true}
}

// EvidenceShape is what is wrong with a declaration as a ledger check writes
// it: not a mapping, a key it lacks, a list that is not texts. where names
// the check.
func EvidenceShape(raw any, where, rule string) []out.Problem {
	m, ok := raw.(*value.Map)
	if !ok {
		return []out.Problem{{Rule: rule, Message: where + "'s evidence is not a mapping",
			Fix: "write evidence: {} to opt in with nothing declared, or evidence: { inputs: […], env: […], tools: […], head: true }"}}
	}
	var found []out.Problem
	for _, k := range m.Keys() {
		if !slices.Contains(EvidenceKeys, k) {
			found = append(found, out.Problem{Rule: rule, Message: where + "'s evidence has an unknown key " + k,
				Fix: "remove " + k + "; evidence's keys are " + strings.Join(EvidenceKeys, ", ")})
			continue
		}
		if k == "head" {
			if value.TypeOf(m.At(k)) != "boolean" {
				found = append(found, out.Problem{Rule: rule, Message: fmt.Sprintf("%s's evidence has head: %s; head is true or false", where, value.JSON(m.At(k))),
					Fix: "write head: true or head: false"})
			}
			continue
		}
		list, isList := m.At(k).([]any)
		for _, item := range list {
			if _, isText := item.(string); !isText {
				isList = false
			}
		}
		if !isList {
			found = append(found, out.Problem{Rule: rule, Message: where + "'s evidence " + k + " is not a list of texts",
				Fix: "write " + k + " as a list of texts"})
		}
	}
	return found
}

// Problems are what is wrong with the texts of a declaration: a path that is
// a glob, is absolute or leaves the repository, a name that is empty or an
// environment variable written with its value.
func (e Evidence) Problems(where, rule string) []out.Problem {
	var found []out.Problem
	problem := func(message, fix string) {
		found = append(found, out.Problem{Rule: rule, Message: where + "'s evidence " + message, Fix: fix})
	}
	for _, p := range e.Inputs {
		switch {
		case IsGlob(p):
			problem(fmt.Sprintf("input %s is a glob", p), "declare the folder that holds the files, or each file by its path")
		case !IsRepoPath(p):
			problem(fmt.Sprintf("input %q is not a path from the repository's top", p),
				"write the path from the repository's top, with / between its parts")
		}
	}
	for _, name := range e.Env {
		if name == "" || strings.Contains(name, "=") {
			problem(fmt.Sprintf("env %q is not the name of a variable", name), "write the variable's name alone, as it is spelled")
		}
	}
	for _, name := range e.Tools {
		if name == "" {
			problem("tool is empty", "write the program's name, as the check runs it")
		}
	}
	return found
}
