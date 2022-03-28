package controller

import (
	"fmt"
	"time"

	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"

	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/authorizer"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/controller/dbsyncer"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/db"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/intervalpolicy"
	"k8s.io/apimachinery/pkg/runtime"
	"open-cluster-management.io/multicloud-operators-subscription/pkg/apis"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	managedClustersGroupStorageToDBSyncerTag = "ManagedClustersGroup"
	managedClusterSetStorageToDBSyncerTag    = "HubOfHubsManagedClusterSet"
)

// AddToScheme adds all Resources to the Scheme.
func AddToScheme(runtimeScheme *runtime.Scheme) error {
	// Setup Scheme for all channel-subscription resources
	if err := apis.AddToScheme(runtimeScheme); err != nil {
		return fmt.Errorf("failed to add subscription apis to mgr scheme - %w", err)
	}
	// Setup Scheme for cluster/v1beta1
	if err := clusterv1beta1.Install(runtimeScheme); err != nil {
		return fmt.Errorf("failed to install cluster/v1beta1 scheme to mgr scheme - %w", err)
	}

	return nil
}

// AddGitStorageWalker adds the controllers that sync (/process) files from process into the DB to the Manager.
func AddGitStorageWalker(mgr ctrl.Manager, gitStorageDirPath string, specDB db.SpecDB,
	rbacAuthorizer authorizer.Authorizer, syncInterval time.Duration,
) error {
	k8sClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		return fmt.Errorf("failed to start k8s client from mgr - %w", err)
	}

	tagToSyncerMap := map[string]dbsyncer.StorageToDBSyncer{
		managedClustersGroupStorageToDBSyncerTag: dbsyncer.NewManagedClustersGroupStorageToDBSyncer(specDB,
			rbacAuthorizer),
		managedClusterSetStorageToDBSyncerTag: dbsyncer.NewManagedClusterSetStorageToDBSyncer(specDB,
			k8sClient, rbacAuthorizer),
	}

	if err := mgr.Add(&gitStorageWalker{
		log:            ctrl.Log.WithName("git-storage-walker"),
		k8sClient:      k8sClient,
		rootDirPath:    gitStorageDirPath,
		tagToSyncerMap: tagToSyncerMap,
		intervalPolicy: intervalpolicy.NewExponentialBackoffPolicy(syncInterval),
	}); err != nil {
		return fmt.Errorf("failed to add git-storage-walker to mgr - %w", err)
	}

	return nil
}
