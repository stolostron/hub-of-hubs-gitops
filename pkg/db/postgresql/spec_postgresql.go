package postgresql

import (
	"context"

	set "github.com/deckarep/golang-set"
)

// UpdateManagedClustersSetLabel receives a map of hub -> set of managed clusters and updates their labels to be
// appended by the given group label. // TODO: allow retaining multiple group label tags.
func (p *PostgreSQL) UpdateManagedClustersSetLabel(ctx context.Context, tableName string, label string,
	hubToManagedClustersMap map[string]set.Set,
) error {
	return nil
}
