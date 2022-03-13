package postgresql

import (
	"context"
	"fmt"

	set "github.com/deckarep/golang-set"
)

// GetAccessibleManagedClusters gets a map of hub -> set { managed-clusters } that are accessible with the given
// filter clause (WHERE ...).
func (p *PostgreSQL) GetAccessibleManagedClusters(ctx context.Context, tableName string,
	filterClause string,
) (map[string]set.Set, error) {
	hubToManagedClustersMap := map[string]set.Set{}

	rows, _ := p.conn.Query(ctx, fmt.Sprintf(`SELECT leaf_hub_name, payload->'metadata'->>'name' FROM status.%s WHERE 
TRUE AND %s`, tableName, filterClause))

	defer rows.Close()

	for rows.Next() {
		var (
			hubName            string
			managedClusterName string
		)

		if err := rows.Scan(&hubName, &managedClusterName); err != nil {
			return nil, fmt.Errorf("error reading from table status.%s - %w", tableName, err)
		}

		clustersSet, found := hubToManagedClustersMap[hubName]
		if !found {
			clustersSet = set.NewSet()
			hubToManagedClustersMap[hubName] = clustersSet
		}

		clustersSet.Add(managedClusterName)
	}

	return hubToManagedClustersMap, nil
}
