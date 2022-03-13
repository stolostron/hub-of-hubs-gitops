package dbsyncer

import (
	set "github.com/deckarep/golang-set"
)

// createSetFromSlice returns a set contains all items in the given slice. if slice is nil, returns empty set.
func createSetFromSlice(slice []string) set.Set {
	if slice == nil {
		return set.NewSet()
	}

	result := set.NewSet()

	for _, item := range slice {
		result.Add(item)
	}

	return result
}
