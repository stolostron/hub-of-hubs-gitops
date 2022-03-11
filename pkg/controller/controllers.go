package controller

import (
	"fmt"
	"time"

	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/authorizer"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/controller/dbsyncer"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/db"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/intervalpolicy"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const managedClustersSetStorageToDBSyncerTag = "ManagedClustersSet"

// AddGitStorageWalker adds the controllers that sync (/process) files from process into the DB to the Manager.
func AddGitStorageWalker(mgr ctrl.Manager, gitStorageDirPath string, specDB db.SpecDB,
	rbacAuthorizer authorizer.Authorizer, syncInterval time.Duration,
) error {
	tagToSyncerMap := map[string]dbsyncer.StorageToDBSyncer{
		managedClustersSetStorageToDBSyncerTag: dbsyncer.NewManagedClustersSetStorageToDBSyncer(specDB, rbacAuthorizer),
	}

	k8sClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		return fmt.Errorf("failed to start k8s client from mgr - %w", err)
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
