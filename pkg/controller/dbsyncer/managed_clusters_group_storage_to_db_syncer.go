package dbsyncer

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"

	set "github.com/deckarep/golang-set"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/authorizer"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/db"
	yamltypes "github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	managedClusterLabelsDBTableName = "managed_clusters_labels"
)

// NewManagedClustersGroupStorageToDBSyncer returns a new instance of ManagedClustersGroupStorageToDBSyncer.
func NewManagedClustersGroupStorageToDBSyncer(specDB db.SpecDB,
	rbacAuthorizer authorizer.Authorizer,
) *ManagedClustersGroupStorageToDBSyncer {
	syncer := &ManagedClustersGroupStorageToDBSyncer{
		genericStorageToDBSyncer: &genericStorageToDBSyncer{
			log:                ctrl.Log.WithName("managed-clusters-group-storage-to-db-syncer"),
			db:                 specDB,
			authorizer:         rbacAuthorizer,
			dbTableName:        managedClusterLabelsDBTableName,
			gitRepoToCommitMap: make(map[string]string),
		},
	}

	syncer.syncGitResourceFunc = syncer.syncManagedClustersGroup

	return syncer
}

// ManagedClustersGroupStorageToDBSyncer handles syncing managed-clusters-group from git storage.
type ManagedClustersGroupStorageToDBSyncer struct {
	*genericStorageToDBSyncer
}

func (syncer *ManagedClustersGroupStorageToDBSyncer) syncManagedClustersGroup(ctx context.Context, base64UserID string,
	base64UserGroup string, buf *bytes.Buffer,
) error {
	managedClustersGroup, err := yamltypes.NewManagedClustersGroupFromBytes(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to create managed clusters group - %w", err)
	}

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

	if err := syncer.db.UpdateLabelForManagedClusters(ctx, managedClusterLabelsDBTableName, labelKey,
		managedClustersGroup.Spec.TagValue, hubToManagedClustersMap); err != nil {
		return fmt.Errorf("failed to update managed clusters group - %w", err)
	}

	return nil
}
