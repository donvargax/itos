// Package glob is the path globs the config writes (commits.scopes, ci
// patterns, smoke and prose paths): `*` does not cross `/`, `**` does, `**/`
// may match nothing, `{a,b}` is either, a glob matches the whole path, and a
// glob with no `/` matches at the root only.
//
// The globs mean what they meant in itos's first, TypeScript implementation,
// quirks included, since configs were written against it: Source turns a
// glob into a regular expression by the same five replacements in the same
// order (`?` stays a regular expression's `?`, and a `*` right after a `.`
// is left alone, so `a.*` is `a` and any dots). RE2 forced two changes of
// construction, neither of meaning: the last replacement's lookbehind (a `*`
// not after a `.`) is a scan, since RE2 has no lookbehind; and every `.` the
// replacements write is JavaScript's dot, any character but a line
// terminator, written out as the class [^\n\r\x{2028}\x{2029}], since RE2's
// dot leaves out `\n` alone.
//
// A glob V8 would refuse is an error worded as V8 words it (`Invalid regular
// expression: /^?a$/: Nothing to repeat`), the leading `?` RE2 would take
// included, and only when a path reaches it: MatchesAny stops at the first
// match, so a bad glob after it is never compiled.
package glob
