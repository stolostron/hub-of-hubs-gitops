# Examples
In this example we're going to deploy a Subscription that syncs a repository containing two ManagedClustersGroup objects.
#### Familiarity with ACM's Channel/Subscription logic is assumed.

## Subscription
The examples below refer to a concrete Git repository. You may change the Channel's spec.pathname to any of yours.
### Deploy the Channel:
The channel is a regular ACM channel that points to a repository
```
apiVersion: apps.open-cluster-management.io/v1
kind: Channel
metadata:
  name: test-repo-git
  namespace: hoh-subscriptions
spec:
    type: Git
    pathname: https://github.com/vMaroon/test-repo
```
Run:
```
    kubectl apply -f subscriptions/01-channel.yaml
```

### Deploy the Subscription:
The Subscription is extended with `spec.placement.hubOfHubsGitOps` field to hold the type of a nonk8s-gitops processor.

The `spec.placement.local` field has to be set to true when the above field is set, otherwise the Subscription will be ignored.
```
apiVersion: apps.open-cluster-management.io/v1
kind: Subscription
metadata:
  name: git-test-repo-subscription
  namespace: hoh-subscriptions
  annotations:
    apps.open-cluster-management.io/github-branch: main
spec:
  channel: hoh-subscriptions/test-repo-git
  name: test-repo
  placement:
    local: true
    hubOfHubsGitOps: ManagedClustersGroup
```
Run:
```
    kubectl apply -f subscriptions/01-channel.yaml
```

## Git Object
Currently, the only supported type of Git object is ManagedClustersGroup, which groups a set of Managed-Clusters.

Example object:
```
kind: ManagedClustersGroup # not a k8s resource, but the formatting is intentionally similar.
metadata:
  name: east-region-group # name of group
spec:
  tagValue: 'true'
  identifiers: # can contain multiple hub-identifier entries
    - hubIdentifier:
        name: hub3 # hub name
        managedClusterIdentifiers: # currently, MCs are identified by name.
          - cluster5
          - cluster6
          - cluster7
    - hubIdentifier:
        name: hub4
        managedClusterIdentifiers:
          - cluster9
  # identified MCs will be labeled with hub-of-hubs.open-cluster-management.io/{metadata.name}={spec.tagValue}
```