package db

import (
	"context"

	set "github.com/deckarep/golang-set"
)

// StatusDB is the needed interface for the db transport bridge to fetch information from status DB.
type StatusDB interface {
	// GetAccessibleManagedClusters gets a map of hub -> set { managed-clusters } that are accessible with the given
	// filter clause (WHERE ...).
	GetAccessibleManagedClusters(ctx context.Context, tableName string, filterClause string) (map[string]set.Set, error)
	// Stop stops db and releases resources (e.g. connection pool).
	Stop()
}
