package kafkauser

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/open-cluster-management/hub-of-hubs-kafka-transport/apis/strimzi-operator/kafka-user/v1beta2"
	"github.com/stolostron/hub-of-hubs-hub-lifecycle-management/pkg/helpers"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	logName        = "kafkauser-controller"
	kafkaNamespace = "kafka-user"
)

// AddKafkaUserController creates a new instance of KafkaUser controller and adds it to the manager.
func AddKafkaUserController(mgr ctrl.Manager) error {
	kafkaUserCtrl := &kafkaUserController{
		client: mgr.GetClient(),
		log:    ctrl.Log.WithName(logName),
	}

	kafkaNamespacePredicate := predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetNamespace() == kafkaNamespace
	})

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.KafkaUser{}).
		WithEventFilter(kafkaNamespacePredicate).
		Complete(kafkaUserCtrl); err != nil {
		return fmt.Errorf("failed to add hub controller to the manager - %w", err)
	}

	return nil
}

type kafkaUserController struct {
	client client.Client
	log    logr.Logger
}

// Reconcile reconciles Hub CR.
func (c *kafkaUserController) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	reqLogger := c.log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	kafkaUser := &v1beta2.KafkaUser{}
	if err := c.client.Get(ctx, request.NamespacedName, kafkaUser); apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		reqLogger.Info(fmt.Sprintf("Reconciliation failed: %s", err))
		return ctrl.Result{Requeue: true, RequeueAfter: helpers.RequeuePeriod},
			fmt.Errorf("reconciliation failed: %w", err)
	}

	// TODO: work
	reqLogger.Info("Reconciliation complete.")

	return ctrl.Result{}, nil
}
