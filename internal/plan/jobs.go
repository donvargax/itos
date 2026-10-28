package plan

import (
	"context"
	"path/filepath"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/glob"
	"github.com/donvargax/itos/v8/internal/value"
	vcs "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Why a job of the CI workflow runs or does not (slice 202), as the plan's
// JSON gives it.
const (
	// JobNoPaths: the job declares no paths, so it always runs.
	JobNoPaths = "no-paths"
	// JobWhole: the plan has no readable range, so every job runs.
	JobWhole = "whole"
	// JobConfig: the range touches the config file itos read.
	JobConfig = "config"
	// JobWorkflow: the range touches a workflow, where the jobs are defined.
	JobWorkflow = "workflow"
	// JobTouched: the range touches one of the job's paths.
	JobTouched = "touched"
	// JobUntouched: it touches none.
	JobUntouched = "untouched"
)

// workflows is where the CI workflow defines its jobs: a range touching a file
// under it may change any job.
const workflows = ".github/workflows/"

// JobRun is a job ci.jobs declares and whether the plan's range runs it.
type JobRun struct {
	Name   string
	Runs   bool
	Reason string
}

// jobsJSON is the plan's "jobs": an object of each declared job's answer, in
// the order the config writes them; {} when it declares none.
func jobsJSON(jobs []JobRun) *value.Map {
	m := value.NewMap()
	for _, j := range jobs {
		m.Set(j.Name, value.NewMap("runs", j.Runs, "reason", j.Reason))
	}
	return m
}

// jobsOf are the jobs ci.jobs declares, in the config's order, with whether a
// range runs each, for the first reason that holds: it declares no paths; the
// range is whole (the plan has no readable range, so nothing is judged by its
// paths); the range touches the config file or a workflow; it touches one of
// the job's paths, or none. changed are the range's changed paths. An error
// for a glob itos cannot read, when a path reaches it.
func jobsOf(cfg *config.Loaded, whole bool, changed []string) ([]JobRun, error) {
	jobs := make([]JobRun, 0, len(cfg.CI.Jobs.Keys))
	var configPath string
	if !whole && len(cfg.CI.Jobs.Keys) > 0 {
		configPath = configRel(cfg)
	}
	workflow := slices.ContainsFunc(changed, func(p string) bool { return strings.HasPrefix(p, workflows) })
	for _, name := range cfg.CI.Jobs.Keys {
		paths := cfg.CI.Jobs.Values[name].Paths
		run := JobRun{Name: name, Runs: true}
		switch {
		case len(paths) == 0:
			run.Reason = JobNoPaths
		case whole:
			run.Reason = JobWhole
		case configPath != "" && slices.Contains(changed, configPath):
			run.Reason = JobConfig
		case workflow:
			run.Reason = JobWorkflow
		default:
			touched, err := touches(paths, changed)
			if err != nil {
				return nil, err
			}
			run.Runs, run.Reason = touched, JobUntouched
			if touched {
				run.Reason = JobTouched
			}
		}
		jobs = append(jobs, run)
	}
	return jobs, nil
}

// touches is whether any of the paths matches one of the globs.
func touches(globs, paths []string) (bool, error) {
	for _, p := range paths {
		ok, err := glob.MatchesAny(p, globs)
		if err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

// configRel is the config file itos read as a path of the repository: relative
// to its top level with symlinks resolved, forward slashes; "" for a config
// outside the working tree (the stealth one's in a linked worktree's common
// dir, an ITOS_CONFIG outside the repository), which no range touches, and
// for one that cannot be placed.
func configRel(cfg *config.Loaded) string {
	loc, err := cfg.VCS.Locate(context.Background(), "")
	if err != nil {
		return ""
	}
	top, file := realPath(loc.Top), realPath(vcs.In(cfg.Folder(), cfg.Path))
	if top == "" || file == "" {
		return ""
	}
	rel, err := filepath.Rel(top, file)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	return filepath.ToSlash(rel)
}

// realPath is a path with its symlinks resolved; "" for none or one that
// cannot be.
func realPath(p string) string {
	resolved, err := filepath.EvalSymlinks(p)
	if p == "" || err != nil {
		return ""
	}
	return resolved
}
