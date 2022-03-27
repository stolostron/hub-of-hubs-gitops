package yamltypes

import (
	"fmt"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"

	"gopkg.in/yaml.v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

// NewManagedClusterSetFromBytes unmarshals a byte slice into a ManagedClusterSet.
func NewManagedClusterSetFromBytes(data []byte) (*ManagedClusterSet, error) {
	managedClusterSet := &ManagedClusterSet{}

	if err := yaml.Unmarshal(data, managedClusterSet); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml - %w", err)
	}

	return managedClusterSet, nil
}

// ManagedClusterSet implements the API for a ManagedClustersSet.
type ManagedClusterSet struct {
	// Kind is kind of yaml.
	Kind string `yaml:"kind"`
	// ManagedClustersSetMetadata is the metadata of a ManagedClustersGroup.
	Metadata ManagedClusterSetMetadata `yaml:"metadata"`
	// ManagedClustersSetSpec is the spec of a ManagedClustersGroup.
	Spec ManagedClusterSetSpec `yaml:"spec"`
}

// ManagedClusterSetMetadata is the metadata of a ManagedClusterSet.
type ManagedClusterSetMetadata struct {
	// Name of the clusters set.
	Name string `yaml:"name"`
}

// ManagedClusterSetSpec is the spec of a ManagedClustersGroup. The spec contains identifiers of MCs to be assigned
// with the cluster set.
type ManagedClusterSetSpec struct {
	// Identifiers of the managed clusters.
	Identifiers []map[string]HubIdentifier `yaml:"identifiers"`
}

// GetCR returns a CR object representing the set.
func (mcs *ManagedClusterSet) GetCR() *clusterv1beta1.ManagedClusterSet {
	return &clusterv1beta1.ManagedClusterSet{
		ObjectMeta: controllerruntime.ObjectMeta{
			Name: mcs.Metadata.Name,
		},
	}
}
