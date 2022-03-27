package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	set "github.com/deckarep/golang-set"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/db"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/intervalpolicy"
)

const (
	optimisticConcurrencyRetriesCount = 5
	retryInterval                     = 5 * time.Second
)

var (
	errDidNotSyncAllEntries              = errors.New("failed to sync all entries")
	errOptimisticConcurrencyUpdateFailed = errors.New("zero rows were affected by an optimistic concurrency update")
)

// UpdateLabelForManagedClusters receives a map of hub -> set of managed clusters and updates their labels to be
// appended by the given label
//
// If the operation fails, hubToManagedClustersMap will contain un-synced entries only.
func (p *PostgreSQL) UpdateLabelForManagedClusters(ctx context.Context, tableName string, labelKey string,
	labelValue string, hubToManagedClustersMap map[string]set.Set,
) error {
	intervalPolicy := intervalpolicy.NewExponentialBackoffPolicy(retryInterval)
	retryAttempts := optimisticConcurrencyRetriesCount
	// update labels for each managed cluster separately under optimistic concurrency control with exponential backoff
	// for retries
	for retryAttempts > 0 {
		for hubName, managedClustersSet := range hubToManagedClustersMap {
			for _, managedClusterName := range managedClustersSet.ToSlice() {
				clusterName, ok := managedClusterName.(string)
				if !ok {
					p.log.Info("bad cast", "cluster", managedClusterName)
					continue
				}

				if err := p.updateLabels(ctx, hubName, clusterName, labelKey, labelValue); err != nil {
					p.log.Error(err, "failed to update labels for cluster", "hub", hubName, "cluster", clusterName,
						"label", labelKey)
					continue
				}
				// succeeded with update, remove from set
				managedClustersSet.Remove(managedClusterName)
				// if set is empty then remove leaf hub from entry
				if len(managedClustersSet.ToSlice()) == 0 {
					delete(hubToManagedClustersMap, hubName)
				}
			}
		}

		if len(hubToManagedClustersMap) == 0 {
			break // all synced
		}

		intervalPolicy.Evaluate()
		time.Sleep(intervalPolicy.GetInterval())

		retryAttempts--
	}

	if len(hubToManagedClustersMap) != 0 { // some failed
		return errDidNotSyncAllEntries
	}

	return nil
}

func (p *PostgreSQL) updateLabels(ctx context.Context, hubName string, cluster string, labelKey string,
	labelValue string,
) error {
	rows, err := p.conn.Query(ctx,
		"SELECT labels, deleted_label_keys, version from spec.managed_clusters_labels WHERE leaf_hub_name = $1 AND "+
			"managed_cluster_name = $2", hubName, cluster)
	if err != nil {
		return fmt.Errorf("failed to read from managed_clusters_labels: %w", err)
	}
	defer rows.Close()

	if labelValue == "" {
		labelValue = db.ManagedClusterSetDefaultTagValue
	}

	labelsToAdd := map[string]string{labelKey: labelValue}

	if !rows.Next() { // insert the labels
		_, err := p.conn.Exec(ctx,
			`INSERT INTO spec.managed_clusters_labels (leaf_hub_name, managed_cluster_name, labels, version, updated_at) 
				values($1, $2, $3::jsonb, 0, now())`,
			hubName, cluster, labelsToAdd)
		if err != nil {
			return fmt.Errorf("failed to insert into the managed_clusters_labels table: %w", err)
		}

		return nil
	}

	var (
		currentLabelsToAdd         map[string]string
		currentLabelsToRemoveSlice []string
		version                    int64
	)

	err = rows.Scan(&currentLabelsToAdd, &currentLabelsToRemoveSlice, &version)
	if err != nil {
		return fmt.Errorf("failed to scan a row: %w", err)
	}

	// every label that is not prefixed by hohGroup should be dropped
	labelsToRemove := map[string]struct{}{}

	for key, value := range currentLabelsToAdd {
		if labelKeyIsAllowed(key) {
			labelsToAdd[key] = value // label should be retained
			continue
		}

		labelsToRemove[key] = struct{}{}
	}

	err = p.putRow(ctx, cluster, labelsToAdd, currentLabelsToAdd, labelsToRemove, p.getMap(currentLabelsToRemoveSlice),
		version)
	if err != nil {
		return fmt.Errorf("failed to update managed_clusters_labels table: %w", err)
	}

	return nil
}

func (p *PostgreSQL) putRow(ctx context.Context, cluster string, labelsToAdd map[string]string,
	currentLabelsToAdd map[string]string, labelsToRemove map[string]struct{}, currentLabelsToRemove map[string]struct{},
	version int64,
) error {
	newLabelsToAdd := make(map[string]string)
	newLabelsToRemove := make(map[string]struct{})

	for key := range currentLabelsToRemove {
		if _, keyToBeAdded := labelsToAdd[key]; !keyToBeAdded {
			newLabelsToRemove[key] = struct{}{}
		}
	}

	for key := range labelsToRemove {
		newLabelsToRemove[key] = struct{}{}
	}

	for key, value := range currentLabelsToAdd {
		if _, keyToBeRemoved := labelsToRemove[key]; !keyToBeRemoved {
			newLabelsToAdd[key] = value
		}
	}

	for key, value := range labelsToAdd {
		newLabelsToAdd[key] = value
	}

	commandTag, err := p.conn.Exec(ctx,
		`UPDATE spec.managed_clusters_labels SET
		labels = $1::jsonb,
		deleted_label_keys = $2::jsonb,
		version = version + 1,
		updated_at = now()
		WHERE managed_cluster_name=$3 AND version=$4`,
		newLabelsToAdd, p.getKeys(newLabelsToRemove), cluster, version)
	if err != nil {
		return fmt.Errorf("failed to insert a row: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return errOptimisticConcurrencyUpdateFailed
	}

	return nil
}

func (p *PostgreSQL) getMap(aSlice []string) map[string]struct{} {
	mapToReturn := make(map[string]struct{}, len(aSlice))

	for _, key := range aSlice {
		mapToReturn[key] = struct{}{}
	}

	return mapToReturn
}

// from https://stackoverflow.com/q/21362950
func (p *PostgreSQL) getKeys(aMap map[string]struct{}) []string {
	keys := make([]string, len(aMap))
	index := 0

	for key := range aMap {
		keys[index] = key
		index++
	}

	return keys
}
