# Examples
In this example we're going to deploy subscriptions that sync nonk8s objects found in 
[nonk8s-resources](git-objects/nonk8s-resources) and k8s-native resources found in [nonk8s-resources](git-objects/k8s-resources)

#### Familiarity with ACM's Channel/Subscription logic is assumed.

---
## Deploy the Channel / Subscription resources:
For k8s resources, subscription-admin permissions might be required, refer to
[examples when Subscription Admin needs to be enabled in RHACM-Gitops scenarios](https://access.redhat.com/solutions/6010251).

Note: the article states that policies cannot be deployed starting from ACM 2.4, BUT, the cluster-role permissions can be modified.

Run:
```
kubectl apply -f subscriptions
```

---
## Applied Resources
### Channel
The channel is a regular ACM channel that points to this repository
```
apiVersion: apps.open-cluster-management.io/v1
kind: Channel
metadata:
  name: hoh-gitops
  namespace: hoh-subscriptions
spec:
    type: Git
    pathname: https://github.com/stolostron/hub-of-hubs-gitops
```

### Subscriptions:
#### k8s-native resources using regular ACM subscription:
```
apiVersion: apps.open-cluster-management.io/v1
kind: Subscription
metadata:
  name: hoh-gitops-k8s-resources-subscription
  namespace: default
  annotations:
    apps.open-cluster-management.io/git-path: examples/git-objects/k8s-resources
    apps.open-cluster-management.io/github-branch: main
spec:
  channel: hoh-subscriptions/hoh-gitops
  name: hub-of-hubs-gitops
  placement:
    local: true
```
#### non-k8s resources using modified subscriptions:
The customized Subscription is extended with `spec.placement.hubOfHubsGitOps` field to hold the path to an index file.
The index file is `{repo-path}/{annotations.git-path}/{spec.placement.hubOfHubsGitOps}`.

The `spec.placement.local` field has to be set to true when the above field is set, otherwise the Subscription will be ignored.

```
apiVersion: apps.open-cluster-management.io/v1
kind: Subscription
metadata:
  name: hoh-gitops-mcgroup-subscription
  namespace: hoh-subscriptions
  annotations:
    # apps.open-cluster-management.io/git-path: examples
    apps.open-cluster-management.io/github-branch: main
spec:
  channel: hoh-subscriptions/hoh-gitops
  name: hub-of-hubs-gitops
  placement:
    local: true
    hubOfHubsGitOps: examples/index.yaml # directory could be specified in git-path or here
    # files in index are processed relative to its path
```
---
## Index File
The index file can be placed anywhere and called anything.
The index file maps processor-tags to work-directories relative to its location.
```
kind: Index
types: # map of processor tags to (multiple) dirs relative to the index's filepath.
  - HubOfHubsManagedClusterSet:
    - git-objects/nonk8s-resources/managed-cluster-set # dir
  - ManagedClustersGroup:
    - git-objects/nonk8s-resources/managed-clusters-group # dir
```

## Git Objects
Git objects can be k8s resources that are pulled and applied to the cluster via regular subscriptions, e.g.:
```
apiVersion: cluster.open-cluster-management.io/v1alpha1
kind: Placement
metadata:
  name: placement-policy-etcdencryption-gitops-hohset
  namespace: default
spec:
  predicates:
    - requiredClusterSelector:
        labelSelector:
          matchLabels:
            vendor: Kind
```

Additionally, they can be custom yamls representing nonk8s resources that specific hub-of-hubs-gitops processors can handle.
Such resources can be found in [nonk8s-resources](git-objects/k8s-resources) and their API is present in 
[pkg/types](../pkg/types).

For example:
```
kind: ManagedClustersGroup # not a k8s resource, but the formatting is intentionally similar.
metadata:
  name: west-region-group # name of group
spec:
  tagValue: 'true'
  identifiers: # can contain multiple hub-identifier entries
    - hubIdentifier:
        name: hub3 # hub name
        managedClusterIdentifiers: # currently, MCs are identified by name.
          - cluster7
          - cluster8
          - cluster9
  # identified MCs will be labeled with hub-of-hubs.open-cluster-management.io/{metadata.name}={spec.tagValue}
```

Custom resources can wrap k8s resources, such as:
```
kind: HubOfHubsManagedClusterSet # not a k8s resource, but the formatting is intentionally similar.
metadata:
  name: hoh-set # will result in the deployment of a ManagedClusterSet (v1beta1) with this name
spec:
  identifiers: # can contain multiple hub-identifier entries
    - hubIdentifier:
        name: hub3 # hub name
        managedClusterIdentifiers: # currently, MCs are identified by name.
          - cluster8
          - cluster9
  # identified MCs will be labeled with cluster.open-cluster-management.io/clusterset={metadata.name}
```

which leads to the creation of the ManagedClusterSet `hoh-set`, its storing in the database and the assigning of proper labels for all identified managed-clusters.