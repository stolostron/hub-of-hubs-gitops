package dbsyncer

import (
	"bufio"
	"fmt"
	"os"

	set "github.com/deckarep/golang-set"
	"gopkg.in/yaml.v2"
)

var errInvalidGitFile = fmt.Errorf("invalid git yaml file - first line should be \"kind: ...\"")

// getGitYamlKind returns kind of yaml file, which is the value mapped to "kind" in yaml.
func getGitYamlKind(file *os.File) (string, error) {
	kindWrapper := struct {
		kind string `yaml:"kind"`
	}{}

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		// first line must be kind: ...
		if err := yaml.Unmarshal(scanner.Bytes(), &kindWrapper); err != nil {
			return "", fmt.Errorf("failed to unmarshal first line in file to yaml - %w", err)
		}

		if kindWrapper.kind == "" {
			return "", errInvalidGitFile
		}

		return kindWrapper.kind, nil
	}

	return "", errInvalidGitFile // no scans
}

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
