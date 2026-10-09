// A program of this repository's is a shell script: tools/bin/itos builds and
// runs the Go binary, tools/bin/dev-version asks git for this checkout's
// version, tools/bin/pinned fetches a pinned tool. Linux and macOS start one by
// its #! line, which the kernel reads; Windows starts no script that way, so a
// spawn of one there ends with no exit code (ENOENT) and the gate that ran it
// fails without saying why. This gives the command and the arguments to spawn
// one with: sh and the script first on Windows, the script alone elsewhere, so
// a program under tools/ runs the repository's programs on every platform and
// nothing differs where it already works.
//
// Every such spawn goes through here. A program that runs something else — go,
// git, gh, node — is spawned as itself on every platform and does not come
// here.
export function repoProgram(
	script: string,
	args: readonly string[] = [],
): [file: string, argv: string[]] {
	if (process.platform !== "win32") return [script, [...args]];
	// Slashes: sh reads a backslash in the path as an escape.
	return ["sh", [script.replace(/\\/g, "/"), ...args]];
}
