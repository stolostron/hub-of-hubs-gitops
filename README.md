[comment]: # ( Copyright Contributors to the Open Cluster Management project )

# Hub-of-Hubs GitOps

[![Go Report Card](https://goreportcard.com/badge/github.com/stolostron/hub-of-hubs-gitops)](https://goreportcard.com/report/github.com/stolostron/hub-of-hubs-gitops)
[![Go Reference](https://pkg.go.dev/badge/github.com/stolostron/hub-of-hubs-gitops)](https://pkg.go.dev/github.com/stolostron/hub-of-hubs-gitops)
[![License](https://img.shields.io/github/license/stolostron/hub-of-hubs-gitops)](/LICENSE)

The GitOps component of [Hub-of-Hubs](https://github.com/stolostron/hub-of-hubs).

Go to the [Contributing guide](CONTRIBUTING.md) to learn how to get involved.

## Overview
![image](https://user-images.githubusercontent.com/73340153/159141166-b6b75c28-3c4c-4d08-b70d-14f49e88b557.png)

The Hub-of-Hubs (HOH) GitOps component shares a volume (persistent storage) with a 
[modified version](https://github.com/vMaroon/multicloud-operators-subscription) of the 
[multicloud-operators-subscription](https://github.com/open-cluster-management-io/multicloud-operators-subscription) operator,
where the subscriptions-operator is responsible for syncing Git objects via the ACM Subscriptions mechanism, 
while the HOH GitOps component watches the files and processes them to provide support for customized gitops / non-k8s gitops.

Disclaimers: 
* The component was implemented to demonstrate the mechanism. It is not tested for scale and can use 
optimizations such as parallelized storage-walking / parallelized DB job handling.

## Prerequisites
### Deploying the Shared Volume
1. Set the `GITOPS_NODE_HOSTNAME` to the hostname of the node (e.g., `ip-10-0-136-193`) that the storage, HOH-gitops and the 
customized operator will run on:
    ```
    $ export GITOPS_NODE_HOSTNAME=$(kubectl get node --selector='node-role.kubernetes.io/worker' -o=jsonpath='{.items[0].metadata.labels.kubernetes\.io\/hostname}')
    ```

2. Run the following command to deploy the `hoh-gitops-pv` PersistentVolume and the `hoh-gitops-pv-claim` PersistentVolumeClaim 
that claims it to your hub of hubs cluster:
    ```
    envsubst < deploy/hub-of-hubs-gitops-pv.yaml | kubectl apply -f -
    ```
    
### Deploying the customized Subscriptions Operator

#### Deploying the modified Subscription CRD
    kubectl -n open-cluster-management apply -f deploy/customized-subscriptions-operator/apps.open-cluster-management.io_subscriptions_crd_v1.yaml

#### Creating the namespace to easily track custom Subscription CRs (used in [examples](examples))
    kubectl create namespace hoh-subscriptions

#### Deploying the modified operator
The subscriptions operator deployment is managed by the [ACM for Kubernetes Operator](https://console-openshift-console.apps.mayoub-hoh2.scale.red-chesterfield.com/k8s/ns/open-cluster-management/operators.coreos.com~v1alpha1~ClusterServiceVersion/advanced-cluster-management.v2.4.2). To have the latter deploy the customized version, modify the "multicluster-operators-standalone-subscription" deployment 
to that present in [standalone-subscriptions-operator-deployment.yaml](deploy/customized-subscriptions-operator/operators-subscriptions-deployments-patch.yaml).

The modified code has small modifications of the upstream stable release of the operator in [Open Cluster Management](https://github.com/open-cluster-management-io) organization,
therefore it is forked to a [personal Git](https://github.com/vMaroon/multicloud-operators-subscription).

1. Set the `MODIFIED_OPERATOR_IMAGE` environment variable to hold the URL of the image:
    ```
    $ export MODIFIED_OPERATOR_IMAGE=quay.io/maroonayoub/multicloud-operators-subscription@sha256:1c57e1e77ea3c929c7176681d5b64eca43354bbaf00aeb7f7ddb01d3c6d15ad0
    ```
1. Patch the ACM for K8s operator:
   ```
   kubectl -n open-cluster-management patch ClusterServiceVersion advanced-cluster-management.v2.4.2 --type=merge --patch "$(envsubst < deploy/customized-subscriptions-operator/operators-subscriptions-deployments-patch.yaml)"
   ```

### Visit [examples](examples) for example Subscription deployments / Git objects

## Getting Started

## Build and push the image to docker registry

1.  Set the `REGISTRY` environment variable to hold the name of your docker registry:
    ```
    $ export REGISTRY=...
    ```

1.  Set the `IMAGE_TAG` environment variable to hold the required version of the image.  
    default value is `latest`, so in that case no need to specify this variable:
    ```
    $ export IMAGE_TAG=latest
    ```

1.  Run make to build and push the image:
    ```
    $ make push-images
    ```

## Deploy on the hub of hubs

1. If the `hub-of-hubs-database-transport-bridge-secret` does not exist:
   1. Set the `DATABASE_URL` according to the PostgreSQL URL format: `postgres://YourUserName:YourURLEscapedPassword@YourHostname:5432/YourDatabaseName?sslmode=verify-full&pool_max_conns=50`.
   Remember to URL-escape the password, you can do it in bash:
      ```
      python -c "import sys, urllib as ul; print ul.quote_plus(sys.argv[1])" 'YourPassword'
      ```

   1. Create a secret with your database url:
       ```
       kubectl create secret generic hub-of-hubs-database-transport-bridge-secret -n open-cluster-management --from-literal=url=$DATABASE_URL
       ```

1.  Set the `REGISTRY` environment variable to hold the name of your docker registry:
    ```
    $ export REGISTRY=...
    ```

1.  Set the `IMAGE` environment variable to hold the name of the image.

    ```
    $ export IMAGE=$REGISTRY/$(basename $(pwd)):latest
    ```
    
1.  Run the following command to give the `hub-of-hubs-gitops` service account "privileged" security context constraint permissions:
    ```
    oc adm policy add-scc-to-user privileged -z hub-of-hubs-gitops -n open-cluster-management
    ```

1.  Run the following command to deploy the `hub-of-hubs-gitops` to your hub of hubs cluster:
    ```
    envsubst < deploy/hub-of-hubs-gitops.yaml.template | kubectl apply -f -
    ```

## Cleanup from the hub of hubs

1.  Run the following command to clean `hub-of-hubs-gitops` from your hub of hubs cluster:
    ```
    envsubst < deploy/hub-of-hubs-gitops.yaml.template | kubectl delete -f -
    ```

1.  Run the following command to remove the "privileged" security context constraint permissions from `hub-of-hubs-gitops` service account :
    ```
    oc adm policy remove-scc-from-user privileged -z hub-of-hubs-gitops -n open-cluster-management
    ```
    
1.  If you wish to revert the ACM for K8s operator's customization, run the following:
    ```
    kubectl -n open-cluster-management patch ClusterServiceVersion advanced-cluster-management.v2.4.2 \ 
       --type=merge --patch $(cat deploy/customized-subscriptions-operator/revert-operators-subscriptions-deployments-patch.yaml)
    ```

1.  Finally, delete PV and PVC:
    ```
    kubectl -n open-cluster-management delete -f deploy/hub-of-hubs-gitops-pv.yaml
    ```
