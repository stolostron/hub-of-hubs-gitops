package controller

import (
	"fmt"

	clustersv1 "github.com/open-cluster-management/api/cluster/v1"
	hubv1 "github.com/open-cluster-management/hub-of-hubs-data-types/apis/config/v1"
	"github.com/stolostron/hub-of-hubs-hub-lifecycle-management/pkg/controller/hub"
	"github.com/stolostron/hub-of-hubs-hub-lifecycle-management/pkg/controller/kafka"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// AddToScheme adds all Resources to the Scheme.
func AddToScheme(runtimeScheme *runtime.Scheme) error {
	// add cluster scheme
	if err := clustersv1.Install(runtimeScheme); err != nil {
		return fmt.Errorf("failed to add scheme: %w", err)
	}

	schemeBuilders := []*scheme.Builder{
		hubv1.SchemeBuilder,
	} // add schemes

	for _, schemeBuilder := range schemeBuilders {
		if err := schemeBuilder.AddToScheme(runtimeScheme); err != nil {
			return fmt.Errorf("failed to add scheme: %w", err)
		}
	}

	return nil
}

// AddControllers adds all the controllers to the Manager.
func AddControllers(mgr ctrl.Manager) error {
	addControllerFunctions := []func(ctrl.Manager) error{
		hub.AddHubController, kafka.AddKafkaUserController,
	}

	for _, addControllerFunction := range addControllerFunctions {
		if err := addControllerFunction(mgr); err != nil {
			return fmt.Errorf("failed to add controller: %w", err)
		}
	}

	return nil
}
