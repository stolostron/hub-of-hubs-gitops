package postgresql

import (
	"context"

	set "github.com/deckarep/golang-set"
)

// GetAccessibleManagedClusters gets a map of hub -> set { managed-clusters } that are accessible with the given
// filter clause (WHERE ...).
func (p *PostgreSQL) GetAccessibleManagedClusters(ctx context.Context, tableName string,
	filterClause string,
) (map[string]set.Set, error) {
	return map[string]set.Set{}, nil // TODO: implement
}
