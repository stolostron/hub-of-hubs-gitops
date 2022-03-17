package dbsyncer

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	set "github.com/deckarep/golang-set"
	"github.com/go-logr/logr"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/authorizer"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/db"
	yamltypes "github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/types"
	"gopkg.in/src-d/go-git.v4"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	managedClusterLabelsDBTableName = "managed_clusters_labels"
)

// NewManagedClustersGroupStorageToDBSyncer returns a new instance of ManagedClustersGroupStorageToDBSyncer.
func NewManagedClustersGroupStorageToDBSyncer(db db.SpecDB,
	rbacAuthorizer authorizer.Authorizer,
) *ManagedClustersGroupStorageToDBSyncer {
	return &ManagedClustersGroupStorageToDBSyncer{
		log:                ctrl.Log.WithName("managed-clusters-group-storage-to-db-syncer"),
		db:                 db,
		authorizer:         rbacAuthorizer,
		dbTableName:        managedClusterLabelsDBTableName,
		gitRepoToCommitMap: make(map[string]string),
	}
}

// ManagedClustersGroupStorageToDBSyncer handles syncing managed-clusters-group from git storage.
type ManagedClustersGroupStorageToDBSyncer struct {
	log                logr.Logger
	db                 db.SpecDB
	authorizer         authorizer.Authorizer
	dbTableName        string
	gitRepoToCommitMap map[string]string // TODO: map repo -> file -> mod time
}

// SyncGitRepo operates on a local git repo to sync contained objects of managed-cluster-groups.
func (syncer *ManagedClustersGroupStorageToDBSyncer) SyncGitRepo(ctx context.Context, base64UserIdentity string,
	base64UserGroup string, gitRepoFullPath string, forceReconcile bool,
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

	if syncer.walkGitRepo(ctx, base64UserIdentity, base64UserGroup, gitRepoFullPath) { // all succeeded
		syncer.gitRepoToCommitMap[gitRepoFullPath] = commit.ID().String()
		syncer.log.Info("synced repo", "root", gitRepoFullPath, "commit", commit.ID().String())

		return true
	}

	return false // at least one failed
}

func (syncer *ManagedClustersGroupStorageToDBSyncer) walkGitRepo(ctx context.Context, base64UserIdentity string,
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

		managedClustersGroup, err := yamltypes.NewManagedClustersGroupFromBytes(buf.Bytes())
		if err != nil {
			syncer.log.Error(err, "failed to create managed clusters group", "filepath", path)
			return nil
		}

		if err := syncer.syncManagedClustersGroup(ctx, base64UserIdentity, base64UserGroup,
			managedClustersGroup); err != nil {
			syncer.log.Error(err, "failed to sync managed-clusters-group in local git repo", "filepath", path)
			return nil
		}

		successRate++ // succeeded

		return nil
	})

	return successRate == 0 // all succeeded
}

func (syncer *ManagedClustersGroupStorageToDBSyncer) syncManagedClustersGroup(ctx context.Context, base64UserID string,
	base64UserGroup string, managedClustersGroup *yamltypes.ManagedClustersGroup,
) error {
	// get decoded identity - assuming correctness because annotated by operator
	userID, _ := base64.StdEncoding.DecodeString(base64UserID)
	userGroup, _ := base64.StdEncoding.DecodeString(base64UserGroup)

	// get group label key
	labelKey := fmt.Sprintf("%s/%s", db.HubOfHubsGroup, managedClustersGroup.Metadata.Name)

	hubToManagedClustersMap := make(map[string]set.Set)

	for _, identifier := range managedClustersGroup.Spec.Identifiers {
		for _, hubIdentifier := range identifier {
			hubToManagedClustersMap[hubIdentifier.Name] = createSetFromSlice(hubIdentifier.ManagedClusterIDs)
			syncer.log.Info("found identifier in request", "user", userID, "group", userGroup,
				"hub", hubIdentifier.Name, "clusters", hubToManagedClustersMap[hubIdentifier.Name].String())
		}
	}

	// get unauthorized managed clusters for subscribed user
	unauthorizedHubToManagedClustersMap, err := syncer.authorizer.FilterManagedClustersForUser(ctx, string(userID),
		[]string{string(userGroup)}, hubToManagedClustersMap)
	if err != nil {
		return fmt.Errorf("failed to filter by authorization - %w", err)
	}

	if len(unauthorizedHubToManagedClustersMap) != 0 { // found unauthorized managed clusters in user request
		for hubName, clustersSet := range unauthorizedHubToManagedClustersMap {
			if len(clustersSet.ToSlice()) == 0 {
				continue // means all good
			}

			syncer.log.Info("unauthorized entry found in request (removed)", "hub", hubName,
				"clusters", clustersSet.String())

			hubToManagedClustersMap[hubName] = hubToManagedClustersMap[hubName].Difference(clustersSet) // remove them
			if len(hubToManagedClustersMap[hubName].ToSlice()) == 0 {
				delete(hubToManagedClustersMap, hubName)
			}
		}
	}

	syncer.log.Info("updating managed cluster labels", "label", labelKey, "value", managedClustersGroup.Spec.TagValue)

	if err := syncer.db.UpdateManagedClustersSetLabel(ctx, managedClusterLabelsDBTableName, labelKey,
		managedClustersGroup.Spec.TagValue, hubToManagedClustersMap); err != nil {
		return fmt.Errorf("failed to update managed clusters group - %w", err)
	}

	return nil
}
