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
	"time"

	set "github.com/deckarep/golang-set"
	"github.com/go-logr/logr"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/authorizer"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/db"
	yamltypes "github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/yaml-types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	managedClusterLabelsDBTableName = "managed_clusters_labels"
)

// NewManagedClustersSetStorageToDBSyncer returns a new instance of ManagedClustersSetStorageToDBSyncer.
func NewManagedClustersSetStorageToDBSyncer(db db.SpecDB,
	rbacAuthorizer authorizer.Authorizer,
) *ManagedClustersSetStorageToDBSyncer {
	return &ManagedClustersSetStorageToDBSyncer{
		log:                 ctrl.Log.WithName("managed-clusters-set-storage-to-db-syncer"),
		db:                  db,
		authorizer:          rbacAuthorizer,
		dbTableName:         managedClusterLabelsDBTableName,
		gitRepoToModTimeMap: make(map[string]time.Time),
	}
}

// ManagedClustersSetStorageToDBSyncer handles syncing managed-clusters-set from git storage.
type ManagedClustersSetStorageToDBSyncer struct {
	log                 logr.Logger
	db                  db.SpecDB
	authorizer          authorizer.Authorizer
	dbTableName         string
	gitRepoToModTimeMap map[string]time.Time // TODO: map repo -> file -> mod time
}

// SyncGitRepo operates on a local git repo to sync contained objects of managed-cluster-sets.
func (syncer *ManagedClustersSetStorageToDBSyncer) SyncGitRepo(ctx context.Context, base64UserIdentity string,
	base64UserGroup string, gitRepoFullPath string,
) bool {
	dirInfo, err := os.Stat(gitRepoFullPath)
	if err != nil {
		syncer.log.Error(err, "failed to stat git root", "root", gitRepoFullPath)
		return false
	}

	if lastModTime, found := syncer.gitRepoToModTimeMap[gitRepoFullPath]; !found {
		lastModTime = time.Time{}
		syncer.gitRepoToModTimeMap[gitRepoFullPath] = lastModTime
	} else if dirInfo.ModTime().Equal(lastModTime) {
		return false // no updates
	}

	if syncer.walkGitRepo(ctx, base64UserIdentity, base64UserGroup, gitRepoFullPath) { // all succeeded
		syncer.gitRepoToModTimeMap[gitRepoFullPath] = dirInfo.ModTime()
		syncer.log.Info("synced repo", "root", gitRepoFullPath, "mod-time", dirInfo.ModTime())

		return true
	}

	return false // at least one failed
}

func (syncer *ManagedClustersSetStorageToDBSyncer) walkGitRepo(ctx context.Context, base64UserIdentity string,
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
	// get decoded identity - assuming correctness because annotated by operator
	userID, _ := base64.StdEncoding.DecodeString(base64UserID)
	userGroup, _ := base64.StdEncoding.DecodeString(base64UserGroup)

	// get group label key
	labelKey := fmt.Sprintf("%s/%s", managedClustersSet.Metadata.Group, managedClustersSet.Metadata.Name)

	hubToManagedClustersMap := make(map[string]set.Set)
	for _, hubIdentifier := range managedClustersSet.Spec.Identifiers {
		hubToManagedClustersMap[hubIdentifier.Name] = createSetFromSlice(hubIdentifier.ManagedClusterIDs)
		syncer.log.Info("found identifier in request", "user", userID, "group", userGroup,
			"cluster", hubToManagedClustersMap[hubIdentifier.Name].String())
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
		}
	}

	syncer.log.Info("updating managed cluster labels", "label", labelKey)

	if err := syncer.db.UpdateManagedClustersSetLabel(ctx, managedClusterLabelsDBTableName, labelKey,
		hubToManagedClustersMap); err != nil {
		return fmt.Errorf("failed to update managed clusters group - %w", err)
	}

	return nil
}
