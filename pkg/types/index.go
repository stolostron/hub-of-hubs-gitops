package yamltypes

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

// NewIndex unmarshals a byte slice into an Index.
func NewIndex(data []byte) (*Index, error) {
	index := &Index{}

	if err := yaml.Unmarshal(data, index); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml - %w", err)
	}

	return index, nil
}

// Index maps other types to files for the git-walker to distributed to processors.
type Index struct {
	TypeToDirs []map[string][]string `yaml:"types"`
}
