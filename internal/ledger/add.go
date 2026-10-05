package ledger

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v4/internal/config"
	"github.com/donvargax/itos/v4/internal/out"
	"github.com/donvargax/itos/v4/internal/value"
)

// NewTask is the task itos task add writes (slice 55): its id, type, title
// and why, its group, and its checks in order.
type NewTask struct {
	ID, Type, Title, Why, Group string
	Checks                      []NewCheck
}

// NewCheck is one check of a new task: its command, run: (it must pass),
// and its own timeout in seconds, nil for the ledger's.
type NewCheck struct {
	Run     string
	Timeout *float64
}

// Added is the ledger with a task added: the file of its group, the file's
// text before ("" for a file not there yet) and after, whether the file is
// new, and the task as it reads.
type Added struct {
	Path, Old, Text string
	Created         bool
	Task            *value.Map
}

// Add is the ledger with a new task at the end of its group's file, its keys
// in the order a task's are written (id, type, title, why, done_when), its
// why a folded text; the file is made when the group has none yet. Refused,
// with nothing written: an id ledger.id does not match, a type
// commits.types does not list, a ledger that has problems (config check's)
// or already has the id, and a group whose file ledger.files cannot name
// (ledger.group.pattern). A ledger folder that is missing is Files' error.
func Add(cfg *config.Loaded, n NewTask) (Added, []out.Problem, error) {
	refuse := func(rule, message, fix string) (Added, []out.Problem, error) {
		return Added{}, []out.Problem{problem(rule, message, fix)}, nil
	}
	if pattern := IDPattern(cfg); !pattern.MatchString(n.ID) {
		return refuse("ledger-add-not-task-id", fmt.Sprintf("%s is not a task ID by ledger.id (%s)", n.ID, pattern.String()),
			"give the task an ID ledger.id matches, so a Task footer can name it")
	}
	if types := cfg.Commits.Types; types != nil && !value.Includes(types, n.Type) {
		return refuse("ledger-add-type", fmt.Sprintf("%s is not a commit type: commits.types lists %s", n.Type, strings.Join(types, ", ")),
			"pass --type with one of "+strings.Join(types, ", "))
	}
	layout, err := LayoutOf(cfg)
	if err != nil {
		return Added{}, nil, err
	}
	files, err := Files(cfg)
	if err != nil {
		return Added{}, nil, err
	}
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	if found := Issues(cfg, paths); len(found) > 0 {
		return Added{}, found, nil
	}
	file := ""
	for _, f := range files {
		tasks, _ := read(f.Path)
		for _, t := range tasks {
			if value.Prop(t, "id") == n.ID {
				return refuse("ledger-add-taken", fmt.Sprintf("%s is already a task of the ledger, in %s: %s", n.ID, f.Path, value.String(value.Prop(t, "title"))),
					"give the new task an ID the ledger does not have")
			}
		}
		if file == "" && sameGroup(cfg, f.Group, n.Group) {
			file = f.Path
		}
	}
	label := cfg.Ledger.Group.Label
	if file == "" {
		file = path.Join(layout.Dir, strings.ReplaceAll(path.Base(filepath.ToSlash(cfg.Ledger.Files)), "{group}", n.Group))
		if !strings.Contains(cfg.Ledger.Files, "{group}") || !layout.File.MatchString(path.Base(file)) {
			return refuse("ledger-add-group", fmt.Sprintf("%s is not a %s ledger.files can name: ledger.group.pattern is %s", n.Group, label, cfg.Ledger.Group.Pattern),
				"pass a "+label+" ledger.group.pattern matches")
		}
	}
	task := value.NewMap("id", n.ID, "type", n.Type, "title", n.Title)
	if value.Trim(n.Why) != "" {
		task.Set("why", n.Why)
	}
	if len(n.Checks) > 0 {
		checks := make([]any, len(n.Checks))
		for i, c := range n.Checks {
			check := value.NewMap("run", c.Run)
			if c.Timeout != nil {
				check.Set("timeout", *c.Timeout)
			}
			checks[i] = check
		}
		task.Set("done_when", checks)
	}
	if found := taskProblems(cfg, task); len(found) > 0 {
		for i := range found {
			found[i].Message = n.ID + ": " + found[i].Message
		}
		return Added{}, found, nil
	}
	old, err := os.ReadFile(file)
	created := errors.Is(err, fs.ErrNotExist)
	if err != nil && !created {
		return Added{}, nil, err
	}
	text, err := appendTask(string(old), task)
	if err != nil {
		return Added{}, nil, fmt.Errorf("%s cannot be edited in place: %w", file, err)
	}
	read, err := value.Parse(text)
	if err != nil {
		return Added{}, nil, err
	}
	list := read.([]any)
	return Added{Path: file, Old: string(old), Text: text, Created: created, Task: list[len(list)-1].(*value.Map)}, nil, nil
}

// sameGroup is whether a file's group is the one given: Number() of each
// when the groups are numeric, so 01 is group 1.
func sameGroup(cfg *config.Loaded, file, group string) bool {
	if cfg.Ledger.Group.Numeric {
		return value.ToNumber(file) == value.ToNumber(group)
	}
	return file == group
}

// appendTask is a ledger file's text with the task after its last: appended
// in place (value.Doc), every comment and line kept, or, in a file that
// holds no task (none at all, or only comments), written after what is
// there.
func appendTask(text string, task *value.Map) (string, error) {
	tasks, err := value.Parse(text)
	if err != nil {
		return "", err
	}
	if tasks == nil {
		item, err := value.BlockItem(task, "why")
		if err != nil {
			return "", err
		}
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		return text + item, nil
	}
	doc, err := value.OpenDoc(text)
	if err != nil {
		return "", err
	}
	if err := doc.Append(nil, task, "why"); err != nil {
		return "", err
	}
	return doc.Text()
}
