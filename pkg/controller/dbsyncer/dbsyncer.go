package dbsyncer

import (
	"context"
)

// StorageToDBSyncer abstracts the functionality needed from a storage to DB syncer.
type StorageToDBSyncer interface {
	// SyncGitRepo operates on a local git repo to sync contained yaml files (depth 1). workPath is the relative
	// path to sync objects from (sets workdir = gitRepoPath/workPath). If left empty, the workdir is gitRepoPath.
	SyncGitRepo(ctx context.Context, base64UserIdentity string, base64UserGroup string, gitRepoPath string,
		workPath string, dirs []string, forceReconcile bool) bool
}
