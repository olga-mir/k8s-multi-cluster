# Detailed workflow

## Create IAM resources

Create required IAM resources: [./aws](aws/README.md)
Note that this step is not required if following official start guide, more on why this is implemented differently below.

## Temporary management cluster

tl;dr - run [script](../scripts/deploy-bootstrap-cluster.sh)

We'll use `kind` as a shortlived temp cluster which will be used to spin up permanent management cluster on AWS. This approach is known as "bootstrap and pivot"

By default `kind` creates a cluster with one control plane node and no worker nodes. Use following config to create at least one worker node:
```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: worker
```

Then create the cluster and initialise it as a management cluster:
```
kind create cluster --config bootstrap.yaml # assuming above config is saved in bootstrap.yaml
clusterctl init --infrastructure aws
```

## Generate config for permanent management cluster

Nest step is generating and applying a "CAPI" cluster which will be the permanent management cluster.

There is a few environment variables that need to be configured before generating manifest.
The list can be obtained by running `clusterctl generate cluster --list-variables aws`.
Store it in a separate file, provide your values  and source it.

```
export AWS_CONTROL_PLANE_MACHINE_TYPE=...
export AWS_NODE_MACHINE_TYPE=...
export AWS_REGION=...
export AWS_SSH_KEY_NAME=...
export KUBERNETES_VERSION=..

# optional:
export CLUSTER_NAME=mgmt
export CONTROL_PLANE_MACHINE_COUNT=1
export WORKER_MACHINE_COUNT=1
```

[CAPI quick-start guide](https://cluster-api.sigs.k8s.io/user/quick-start.html) instructs to run these two commands:
1. clusterawsadm bootstrap iam create-cloudformation-stack
2. export AWS_B64ENCODED_CREDENTIALS=$(clusterawsadm bootstrap credentials encode-as-profile)

If IAM resources have been created using cloudformation stack then there is no need to do the first command
for the credentials part I prefer to create Access Key creds for the CapiUser (created as part of aws cloudformation stacks)
and then manually encode them. Create aws temp profile file:
```
[default]
aws_access_key_id = ...
aws_secret_access_key = ...
region = ...

```
and then `export AWS_B64ENCODED_CREDENTIALS=$(cat creds.txt | base64 -)`

This base64 encoded value will be stored in k8s secret `capa-manager-bootstrap-credentials` in `capa-system` namespace.

Generate cluster config:
```
clusterctl generate cluster mgmt > mgmt.yaml
```

By default CAPA will create private/public subnets in 3 AZs (this will require 3 NATs, and 3 Elastic IPs).
To avoid this set limit on number of AZs as described in the [control plane doc](https://cluster-api-aws.sigs.k8s.io/topics/failure-domains/control-planes.html)
in the generated cluster manifest find AWSCluster resource and add following:
```
spec:
  network:
    vpc:
      availabilityZoneUsageLimit: 1
```

Apply mgmt.yaml and validate:
```
k get clusters
k get kubeadmcontrolplane
```
Control plane will not be available until CNI is installed https://cluster-api.sigs.k8s.io/user/quick-start.html
```
clusterctl get kubeconfig mgmt > target-mgmt.kubeconfig
k --kubeconfig=./target-mgmt.kubeconfig -f https://docs.projectcalico.org/v3.21/manifests/calico.yaml
```

CAPI cluster should be ready now:
```
$ k get cluster
NAME   PHASE         AGE     VERSION
mgmt   Provisioned   9m51s
$ k --kubeconfig=./target-mgmt.kubeconfig get nodes
NAME                                              STATUS   ROLES                  AGE     VERSION
ip-10-0-177-108.ap-southeast-2.compute.internal   Ready    <none>                 2m11s   v1.21.11
ip-10-0-239-128.ap-southeast-2.compute.internal   Ready    control-plane,master   3m23s   v1.21.11
```

## Pivot
```
export KUBECONFIG=./target-mgmt.kubeconfig  # from here on commands run on CAPI mgmt cluster (which is not yet a management cluster)
clusterctl init --infrastructure aws
```


