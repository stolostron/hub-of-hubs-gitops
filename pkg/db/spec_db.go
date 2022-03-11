package db

import (
	"context"

	set "github.com/deckarep/golang-set"
)

// SpecDB is the needed interface for the db transport bridge.
type SpecDB interface {
	// UpdateManagedClustersSetLabel receives a map of hub -> set of managed clusters and updates their labels to be
	// appended by the given group label (value is hohTag). // TODO: allow retaining multiple group label tags.
	UpdateManagedClustersSetLabel(ctx context.Context, tableName string, labelKey string,
		hubToManagedClustersMap map[string]set.Set) error
	// Stop stops db and releases resources (e.g. connection pool).
	Stop()
}
