// Package scope is the path rules of commits.scopes: which paths each commit
// type may touch. only: every touched path must match; never: no touched
// path may match, unless it matches except, which takes paths back out of
// never alone; must_touch: at least one must. Its $sets are already expanded
// by the config's loader.
//
// Of takes the loaded config and is a config error without a commits
// section. Its Rules give Ruled (whether a type has rules, even empty ones:
// the commit-msg hook runs neither the paths nor the staged range checks for
// one without), Issues (a type and its paths to the problems, with the rule
// ids and fixes naming the types that would take a path, from TypesFor) and
// Reject (the rejection as the hook prints it). So itos commit check-paths,
// the commit-msg hook (on the staged paths) and verify (on each commit's)
// share one judgement, and none of them goes through the command line to
// reach it.
package scope
