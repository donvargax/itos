// Package message is one commit message through itos's commit rules: the
// footer rules, the one footer reader, and the header lint beside them,
// built in (header.go) or a delegate. The footer rules are itos's and run
// always, after the header lint, whatever it is: the header lint judges the
// header and body, and one without the footer rules never skips them. Both
// report before the exit, so a header problem does not hide a footer one.
//
// # Footers
//
// A footer is `<Key>: <id> <id>, …` at the start of a line, and may repeat
// over several lines to stay within the line length limit (FooterLines packs
// IDs so, SplitIDs reads them). Per key of commits.footers: its source (the
// ledger, the work registry's items, a kind's named tests as its adapter
// lists them, or text), strip_prefix, required_for, validate_for,
// must_be_live, in_place_of (the footers it stands in for, and for which
// types: a type that requires one of them is satisfied by this one
// instead), and read_at: commit reads the IDs that exist at the commit being
// checked (the staged tree by default, the commit Reading.At names), so a
// later commit that sets a test back to wip or drops a task does not fail an
// older one, but for the stealth config's ledger and registry, in no commit,
// read in the working tree; worktree, or none, the working tree; since, a
// commit that verify leaves out of required_for with its ancestors
// (Reading.Made, config.Before). Nothing reads a footer by its key: CI's
// tasks and the commit-msg hook's are those of every footer whose source is
// the ledger (LedgerFooter), a kind's named tests those of every footer
// whose source is that kind (TestsFooter), so Task: and Scenarios: are only
// this repository's names for them.
//
// A footer whose source is text carries no IDs: `<Key>: <text>`, each line
// one footer, the text whatever a consumer of the project must do, or the
// word none (None is the one reading of it). Its rule is only that it is
// there (required_for) and not empty (validate_for); Texts reads it, a line
// each, trimmed. CI's plan reads no text footer; Gathered gives a range's
// texts with their commits (git log --no-merges --reverse over from..to),
// which itos commit footers prints for a release's notes.
//
// FooterProblems(cfg, message, Reading) runs each footer's rule in the
// config's order, a read_at: commit footer at Reading.At and the others at
// the working tree, a commit that predates the source read against the
// working tree with its warning on Reading.Warn. Check is commit
// check-message: the delegate (commits.header_lint.stdin) through
// internal/shell with the message on stdin and ITOS_AT in its environment,
// its report printed as it comes or read by ParseReport into leveled
// problems under --json, then the footer rules always. LintFile is the
// commit-msg hook's, with the hook delegate, its footer rules reading the
// message Cleaned: the lines the header lint keeps, so a blank or comment
// first line does not hide the type (bug 36). Verify reads each commit
// through FooterProblems with At set to it and Made set.
//
// IDsIn reads a range's links for CI's plan (git log --format=%B from..to,
// newest first); History logs every commit of a revision with its links
// (Links: each footer whose source is the ledger or a kind of named tests,
// its strip_prefix taken off), for itos work show.
//
// # The stealth mode's notes
//
// Under a stealth config a commit's links, its footers of IDs, are not in
// its message but in a git note in refs/notes/itos (NotesRef, notes.go): a
// footer of free text is content, not a link, and stays in the message.
// Reading.Note carries a commit's links and FooterProblems reads them there,
// its type and its free text still the message's; a link typed into the
// message is refused (typedFooter), since it would show to everyone, and a
// missing one's sentence ends with the itos commit flag that writes it
// (stealthNeed, stealthFix). Who fills Note: the commit-msg hook, from
// ITOS_FOOTERS (FootersEnv, which itos commit sets) or, for an amend, HEAD's
// note, which the rewrite carries to the new commit; check-message, from
// ITOS_FOOTERS; verify, from Note of each commit. The range readers log %N
// with --no-notes --notes=refs/notes/itos in place of %B.
//
// # The header lint
//
// Under commits.header_lint.use: builtin the built-in lint (HeaderProblems)
// runs in the delegate's place in all three: it is commitlint 21 with
// config-conventional, read from those packages. header.go is
// conventional-commits-parser with the conventionalcommits preset's patterns
// (the breaking-change header tried first, the body ending at the first
// footer-token line, a breaking-change note running to the next one) and
// config-conventional's rules in its order, each a judgement that gives
// commitlint's words or none; ignore.go is is-ignored's wildcards (merges,
// reverts, reapplies, fixups, versions by semver's strict pattern); cases.go
// is subject-case and type-case as @commitlint/ensure and es-toolkit find a
// case, its word pattern written out with the backtracking each alternative
// needs, JavaScript's case mappings where Go's differ, and lengths in UTF-16
// code units. Patterns use JavaScript's dot and \s (value.Space) and spell
// /i out on ASCII, since RE2's differ. The hook passes the message file with
// a line break added and git's comment character (CommentChar,
// core.commentChar or #), whose lines and scissors it leaves out, as
// commitlint --edit does; check-message and verify pass the message as
// given, as commitlint reads stdin. A blank message has no problems in the
// hook's reading, where git aborts the empty commit itself, and on stdin
// names type-empty and subject-empty. PrintLeveled prints the problems as
// commitlint prints them, errors before warnings, then the footer problems,
// or one leveled list under --json; a warning alone passes. Its one known
// difference is wording: es-toolkit's deburr decomposes every precomposed
// letter, and cases.go only those of Latin-1 and Latin Extended-A, so
// subject-case may name another case for a subject with one outside them,
// the verdict the same. tools/selftest/header-agreement.ts holds it to a
// recorded fixture of commitlint's verdicts, commitlint no longer being
// installed.
//
// # A commit's type, git's own headers and merges
//
// Type is the one reading of a commit's type that the paths, the footers,
// the moves rule and itos commit's footer flags share. The header lint
// leaves git's own headers alone, as commitlint does, but every other rule
// judges them by what they stand for: Revert "…" and Reapply "…" are revert,
// and an amend!, fixup! or squash! commit is the type of the header it names
// once the prefixes are off (Named), a named header with no type of the
// commit types refused under named-type (namedProblems, run first among the
// footer rules, so every lint path has it). A merge is judged by its own
// changes (git.OwnPaths): one with none passes whatever its message; one
// with some is judged as its first line's type, its paths its own and its
// moves its own paths against its first parent, and refused under merge-type
// when that type is none of the commit types (untypedMerge, in
// internal/cli). Other headers the lint leaves alone that name no type (a
// "Merge …" header on a commit with one parent, a bare version) keep the
// word they start with, which no rule judges.
//
// # What itos commit writes
//
// Flag names the itos commit flag that writes a footer, Missing a type's
// required footers that neither the message nor the flags give, TextFlags
// each free-text footer's flag (-- and its key in lower case, unless one of
// BuiltinFlags has that name). Wrap breaks a body to BodyLimit (the
// built-in lint's 100, 0 for no lint or a delegate): it reads the -m
// paragraphs as one message, git's blank line between them, keeps the
// header, every line from the first the lint reads as a footer, comment and
// indented lines and lines within the limit, and breaks the rest greedily at
// spaces, a list item's continuation indented by its marker's width. A break
// whose next line would start with a footer token or a breaking-change note,
// a configured footer key and its colon, or the comment character moves back
// a word, and with none left the line runs over the limit. The registry
// writers wrap their bodies with it too.
package message
