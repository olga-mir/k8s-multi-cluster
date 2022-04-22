# Flux and Cluster API

This repo documents my journey into CAPI+Flux land, mostly following getting-started guides with some modifications. The idea is to collect commands from different guides in scripts that will make it easier to spinup the playground for experiments (and help remember "what the heck did I do last week to make it work?").

Deviations from the quick starts are improvements around least privilege principle, preference to "as Code" approach opposed to cli commands, modifications to use less resources than the defaults, etc.

Flux repositories structure follows ["Repo per team"](https://fluxcd.io/docs/guides/repository-structure/#repo-per-team) approach.
Cluster API deployment follow["Boostrap & Pivot"](https://cluster-api.sigs.k8s.io/clusterctl/commands/move.html) approach with initial temporary management cluster running on `kind`.

My previous experiment with permanent management cluster bootstraped by kOps with workload cluster applied by FluxCD: https://github.com/olga-mir/k8s/releases/tag/v0.0.1

# Installation

Detailed process for installing a permanent management cluster and a workload cluster can be found in [docs/bootstrap-and-pivot.md](docs/bootstrap-and-pivot.md)

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
