# Flux and Cluster API

Experimenting with Flux and Cluster API

Flux repositories structure will follow ["Repo per team"](https://fluxcd.io/docs/guides/repository-structure/#repo-per-team) approach.
Cluster API deployment will follow ["Boostrap & Pivot"](https://cluster-api.sigs.k8s.io/clusterctl/commands/move.html) approach with initial temporary management cluster being `kind`

My previous experiment with permanent management cluster bootstraped by kOps: https://github.com/olga-mir/k8s/releases/tag/v0.0.1

# Installation

The installation process mainly follows the getting started guide in CAPI book, with some modifications.

Detailed process for installing a permanent management cluster and a workload cluster can be found in [docs/bootstrap-and-pivot.md](docs/bootstrap-and-pivot.md)

# Resources

* https://www.weave.works/blog/manage-thousands-of-clusters-with-gitops-and-the-cluster-api
* [Weaveworks' The Power of GitOps with Flux youtube playlist](https://www.youtube.com/playlist?list=PL9lTuCFNLaD3fI_g-NXWVxopnJ0adn65d). One of the videos dedicated to CAPI
