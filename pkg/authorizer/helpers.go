package authorizer

import set "github.com/deckarep/golang-set"

// getDisjointEntries returns the entries in the tester sets-map that are not present in the base sets-map.
func getDisjointEntries(tester, base map[string]set.Set) map[string]set.Set {
	disjointCollection := make(map[string]set.Set, len(tester))

	for key, testerSet := range tester {
		baseSet, found := base[key]
		if !found {
			disjointCollection[key] = testerSet
			continue
		}

		disjointCollection[key] = testerSet.Difference(baseSet)
	}

	return disjointCollection
}
