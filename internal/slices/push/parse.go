package push

import (
	"strings"

	"github.com/donvargax/itos/v8/internal/engine/gitrun"
	"github.com/donvargax/itos/v8/internal/out"
)

// CLI is push's command line: the Query it asks, whether it prints
// --json's one object, and whether git is quiet (the global -q).
type CLI struct {
	Query
	JSON, Quiet bool
}

// Parse reads the arguments internal/cli's flag spec hands the push
// command, with the global --json and -q and origin, the folder the person
// stood in (relative to the top itos moved to, "" when it did not move),
// where git runs.
func Parse(args []string, json, quiet bool, origin string) (CLI, error) {
	q, err := readPushArgs(args)
	if err != nil {
		return CLI{}, err
	}
	q.Origin = origin
	return CLI{Query: q, JSON: json, Quiet: quiet}, nil
}

// readPushArgs refuses every argument but --no-wait, --ready, --onto-main,
// --tag <tag> and --ref <branch> (each as --tag=<tag> too, and as often as
// wanted), which it gives, --ready and --onto-main with no --tag or --ref
// (slices 179 and 193): a plain itos push pushes HEAD to the branch's upstream
// and takes nothing else, and an argument that would force the push says why
// it is refused, --onto-main's lease being itos's own. A name is a tag's or a
// branch's, not a refspec.
func readPushArgs(args []string) (Query, error) {
	if len(args) > 0 && (args[0] == "threads" || args[0] == "thread") {
		return readThreadArgs(args[0], args[1:])
	}
	var a Query
	var rest []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--no-wait" {
			a.NoWait = true
			continue
		}
		if arg == "--ready" {
			a.Ready = true
			continue
		}
		if arg == "--onto-main" {
			a.OntoMain = true
			continue
		}
		flag, name, inline := strings.Cut(arg, "=")
		if flag != "--tag" && flag != "--ref" {
			rest = append(rest, arg)
			continue
		}
		if !inline && i+1 < len(args) {
			i++
			name = args[i]
		}
		if err := pushName(flag, name); err != nil {
			return a, err
		}
		if flag == "--tag" {
			a.Tags = append(a.Tags, name)
		} else {
			a.Refs = append(a.Refs, name)
		}
	}
	if a.OntoMain && len(a.Tags)+len(a.Refs) > 0 {
		return a, out.Usage("push --onto-main rebases an item's branch onto main; it takes no --tag or --ref")
	}
	if a.Ready && len(a.Tags)+len(a.Refs) > 0 {
		return a, out.Usage("push --ready pushes an item's branch; it takes no --tag or --ref")
	}
	return a, refusePushArgs(rest)
}

// readThreadArgs reads what follows push threads, which takes nothing, or push
// thread: a thread's id and --reply <text> (as --reply=<text> too), --resolve,
// or both (slice 191). Every other argument is a usage error.
func readThreadArgs(word string, args []string) (Query, error) {
	if word == "threads" {
		if len(args) > 0 {
			return Query{}, out.Usage("push threads takes no arguments (%s): it lists the open review threads of the branch's pull request", args[0])
		}
		return Query{Threads: true}, nil
	}
	var q Query
	for i := 0; i < len(args); i++ {
		flag, text, inline := strings.Cut(args[i], "=")
		switch {
		case args[i] == "--resolve":
			q.Resolve = true
		case flag == "--reply":
			if !inline && i+1 < len(args) {
				i++
				text = args[i]
			}
			if text == "" {
				return Query{}, out.Usage("push thread --reply needs the reply's text")
			}
			q.Reply = text
		case strings.HasPrefix(args[i], "-"):
			return Query{}, out.Usage("push thread takes a thread's id, --reply <text> and --resolve, not %s", args[i])
		case q.Thread != "":
			return Query{}, out.Usage("push thread answers one thread, and %s is a second: %s", args[i], q.Thread)
		default:
			q.Thread = args[i]
		}
	}
	switch {
	case q.Thread == "":
		return Query{}, out.Usage("push thread needs a thread's id: itos push threads lists them")
	case q.Reply == "" && !q.Resolve:
		return Query{}, out.Usage("push thread %s needs --reply <text>, --resolve or both", q.Thread)
	}
	return q, nil
}

// pushName refuses what --tag or --ref is given when it is no name: nothing,
// an option, a forcing refspec or a refspec at all.
func pushName(flag, name string) error {
	what := map[string]string{"--tag": "a tag's name", "--ref": "a branch's name"}[flag]
	switch {
	case name == "" || strings.HasPrefix(name, "-"):
		return out.Usage("push %s needs %s", flag, what)
	case strings.HasPrefix(name, "+"):
		return out.Usage("push never forces: %s is a forcing refspec; push %s takes %s", name, flag, what)
	case strings.Contains(name, ":"):
		return out.Usage("push %s takes %s, not a refspec (%s): it pushes to the remote's of the same name", flag, what, name)
	}
	return nil
}

// refusePushArgs refuses each argument, a forcing one and one that skips the
// pre-push hook (--no-verify, slice 128) saying why.
func refusePushArgs(args []string) error {
	for _, arg := range args {
		switch {
		case gitrun.NoVerify(arg):
			return out.Usage("push refuses %s: it skips the pre-push hook, and the checks it runs; push without it, "+
				"and fix what the hook reports", arg)
		case arg == "-f" || strings.HasPrefix(arg, "--force") ||
			(strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.Contains(arg, "f")):
			return out.Usage("push never forces: %s would overwrite the remote's commits; itos push rebases onto them instead", arg)
		case strings.HasPrefix(arg, "+"):
			return out.Usage("push never forces: %s is a forcing refspec; itos push rebases onto the remote's commits instead", arg)
		}
	}
	if len(args) > 0 {
		return out.Usage("push takes no arguments but --no-wait, --ready, --onto-main, --tag <tag> and --ref <branch>: it pushes HEAD to the "+
			"branch's upstream (%s)", args[0])
	}
	return nil
}
