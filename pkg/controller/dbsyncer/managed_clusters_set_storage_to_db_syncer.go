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
	yamltypes "github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/yaml-types"
	"gopkg.in/src-d/go-git.v4"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	managedClusterLabelsDBTableName = "managed_clusters_labels"
	managedClustersSetKind          = "ManagedClustersSet"
)

// NewManagedClustersSetStorageToDBSyncer returns a new instance of ManagedClustersSetStorageToDBSyncer.
func NewManagedClustersSetStorageToDBSyncer(db db.SpecDB,
	rbacAuthorizer authorizer.Authorizer,
) *ManagedClustersSetStorageToDBSyncer {
	return &ManagedClustersSetStorageToDBSyncer{
		log:                      ctrl.Log.WithName("managed-clusters-set-storage-to-db-syncer"),
		db:                       db,
		authorizer:               rbacAuthorizer,
		dbTableName:              managedClusterLabelsDBTableName,
		gitRepoToSyncedCommitMap: make(map[string]string),
	}
}

// ManagedClustersSetStorageToDBSyncer handles syncing managed-clusters-set from git storage.
type ManagedClustersSetStorageToDBSyncer struct {
	log                      logr.Logger
	db                       db.SpecDB
	authorizer               authorizer.Authorizer
	dbTableName              string
	gitRepoToSyncedCommitMap map[string]string // TODO: map repo -> file -> synced hash
}

// SyncGitRepo operates on a local git repo to sync contained objects of managed-cluster-sets.
func (syncer *ManagedClustersSetStorageToDBSyncer) SyncGitRepo(ctx context.Context, base64UserIdentity string,
	base64UserGroup string, gitRepoPath string,
) bool {
	repo, err := git.PlainOpen(gitRepoPath)
	if err != nil {
		syncer.log.Error(err, "failed to open local git repo", "path", gitRepoPath)
		return false
	}

	ref, err := repo.Head()
	if err != nil {
		syncer.log.Error(err, "failed to get head of local git repo", "path", gitRepoPath)
		return false
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		syncer.log.Error(err, "failed to get latest commit of head of local git repo", "path", gitRepoPath)
		return false
	}

	commitID := commit.ID().String()

	if syncedCommit, found := syncer.gitRepoToSyncedCommitMap[gitRepoPath]; !found {
		syncer.gitRepoToSyncedCommitMap[gitRepoPath] = ""
	} else if syncedCommit == commitID {
		return false // no updates
	}

	return syncer.walkGitRepo(ctx, base64UserIdentity, base64UserGroup, gitRepoPath)
}

func (syncer *ManagedClustersSetStorageToDBSyncer) walkGitRepo(ctx context.Context, base64UserIdentity string,
	base64UserGroup string, gitRepoPath string,
) bool {
	successRate := 0

	_ = filepath.WalkDir(gitRepoPath, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			syncer.log.Error(err, "walkdir failed", "filepath", path)
			return nil
		}

		if dirEntry.IsDir() {
			return nil // for now supporting first depth only
		}

		successRate-- // all function's failure exit paths will not undo this

		// open file for read
		file, err := os.Open(path)
		if err != nil {
			syncer.log.Error(err, "failed to open file in local git repo", "filepath", path)
			return nil
		}
		// make sure kind is correct (reads first line only)
		if kind, err := getGitYamlKind(file); err != nil || kind != managedClustersSetKind {
			//nolint
			return nil // irrelevant file
		}

		// buffer for file
		buf := bytes.NewBuffer(nil)
		// copy bytes into buffer
		if _, err := io.Copy(buf, file); err != nil {
			syncer.log.Error(err, "failed to copy file bytes in local git repo", "filepath", path)
			return nil
		}

		managedClustersSet, err := yamltypes.NewManagedClustersSetFromBytes(buf.Bytes())
		if err != nil {
			syncer.log.Error(err, "failed to create managed clusters set", "filepath", path)
			return nil
		}

		if err := syncer.syncManagedClustersSet(ctx, base64UserIdentity, base64UserGroup,
			managedClustersSet); err != nil {
			syncer.log.Error(err, "failed to sync managed-clusters-set in local git repo", "filepath", path)
			return nil
		}

		successRate++ // succeeded

		return nil
	})

	return successRate == 0 // all succeeded
}

func (syncer *ManagedClustersSetStorageToDBSyncer) syncManagedClustersSet(ctx context.Context, base64UserID string,
	base64UserGroup string, managedClustersSet *yamltypes.ManagedClustersSet,
) error {
	hubToManagedClustersMap := make(map[string]set.Set)
	for _, hubIdentifier := range managedClustersSet.Spec.Identifiers {
		hubToManagedClustersMap[hubIdentifier.Name] = createSetFromSlice(hubIdentifier.ManagedClusterIDs)
	}

	// get decoded identity - assuming correctness because annotated by operator
	userID, _ := base64.StdEncoding.DecodeString(base64UserID)
	userGroup, _ := base64.StdEncoding.DecodeString(base64UserGroup)

	// get group label key
	labelKey := fmt.Sprintf("%s/%s", managedClustersSet.Metadata.Group, managedClustersSet.Metadata.Name)

	// get unauthorized managed clusters for subscribed user
	unauthorizedHubToManagedClustersMap, err := syncer.authorizer.FilterManagedClustersForUser(ctx, string(userID),
		[]string{string(userGroup)}, hubToManagedClustersMap)
	if err != nil {
		return fmt.Errorf("failed to filter by authorization - %w", err)
	}

	if len(unauthorizedHubToManagedClustersMap) != 0 { // found unauthorized managed clusters in user request
		for hubName, clustersSet := range unauthorizedHubToManagedClustersMap {
			syncer.log.Info("unauthorized entry found in request (removed)", "hub", hubName,
				"clusters", clustersSet.String())

			hubToManagedClustersMap[hubName] = hubToManagedClustersMap[hubName].Difference(clustersSet) // remove them
		}
	}

	if err := syncer.db.UpdateManagedClustersSetLabel(ctx, managedClusterLabelsDBTableName, labelKey,
		hubToManagedClustersMap); err != nil {
		return fmt.Errorf("failed to update managed clusters group - %w", err)
	}

	return nil
}
