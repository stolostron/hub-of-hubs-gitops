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
) *ManagedClusterSetStorageToDBSyncer {
	syncer := &ManagedClusterSetStorageToDBSyncer{
		genericStorageToDBSyncer: &genericStorageToDBSyncer{
			log:                ctrl.Log.WithName("managed-cluster-set-storage-to-db-syncer"),
			db:                 specDB,
			authorizer:         rbacAuthorizer,
			dbTableName:        managedClusterLabelsDBTableName,
			gitRepoToCommitMap: make(map[string]string),
		},
		k8sClient: k8sClient,
	}

	syncer.syncGitResourceFunc = syncer.syncManagedClusterSet

	return syncer
}

// ManagedClusterSetStorageToDBSyncer handles syncing managed-clusters-set from git storage.
type ManagedClusterSetStorageToDBSyncer struct {
	*genericStorageToDBSyncer
	k8sClient client.Client
}

func (syncer *ManagedClusterSetStorageToDBSyncer) syncManagedClusterSet(ctx context.Context, base64UserID string,
	base64UserGroup string, buf *bytes.Buffer,
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

	if err := syncer.createCRAndAssignLabels(ctx, managedClusterSet, hubToManagedClustersMap); err != nil {
		return fmt.Errorf("failed to create managed cluster set - %w", err)
	}

	return nil
}

func (syncer *ManagedClusterSetStorageToDBSyncer) createCRAndAssignLabels(ctx context.Context,
	managedClusterSet *yamltypes.ManagedClusterSet, hubToManagedClustersMap map[string]set.Set,
) error {
	// update CR in cluster - if already exists then it's ok
	if err := syncer.k8sClient.Create(ctx, managedClusterSet.GetCR()); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create ManagedClusterSet resource in cluster - %w", err)
	}

	syncer.log.Info("updating managed cluster labels", "label", managedClusterSetLabelKey,
		"value", managedClusterSet.Metadata.Name)

	if err := syncer.db.UpdateLabelForManagedClusters(ctx, managedClusterLabelsDBTableName, managedClusterSetLabelKey,
		managedClusterSet.Metadata.Name, hubToManagedClustersMap); err != nil {
		return fmt.Errorf("failed to update managed clusters group - %w", err)
	}

	return nil
}
