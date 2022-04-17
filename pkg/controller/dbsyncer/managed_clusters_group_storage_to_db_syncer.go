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
) StorageToDBSyncer {
	return &genericStorageToDBSyncer{
		log:                ctrl.Log.WithName("managed-clusters-group-storage-to-db-syncer"),
		gitRepoToCommitMap: make(map[string]string),
		syncGitResourceFunc: func(ctx context.Context, base64UserID string, base64UserGroup string,
			buf *bytes.Buffer) error {
			return syncManagedClustersGroup(ctx, specDB, rbacAuthorizer, base64UserID, base64UserGroup, buf)
		},
	}
}

func syncManagedClustersGroup(ctx context.Context, specDB db.SpecDB, authorizer authorizer.Authorizer,
	base64UserID string, base64UserGroup string, buf *bytes.Buffer,
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
		}
	}

	// get unauthorized managed clusters for subscribed user
	unauthorizedHubToManagedClustersMap, err := authorizer.FilterManagedClustersForUser(ctx, string(userID),
		[]string{string(userGroup)}, hubToManagedClustersMap)
	if err != nil {
		return fmt.Errorf("failed to filter by authorization - %w", err)
	}

	if len(unauthorizedHubToManagedClustersMap) != 0 { // found unauthorized managed clusters in user request
		for hubName, clustersSet := range unauthorizedHubToManagedClustersMap {
			if len(clustersSet.ToSlice()) == 0 {
				continue // means all good
			}

			hubToManagedClustersMap[hubName] = hubToManagedClustersMap[hubName].Difference(clustersSet) // remove them
			if len(hubToManagedClustersMap[hubName].ToSlice()) == 0 {
				delete(hubToManagedClustersMap, hubName)
			}
		}
	}

	if err := specDB.UpdateLabelForManagedClusters(ctx, managedClusterLabelsDBTableName, labelKey,
		managedClustersGroup.Spec.TagValue, hubToManagedClustersMap); err != nil {
		return fmt.Errorf("failed to update managed clusters group - %w", err)
	}

	return nil
}
