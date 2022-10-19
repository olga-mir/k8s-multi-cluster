# Multi Cluster Management

This repository contains manifests and scripts to bootstrap clusters with [Cluster API](https://github.com/kubernetes-sigs/cluster-api). Currently only AWS clusters are supported, but more types will be added later (EKS and GCP).

# Tech Stack

* GitOps. Cluster(s) manifests are managed by [FLuxCD](https://fluxcd.io/) and the repo structure follows ["repo per team example"](https://fluxcd.io/docs/guides/repository-structure/#repo-per-team).

* Infrastruture provisioning. Deploy process follows ["Boostrap & Pivot"](https://cluster-api.sigs.k8s.io/clusterctl/commands/move.html) approach with initial temporary management cluster running on `kind`. Flux manifests are installed on each CAPI cluster using `ClusterResourceSet` (although this feature may become deprecated in future). Flux manifests are pre-generated and packaged as CRS ConfigMaps, flux is running in read-only mode (deploy key does not have write permissions).

* CNI. [cilium](https://cilium.io/), currently it is installed by script when the cluster is bootstrapped because in kube-proxy-free mode it needs to know API endpoint, and it is known only in runtime in this project current state.

* Ingress Controller. [Kong OSS k8s](https://docs.konghq.com/kubernetes-ingress-controller/)

# Installation

## One Time Setup

Create CAPI IAM user. This will ensure the least privilege principle and give the ability to audit CAPI requests separately.
Refer to [aws/README.md](aws/README.md) for more details what required for initial AWS setup.

Setup workload clusters config as described in [config/README.md](config/README.md). Workload clusters can be set and removed on the go, they don't need to exist before running the deploy script.

More details on deploy process can be found here: [docs/bootstrap-and-pivot.md](docs/bootstrap-and-pivot.md)

## Deploy

deploy permanent management cluster on AWS (using temp `kind` cluster and then pivot)
```
./scripts/deploy.sh
```
:warning: for each cluster which is deployed during this script a kubeconfig is merged to `$HOME/.kube/config` preserving any entries that previously existed there. There is no control to change this, but a backup saved to `$HOME/.kube/config-$(date +%F_%H_%M_%S)` just in case.

flux on management cluster will apply CAPI manifests that are currently present in the repo.

When script is complete run script to finalize workload clusters (install cilium which currently is not vi CRS - due to dynamic KAS address) and flux secret (WIP to eliminate this step).
This script without arguments will discover all workload clusters and perform all necessary adjustments:
```
./scripts/workload-cluster.sh
```

## Adding a new cluster

Hands free with just one command!

To add a new cluster create config env for it by copying existing file (`./config/cluster-<num>.env`) and modifying values. This is intended to be manual as script can't or shouldn't guess this values (or too difficult in bash like calc next CIDR)

```
./scripts/workload-cluster.sh -n cluster-02
```

This will generate all necessary files and add the cluster to mgmt kustomization list too. Then it will be pushed to the repo (example commit from the script: https://github.com/olga-mir/k8s-multi-cluster/pull/10/commits/92ee7e094881969736ed666a0e732f073ebc53c6), where flux will apply it and capi will provision it. The `./scripts/workload-cluster.sh` is still waiting for the cluster to come up and finalize the installation.

on mgmt cluster:
```
% k get cluster -A
NAMESPACE      NAME           PHASE          AGE   VERSION
cluster-01     cluster-01     Provisioned    12m
cluster-02     cluster-02     Provisioning   60s
cluster-mgmt   cluster-mgmt   Provisioned    13m
```

# Cleanup

Delete clusters in clean CAPI way:
```
% ./scripts/cleanup.sh
```
The script will move all cluster definitions, including mgmt cluster (which at this point is hosted on the mgmt cluster itself) to the `kind` cluster and delete them in parallel.

When CAPI way is not working for some reasons (bugs), then you need to delete AWS resources that make up the clusters to avoid charges.

* delete NAT gateway.
* release Elastic IP(s).
* delete VPC.
* terminate EC2 instances.
(Resrouces usually are named `<cluster-name>-<resource-type>` pattern, e.g `mgmt-nat`, `mgmt-vpc`)

Alternatively, use script `./scripts/brutal-aws-cleanup.sh` - this script deletes everything it can find (in NATs, EIPs, EC2 instances, ELBs, but not VPCs) without checking if they are related to the clusters in this project. So it is not recommended to use if there are other resources in the account.

# Resources

* https://www.weave.works/blog/manage-thousands-of-clusters-with-gitops-and-the-cluster-api
* [Weaveworks' The Power of GitOps with Flux youtube playlist](https://www.youtube.com/playlist?list=PL9lTuCFNLaD3fI_g-NXWVxopnJ0adn65d). One of the videos dedicated to CAPI
