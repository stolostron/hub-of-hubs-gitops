package dbsyncer

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"gopkg.in/src-d/go-git.v4"
)

type syncGitResourceFunc func(ctx context.Context, base64UserID string, base64UserGroup string,
	buf *bytes.Buffer) error

// genericStorageToDBSyncer generalizes the handling of git storage repos.
type genericStorageToDBSyncer struct {
	log                 logr.Logger
	gitRepoToCommitMap  map[string]string
	syncGitResourceFunc syncGitResourceFunc
}

// SyncGitRepo operates on a local git repo to sync contained objects of managed-cluster-groups.
func (syncer *genericStorageToDBSyncer) SyncGitRepo(ctx context.Context, base64UserIdentity string,
	base64UserGroup string, gitRepoFullPath string, workPath string, forceReconcile bool,
) bool {
	repo, err := git.PlainOpen(gitRepoFullPath)
	if err != nil {
		syncer.log.Error(err, "failed to open local git repo", "root", gitRepoFullPath)
		return false
	}

	ref, err := repo.Head()
	if err != nil {
		syncer.log.Error(err, "failed to open head of local git repo", "root", gitRepoFullPath)
		return false
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		syncer.log.Error(err, "failed to get commit of head", "root", gitRepoFullPath)
		return false
	}

	if forceReconcile {
		syncer.gitRepoToCommitMap[gitRepoFullPath] = ""
	}

	if syncedCommit, found := syncer.gitRepoToCommitMap[gitRepoFullPath]; !found {
		syncedCommit = ""
		syncer.gitRepoToCommitMap[gitRepoFullPath] = syncedCommit
	} else if syncedCommit == commit.ID().String() {
		return false // no updates
	}

	if workPath != "" {
		workPath = filepath.Join(gitRepoFullPath, workPath)
	}

	if syncer.walkGitRepo(ctx, base64UserIdentity, base64UserGroup, workPath) { // all succeeded
		syncer.gitRepoToCommitMap[gitRepoFullPath] = commit.ID().String()
		syncer.log.Info("synced repo", "root", gitRepoFullPath, "commit", commit.ID().String())

		return true
	}

	return false // at least one failed
}

func (syncer *genericStorageToDBSyncer) walkGitRepo(ctx context.Context, base64UserIdentity string,
	base64UserGroup string, gitRepoFullPath string,
) bool {
	successRate := 0

	_ = filepath.WalkDir(gitRepoFullPath, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			syncer.log.Error(err, "walkdir failed", "filepath", path)
			return nil
		}

		if dirEntry.IsDir() || filepath.Dir(path) != gitRepoFullPath {
			return nil // for now supporting first depth only
		}

		successRate-- // all function's failure exit paths will not undo this

		// open file for read
		file, err := os.Open(path)
		if err != nil {
			syncer.log.Error(err, "failed to open file in local git repo", "filepath", path)
			return nil
		}

		// buffer for file
		buf := bytes.NewBuffer(nil)
		// copy bytes into buffer
		if _, err := io.Copy(buf, file); err != nil {
			syncer.log.Error(err, "failed to copy file bytes in local git repo", "filepath", path)
			return nil
		}

		if err := syncer.syncGitResourceFunc(ctx, base64UserIdentity, base64UserGroup,
			buf); err != nil {
			syncer.log.Error(err, "failed to sync git resource in local git repo", "filepath", path)
			return nil
		}

		successRate++ // succeeded

		return nil
	})

	return successRate == 0 // all succeeded
}
