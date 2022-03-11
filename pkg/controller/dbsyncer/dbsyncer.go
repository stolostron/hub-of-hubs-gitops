package dbsyncer

import (
	"context"
)

// StorageToDBSyncer abstracts the functionality needed from a storage to DB syncer.
type StorageToDBSyncer interface {
	// SyncGitRepo operates on a local git repo to sync contained yaml files (depth 1).
	SyncGitRepo(ctx context.Context, base64UserIdentity string, base64UserGroup string, gitRepoPath string) bool
}
