package yamltypes

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

// NewManagedClustersGroupFromBytes unmarshals a byte slice into a ManagedClustersGroup.
func NewManagedClustersGroupFromBytes(data []byte) (*ManagedClustersGroup, error) {
	managedClustersGroup := &ManagedClustersGroup{}

	if err := yaml.Unmarshal(data, managedClustersGroup); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml - %w", err)
	}

	return managedClustersGroup, nil
}

// ManagedClustersGroup implements the API for a ManagedClustersGroup.
type ManagedClustersGroup struct {
	// Kind is kind of yaml.
	Kind string `yaml:"kind"`
	// ManagedClustersGroupMetadata is the metadata of a ManagedClustersGroup.
	Metadata ManagedClustersGroupMetadata `yaml:"metadata"`
	// ManagedClustersGroupSpec is the spec of a ManagedClustersGroup.
	Spec ManagedClustersGroupSpec `yaml:"spec"`
}

// ManagedClustersGroupMetadata is the metadata of a ManagedClustersGroup.
type ManagedClustersGroupMetadata struct {
	// Name of the clusters group.
	Name string `yaml:"name"`
}

// ManagedClustersGroupSpec is the spec of a ManagedClustersGroup. The spec contains identifiers of MCs to be tagged
// with the cluster group.
type ManagedClustersGroupSpec struct {
	// TagValue is the value that will be assigned to the group label's key.
	TagValue string `yaml:"tagValue"`
	// Identifiers of the managed clusters.
	Identifiers []map[string]HubIdentifier `yaml:"identifiers"`
}

// HubIdentifier identifies managed clusters within a specific hub.
type HubIdentifier struct {
	// Name of the hub.
	Name string `yaml:"name"`
	// ManagedClusterIDs is an array of MC identifiers.
	ManagedClusterIDs []string `yaml:"managedClusterIdentifiers"`
}
