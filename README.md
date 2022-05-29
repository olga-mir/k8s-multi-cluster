# Multi Cluster Patterns

This repository explores multi cluster technologies and is primarily based on [Cluster API](https://github.com/kubernetes-sigs/cluster-api) for declarative cluster management and [FLuxCD](https://fluxcd.io/) for GitOps workflows.

Other projects used:
* [Kong OSS k8s ingress controller](https://docs.konghq.com/kubernetes-ingress-controller/)
* [Kubernetes Cluster Federation](https://github.com/kubernetes-sigs/kubefed/)

This is not a complete production-ready pattern rather iterative approach to get from distinct quick-start guides to a state where multiple technologies are integrated together to achieve powerful multi-cluster workflows and patterns.

Deviations from the quick starts are improvements around least privilege principle, preference to "as Code" approach opposed to cli commands, cost-optimization, etc.

Flux repositories structure follows ["Repo per team"](https://fluxcd.io/docs/guides/repository-structure/#repo-per-team) approach.
Cluster API deployment follows ["Boostrap & Pivot"](https://cluster-api.sigs.k8s.io/clusterctl/commands/move.html) approach with initial temporary management cluster running on `kind`. At the moment it is not entirely clear to me how to manage the permanent management cluster.

My previous experiment with permanent management cluster bootstraped by kOps with workload cluster applied by FluxCD: https://github.com/olga-mir/k8s/releases/tag/v0.0.1

# Installation

Detailed process for installing a permanent management cluster and a workload cluster can be found in [docs/bootstrap-and-pivot.md](docs/bootstrap-and-pivot.md)
Script: [deploy-bootstrap-cluster.sh](./scripts/deploy-bootstrap-cluster.sh), followed by [install-flux.sh](./scripts/install-flux.sh)

# Cleanup

Always delete `cluster` objects from management cluster(s) first (`k delete cluster`), otherwise cloud resources are not deleted.
If the cluster resource was not deleted, or if deletion got tangled up in errors clean up resources manually to avoid charges:
* delete NAT gateway.
* release Elastic IP(s).
* delete VPC.
* terminate EC2 instances.
(Resrouces usually are named `<cluster-name>-<resource-type>` pattern, e.g `mgmt-nat`, `mgmt-vpc`)

# Resources

* https://www.weave.works/blog/manage-thousands-of-clusters-with-gitops-and-the-cluster-api
* [Weaveworks' The Power of GitOps with Flux youtube playlist](https://www.youtube.com/playlist?list=PL9lTuCFNLaD3fI_g-NXWVxopnJ0adn65d). One of the videos dedicated to CAPI
