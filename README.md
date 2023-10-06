# Multi Cluster Management

This repository contains manifests and scripts to bootstrap clusters with [Cluster API](https://github.com/kubernetes-sigs/cluster-api). Currently only AWS clusters are supported, but more types will be added later (EKS and GCP).

# Tech Stack

* GitOps. Cluster(s) manifests are managed by [FluxCD](https://fluxcd.io/) and the repo structure follows ["repo per team example"](https://fluxcd.io/docs/guides/repository-structure/#repo-per-team).

* Infrastruture provisioning. Deploy process follows ["Boostrap & Pivot"](https://cluster-api.sigs.k8s.io/clusterctl/commands/move.html) approach with initial temporary management cluster running on `kind`.
Flux manifests are installed on each CAPI cluster by Flux running on management cluster.
Even though it is not a recommended approach, in this project Flux is running in read-only mode (deploy key does not have write permissions).

* CNI. [cilium](https://cilium.io/), currently it is installed by script when the cluster is bootstrapped because in kube-proxy-free mode it needs to know API endpoint, and it is known only in runtime in this project current state.

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

When script is complete run script to finalize workload clusters and flux secret (design choice to avoid storing encrypted secrets in repo, and using sops that currently seems unmaintained).
This script without arguments will discover all workload clusters and perform all necessary adjustments:
```
./scripts/helpers.sh -c
```

## Adding a new cluster

To add a new cluster create config env for it by copying existing file (`./config/cluster-<num>.env`) and modifying values.
This is intended to be manual step because the values must be provided by the end user, not guessed by the system, e.g. CIDRs.

```
./scripts/helper.sh -c cluster-02
```

This will generate all necessary files and add the cluster to mgmt kustomization list too. Then it will be pushed to the repo (example commit from the script: https://github.com/olga-mir/k8s-multi-cluster/pull/10/commits/92ee7e094881969736ed666a0e732f073ebc53c6), where flux will apply it and capi will provision it. The `./scripts/helper.sh` is still waiting for the cluster to come up and finalize the installation.

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

* delete NAT gateways.
* release Elastic IP(s).
* terminate EC2 instances.
* delete VPC.
(Resrouces usually are named `<cluster-name>-<resource-type>` pattern, e.g `mgmt-nat`, `mgmt-vpc`)

Alternatively, use script `./scripts/brutal-aws-cleanup.sh` - this script deletes everything it can find (in NATs, EIPs, EC2 instances, ELBs, but not VPCs) without checking if they are related to the clusters in this project. So it is not recommended to use if there are other resources in the account.
Apparenty deleting VPC is not for the faint-hearted, check out https://github.com/isovalent/aws-delete-vpc

# Roadmap

Initially this project was created to explore Multi Cluster Services implemented with Cilium Cluster mesh. At that time I had kOps AWS cluster and I figured instead of duplicating the cluster.yaml and running the same process I'd try out CAPI. This is fun, but cluster mesh is still not implemented :D My current goal is to get to the cilium mesh asap, but utilising CAPI patterns and not sacrificing good CAPI patterns along the way.

Too much bash?

yes unfortunatelly. There are few reasons for that:
* CAPI itself is not GitOps-able at this point. It is possible to generate manifests that are applied by the `clusterctl` but it is very fragile and requires ugly patching along the way (see https://github.com/olga-mir/k8s-multi-cluster/pull/14)
* replacing aws creds with IRSA obviously not possible for `kind` cluster, but it is also problematic to do on AWS clusters if I want to keep this open source and not expose my AWS account ID (it is not "too" sensitive but better safe than sorry)
* Flux secret is also patched on the fly. It could be avoided with SOPs but I don't want to store even encrypted secrets in repo and SOPs future is not very clear.
* Some of the waits and massaging can be backed into CAPI Runtime Extentions, I'd love to get there but it all comes down to available time.

But all the bash scripts as they are now could be replaced by a nicer Go-implementation. This is again only time constraint.

Features to implement:
* Cilium cluster mesh and Gateway API
* Private clusters
* Cluster Class
* Runtime Extentions

# Resources

* https://www.weave.works/blog/manage-thousands-of-clusters-with-gitops-and-the-cluster-api
* [Weaveworks' The Power of GitOps with Flux youtube playlist](https://www.youtube.com/playlist?list=PL9lTuCFNLaD3fI_g-NXWVxopnJ0adn65d). One of the videos dedicated to CAPI
* [Cluster API Addon Provider for Helm](https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm)

