package draft

import (
	"context"
	"os"
	"path/filepath"

	"github.com/donvargax/itos/v8/internal/config"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Path is where the drafts are: drafts.yaml in the drafts folder of
// itos's folder of the git common dir v locates, "" outside a git
// repository.
func Path(ctx context.Context, v vcsport.VCS) string {
	common := absCommon(ctx, v)
	if common == "" {
		return ""
	}
	return filepath.Join(common, config.StealthFolder, Folder, FileName)
}

// absCommon is the git common dir v locates from the folder itos runs in
// (v's Folder), absolute, the folder's links resolved as git resolves them;
// "" outside a repository.
func absCommon(ctx context.Context, v vcsport.VCS) string {
	l, err := v.Locate(ctx, "")
	if err != nil || l.Common == "" {
		return ""
	}
	if filepath.IsAbs(l.Common) {
		return l.Common
	}
	cwd := v.Folder(ctx)
	if cwd == "" {
		// Where the run's folder is the process's (draft and init, until
		// they move without moving the process).
		cwd, _ = os.Getwd()
	}
	if real, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = real
	}
	return filepath.Join(cwd, l.Common)
}
