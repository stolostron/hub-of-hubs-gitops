package db

import (
	"context"

	set "github.com/deckarep/golang-set"
)

const (
	// ManagedClusterSetDefaultTagValue is the value used for labels that are of kind tag.
	ManagedClusterSetDefaultTagValue = "true"
	// HubOfHubsGroup is the group name that prefixes hoh items.
	HubOfHubsGroup = "hub-of-hubs.open-cluster-management.io"
)

// SpecDB is the needed interface for nonk8s-gitops DB related functionality.
type SpecDB interface {
	ManagedClusterLabelsSpecDB
	// Stop stops db and releases resources (e.g. connection pool).
	Stop()
}

// ManagedClusterLabelsSpecDB is the interface needed by the spec transport bridge to sync managed-cluster labels table.
type ManagedClusterLabelsSpecDB interface {
	// UpdateManagedClustersSetLabel receives a map of hub -> set of managed clusters and updates their labels to be
	// appended by the given group tag label.
	//
	// If the operation fails, hubToManagedClustersMap will contain un-synced entries only.
	UpdateManagedClustersSetLabel(ctx context.Context, tableName string, labelKey string, labelValue string,
		hubToManagedClustersMap map[string]set.Set) error
	// Stop stops db and releases resources (e.g. connection pool).
	Stop()
}

// ManagedClusterLabelsState wraps the information that define a managed-cluster labels state.
type ManagedClusterLabelsState struct {
	LabelsMap        map[string]string
	DeletedLabelKeys []string
}
