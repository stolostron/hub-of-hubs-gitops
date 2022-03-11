package yamltypes

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

// NewManagedClustersSetFromBytes unmarshals a byte slice into a ManagedClusterSet.
func NewManagedClustersSetFromBytes(data []byte) (*ManagedClustersSet, error) {
	managedClustersSet := &ManagedClustersSet{}

	if err := yaml.Unmarshal(data, managedClustersSet); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml - %w", err)
	}

	return managedClustersSet, nil
}

// ManagedClustersSet implements the API for a ManagedClustersSet.
type ManagedClustersSet struct {
	// ManagedClustersSetMetadata is the metadata of a ManagedClusterSet.
	Metadata ManagedClustersSetMetadata `yaml:"metadata"`
	// ManagedClustersSetSpec is the spec of a ManagedClusterSet.
	Spec ManagedClustersSetSpec `yaml:"spec"`
}

// ManagedClustersSetMetadata is the metadata of a ManagedClusterSet.
type ManagedClustersSetMetadata struct {
	// Name of the clusters set.
	Name string `yaml:"name"`
	// Group of the clusters set.
	Group string `yaml:"group"`
}

// ManagedClustersSetSpec is the spec of a ManagedClusterSet. The spec contains identifiers of MCs to be tagged
// with the cluster set.
type ManagedClustersSetSpec struct {
	// Identifiers of the managed clusters.
	Identifiers []HubIdentifier `yaml:"identifiers"`
}

// HubIdentifier identifies managed clusters within a specific hub.
type HubIdentifier struct {
	// Name of the hub.
	Name string `yaml:"name"`
	// ManagedClusterIDs is an array of MC identifiers.
	ManagedClusterIDs []string `yaml:"managedClusterIdentifiers"`
}
