package authorizer

import (
	"context"

	set "github.com/deckarep/golang-set"
)

// Authorizer abstracts the functionality required to authorize DB ops through RBAC.
type Authorizer interface {
	// FilterManagedClustersForUser receives a map of leaf-hub -> set(managed clusters) and returns a map of
	// unauthorized entries.
	FilterManagedClustersForUser(ctx context.Context, user string, groups []string,
		hubToManagedClustersMap map[string]set.Set) (map[string]set.Set, error)
}
