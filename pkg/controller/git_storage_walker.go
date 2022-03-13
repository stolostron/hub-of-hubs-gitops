package controller

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/controller/dbsyncer"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/intervalpolicy"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	appv1 "open-cluster-management.io/multicloud-operators-subscription/pkg/apis/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const hubOfHubsSubscriptionsNamespace = "hoh-subscriptions"

var (
	errSyncerTagNotFound                = fmt.Errorf("subscription's assigned syncer tag is not registered")
	errUserIdentityAnnotationNotFound   = fmt.Errorf("user-identity annotation was not found on subscription")
	errUserGroupAnnotationNotFound      = fmt.Errorf("user-group annotation was not found on subscription")
	errHubOfHubsGitopsPlacementNotFound = fmt.Errorf("hubOfHubsGitops was not set in subscription.spec.placement")
)

// gitStorageWalker watches a local git storage root (contains git repositories) and syncs entries via registered
// syncers.
type gitStorageWalker struct {
	log            logr.Logger
	k8sClient      client.Client
	rootDirPath    string
	tagToSyncerMap map[string]dbsyncer.StorageToDBSyncer
	intervalPolicy intervalpolicy.IntervalPolicy
}

func (walker *gitStorageWalker) Start(ctx context.Context) error {
	walker.init(ctx)

	go walker.periodicSync(ctx)

	<-ctx.Done() // blocking wait for cancel context event
	walker.log.Info("git storage walker", "root", walker.rootDirPath)

	return nil
}

func (walker *gitStorageWalker) init(ctx context.Context) {
	walker.log.Info("initialized git storage walker", "root", walker.rootDirPath)
	walker.syncGitRepos(ctx)
}

func (walker *gitStorageWalker) periodicSync(ctx context.Context) {
	ticker := time.NewTicker(walker.intervalPolicy.GetInterval())

	for {
		select {
		case <-ctx.Done(): // we have received a signal to stop
			ticker.Stop()
			return

		case <-ticker.C:
			// define timeout of max sync interval on the sync function
			ctxWithTimeout, cancelFunc := context.WithTimeout(ctx, walker.intervalPolicy.GetMaxInterval())
			synced := walker.syncGitRepos(ctxWithTimeout)

			cancelFunc() // cancel child ctx and is used to cleanup resources once context expires or sync is done.

			// get current sync interval
			currentInterval := walker.intervalPolicy.GetInterval()

			// notify policy whether sync was actually performed or skipped
			if synced {
				walker.intervalPolicy.Evaluate()
			} else {
				walker.intervalPolicy.Reset()
			}

			// get reevaluated sync interval
			reevaluatedInterval := walker.intervalPolicy.GetInterval()

			// reset ticker if needed
			if currentInterval != reevaluatedInterval {
				ticker.Reset(reevaluatedInterval)
				walker.log.Info(fmt.Sprintf("sync interval has been reset to %s", reevaluatedInterval.String()))
			}
		}
	}
}

func (walker *gitStorageWalker) syncGitRepos(ctx context.Context) bool {
	gitRepos, err := ioutil.ReadDir(walker.rootDirPath)
	if err != nil {
		walker.log.Error(err, "failed to open git root folder", "root-path", walker.rootDirPath)
		return false
	}

	successRate := 0 // to determine whether to evaluate or reset interval policy based on majority success/failure

	for _, gitRepo := range gitRepos {
		if !gitRepo.IsDir() {
			continue // stray file
		}

		repoFullPath := filepath.Join(walker.rootDirPath, gitRepo.Name())

		syncerTag, base64UserIdentity, base64UserGroup, err := walker.getInfoFromSubscription(ctx, gitRepo.Name())
		if err != nil {
			walker.log.Error(err, "failed to sync local git repo", "path", gitRepo.Name())
			successRate--

			continue
		}

		dbSyncer, found := walker.tagToSyncerMap[syncerTag]
		if !found {
			walker.log.Error(errSyncerTagNotFound, "failed to sync local git repo", "path", gitRepo.Name(),
				"walker-tag", syncerTag)
			successRate--

			continue
		}

		walker.log.Info("attempt sync repo", "syncer-tag", syncerTag, "path", repoFullPath)

		if dbSyncer.SyncGitRepo(ctx, base64UserIdentity, base64UserGroup, repoFullPath) {
			successRate++
		}
	}

	return successRate > 0 // majority succeeded
}

// getInfoFromSubscription opens a subscription CR and returns syncer tag (spec.placement.hubOfHubsGitOps),
// base64(user-identity), base64(user-group) and error if failed.
func (walker *gitStorageWalker) getInfoFromSubscription(ctx context.Context,
	subscriptionName string,
) (string, string, string, error) {
	subscription := &appv1.Subscription{}
	// try to get subscription
	objKey := client.ObjectKey{
		Namespace: hubOfHubsSubscriptionsNamespace,
		Name:      subscriptionName,
	}
	if err := walker.k8sClient.Get(ctx, objKey, subscription); err != nil {
		if !apierrors.IsNotFound(err) {
			return "", "", "", fmt.Errorf("subscription with name %s was not found - %w", subscriptionName, err)
		}

		return "", "", "", fmt.Errorf("failed to get subscription with name %s - %w", subscriptionName, err)
	}

	base64UserIdentity, found := subscription.Annotations[appv1.AnnotationUserIdentity]
	if !found {
		return "", "", "", errUserIdentityAnnotationNotFound
	}

	base64UserGroup, found := subscription.Annotations[appv1.AnnotationUserGroup]
	if !found {
		return "", "", "", errUserGroupAnnotationNotFound
	}

	if subscription.Spec.Placement.HubOfHubsGitOps == nil { // shouldn't happen but just for safety
		return "", "", "", errHubOfHubsGitopsPlacementNotFound
	}

	return *subscription.Spec.Placement.HubOfHubsGitOps, base64UserIdentity, base64UserGroup, nil
}
