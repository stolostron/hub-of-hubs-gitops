package hub

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	datatypes "github.com/open-cluster-management/hub-of-hubs-data-types"
	hubv1 "github.com/open-cluster-management/hub-of-hubs-data-types/apis/config/v1"
	"github.com/stolostron/hub-of-hubs-hub-lifecycle-management/pkg/helpers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	hubLogName = "hub-lifecycle-management-controller"
)

// AddHubController creates a new instance of hub controller and adds it to the manager.
func AddHubController(mgr ctrl.Manager) error {
	hubOfHubsConfigCtrl := &hubOfHubsHubController{
		client: mgr.GetClient(),
		log:    ctrl.Log.WithName(hubLogName),
	}

	hohNamespacePredicate := predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetNamespace() == datatypes.HohSystemNamespace
	})

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&hubv1.Hub{}).
		WithEventFilter(hohNamespacePredicate).
		Complete(hubOfHubsConfigCtrl); err != nil {
		return fmt.Errorf("failed to add hub controller to the manager - %w", err)
	}

	return nil
}

type hubOfHubsHubController struct {
	client client.Client
	log    logr.Logger
}

// Reconcile reconciles Hub CR.
func (c *hubOfHubsHubController) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	reqLogger := c.log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	hub := &hubv1.Hub{}
	if err := c.client.Get(ctx, request.NamespacedName, hub); err != nil {
		reqLogger.Info(fmt.Sprintf("Reconciliation failed: %s", err))
		return ctrl.Result{Requeue: true, RequeueAfter: helpers.RequeuePeriod},
			fmt.Errorf("reconciliation failed: %w", err)
	}

	// TODO: work
	reqLogger.Info("Reconciliation complete.")

	return ctrl.Result{}, nil
}
