[comment]: # ( Copyright Contributors to the Open Cluster Management project )

# Hub-of-Hubs Non-K8s GitOps

[![Go Report Card](https://goreportcard.com/badge/github.com/stolostron/hub-of-hubs-nonk8s-gitops)](https://goreportcard.com/report/github.com/stolostron/hub-of-hubs-nonk8s-gitops)
[![Go Reference](https://pkg.go.dev/badge/github.com/stolostron/hub-of-hubs-nonk8s-gitops)](https://pkg.go.dev/github.com/stolostron/hub-of-hubs-nonk8s-gitops)
[![License](https://img.shields.io/github/license/stolostron/hub-of-hubs-nonk8s-gitops)](/LICENSE)

The non-k8s GitOps component of [Hub-of-Hubs](https://github.com/stolostron/hub-of-hubs).

Go to the [Contributing guide](CONTRIBUTING.md) to learn how to get involved.

## Overview
![image](https://user-images.githubusercontent.com/73340153/158602131-29bed67e-8e7c-4bcc-8a8e-b46675472d9e.png)

The Hub-of-Hubs non-k8s GitOps uses shares a volume (persistent storage) with a 
[modified version](https://github.com/vMaroon/multicloud-operators-subscription) of the 
[multicloud-operators-subscription](https://github.com/open-cluster-management-io/multicloud-operators-subscription) component,
where the subscriptions-operator is responsible for syncing Git objects via the ACM Subscriptions mechanism, 
while the non-k8s GitOps component watches the files and processes them.

## Prerequisites
### Deploying the Shared Volume
1. Set the `NODE_HOSTNAME` to the hostname of the node (e.g., `ip-10-0-136-193`) that the storage, nonk8s-gitops and the 
customized operator will run on:
    ```
    $ export NODE_HOSTNAME=...
    ```

2. Run the following command to deploy the `hoh-gitops-pv` PersistentVolume and the `hoh-gitops-pv-claim` PersistentVolumeClaim 
that uses claims it to your hub of hubs cluster:
    ```
    envsubst < deploy/hub-of-hubs-gitops-pv.yaml | kubectl apply -f -
    ```
    
### Deploying the customized Subscriptions Operator

#### Deploying the modified Subscription CRD
    kubectl -n open-cluster-management apply -f deploy/customized-subscriptions-operator/apps.open-cluster-management.io_subscriptions_crd_v1.yaml

#### Deploying the modified operator
The subscriptions operator deployment is managed by the [ACM for Kubernetes Operator](https://console-openshift-console.apps.mayoub-hoh2.scale.red-chesterfield.com/k8s/ns/open-cluster-management/operators.coreos.com~v1alpha1~ClusterServiceVersion/advanced-cluster-management.v2.4.2). To have the latter deploy the customized version, modify the "multicluster-operators-standalone-subscription" deployment 
to that present in [standalone-subscriptions-operator-deployment.yaml](deploy/customized-subscriptions-operator/standalone-subscriptions-operator-deployment.yaml).

The modified code has small modifications of the upstream stable release of the operator in [Open Cluster Management](https://github.com/open-cluster-management-io) organization,
therefore it is forked to a [personal Git](https://github.com/vMaroon/multicloud-operators-subscription).

### Creating the namespace for accessible Subscription CRs
    kubectl create namespace hoh-subscriptions

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

Set the `DATABASE_URL` according to the PostgreSQL URL format: `postgres://YourUserName:YourURLEscapedPassword@YourHostname:5432/YourDatabaseName?sslmode=verify-full&pool_max_conns=50`.

Remember to URL-escape the password, you can do it in bash:

    python -c "import sys, urllib as ul; print ul.quote_plus(sys.argv[1])" 'YourPassword'


1.  Create a secret with your database url:

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

1.  Run the following command to deploy the `hub-of-hubs-nonk8s-gitops` to your hub of hubs cluster:
    ```
    envsubst < deploy/hub-of-hubs-nonk8s-gitops.yaml.template | kubectl apply -f -
    ```

## Cleanup from the hub of hubs

1.  Run the following command to clean `hub-of-hubs-nonk8s-gitops` from your hub of hubs cluster:
    ```
    envsubst < deploy/hub-of-hubs-nonk8s-gitops.yaml.template | kubectl delete -f -
    ```
