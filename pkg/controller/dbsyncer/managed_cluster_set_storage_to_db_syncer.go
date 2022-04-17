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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const managedClusterSetLabelKey = "cluster.open-cluster-management.io/clusterset"

// NewManagedClusterSetStorageToDBSyncer returns a new instance of ManagedClusterSetStorageToDBSyncer.
func NewManagedClusterSetStorageToDBSyncer(specDB db.SpecDB, k8sClient client.Client,
	rbacAuthorizer authorizer.Authorizer,
) StorageToDBSyncer {
	return &genericStorageToDBSyncer{
		log:                ctrl.Log.WithName("managed-cluster-set-storage-to-db-syncer"),
		gitRepoToCommitMap: make(map[string]string),
		syncGitResourceFunc: func(ctx context.Context, base64UserID string, base64UserGroup string,
			buf *bytes.Buffer) error {
			return syncManagedClusterSet(ctx, k8sClient, specDB, rbacAuthorizer, base64UserID, base64UserGroup, buf)
		},
	}
}

func syncManagedClusterSet(ctx context.Context, k8sClient client.Client, specDB db.SpecDB,
	authorizer authorizer.Authorizer, base64UserID string, base64UserGroup string, buf *bytes.Buffer,
) error {
	managedClusterSet, err := yamltypes.NewManagedClusterSetFromBytes(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to create managed cluster set - %w", err)
	}

	// get decoded identity - assuming correctness because annotated by operator
	userID, _ := base64.StdEncoding.DecodeString(base64UserID)
	userGroup, _ := base64.StdEncoding.DecodeString(base64UserGroup)

	hubToManagedClustersMap := make(map[string]set.Set)

	for _, identifier := range managedClusterSet.Spec.Identifiers {
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

	if err := createCRAndAssignLabels(ctx, k8sClient, specDB, managedClusterSet, hubToManagedClustersMap); err != nil {
		return fmt.Errorf("failed to create managed cluster set - %w", err)
	}

	return nil
}

func createCRAndAssignLabels(ctx context.Context, k8sClient client.Client, specDB db.SpecDB,
	managedClusterSet *yamltypes.ManagedClusterSet, hubToManagedClustersMap map[string]set.Set,
) error {
	// update CR in cluster - if already exists then it's ok
	if err := k8sClient.Create(ctx, managedClusterSet.GetCR()); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create ManagedClusterSet resource in cluster - %w", err)
	}

	if err := specDB.UpdateLabelForManagedClusters(ctx, managedClusterLabelsDBTableName, managedClusterSetLabelKey,
		managedClusterSet.Metadata.Name, hubToManagedClustersMap); err != nil {
		return fmt.Errorf("failed to update managed clusters group - %w", err)
	}

	return nil
}
