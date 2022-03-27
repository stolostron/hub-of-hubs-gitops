package postgresql

import "github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/db"

const managedClusterSetLabelKey = "cluster.open-cluster-management.io/clusterset"

func labelKeyIsAllowed(key string) bool {
	legal := false

	if key == managedClusterSetLabelKey {
		legal = true
	}

	if len(key) >= len(db.HubOfHubsGroup) &&
		key[:len(db.HubOfHubsGroup)] == db.HubOfHubsGroup {
		legal = true
	}

	return legal
}
