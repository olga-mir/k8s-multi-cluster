# FluxCD on temporary cluster and Flux install on permanent clusters using ClusterResourceSet

## Intro

Capture decision points why certain things were done in certain ways and document delivery plan.

## Goals

* remove manual install of resources on the temporary mgmt cluster by using flux on that cluster too.
* replace `flux bootstrap` with `flux install` so that deploy keys with write permissions are not used.
* install flux by using output of `flux install` as input for CAPI `ClusterResourceSet`
* rename cluster-dev to "identity-less" cluster in a certaing environment for blue/green deployments.
* create a script that packages CNI and flux as CRS manifests.

This should speed up cluster creation, simplify deploy script, remove manual step of ack giving permission to flux to write to repo during `flux bootstrap` command execution, remove the need for deploy keys with write perms (at the cost of externally managing flux version upgrades, e.g in [flux GA](https://github.com/fluxcd/flux2/tree/main/action#automate-flux-updates)

## Non-Goals

Important things that closely related to this delivery that need to be done, but descoped for simplicity for first iteration:

* generate CAPI cluster config. Continue using hardcoded manifests, that were generated initially.
* kustomize CAPI cluster.yaml configs for workload clusters. This will be main selling point of using CAPI for cluster canary/lifecycle management, but it is a lot of work at this stage.
* install CAPI as a CRS on perm mgmt cluster.

## Current state

`./clusters` dir is the entry point for flux to sync on corresponding cluster.
```
clusters
├── cluster-dev
│   ├── flux-system            // root for flux installed on `cluster-dev`
│   │   ├──  ....
│   ├── infrastructure.yaml    // include/pointer to resources that "platform" team would install on cluster. In this case Kong ingress controller
│   └── tenants.yaml           // include/pointer to resources that "developer" teams would deploy on cluster. (apps, business logic)
└── mgmt
    ├── ....                   // same structure as `cluster-dev` except `infrastructure.yaml` points to a different tech stack:
                               // kubefed, capi "payload". And developer teams are not welcome here.
```

`./infrastructure` - defines sources (`HelmRepositories`) for tools installed on the clusters and HelmReleases to tell flux to install platform components on the clusters according to those clusters' roles.
```
infrastructure
├── control-plane-cluster
│   ├── capi                      // CAPI "payload": CAPI manifests describing the workload clusters, deployed in this mgmt cluster.
│   │   ├── cluster.yaml
│   │   ├── cm-calico-v3.21.yaml  // ConfigMap that contains calico manifests for CRS to consume
│   │   ├── kustomization.yaml
│   │   └── namespace.yaml
│   ├── kubefed
│   │   ├── helm-release.yaml
│   │   ├── kustomization.yaml
│   │   └── namespace.yaml
│   └── kustomization.yaml
├── sources                       // HelmRepositories
│   ├── kong.yaml
│   └── kubefed.yaml
└── workload-clusters
    ├── kong
    │   ├── helm-release.yaml
    │   ├── kustomization.yaml
    │   └── namespace.yaml
    └── kustomization.yaml
```

`./mgmt-cluster` - manual dump of all the things that are currently applied step by step by the install script.
```
./mgmt-cluster
├── README.md
├── cluster.yaml
├── cm-calico-v3.21.yaml
├── init-config-mgmt.yaml      // clusterctl init file for the AWS mgmt cluster
└── init-config-workload.yaml  // clusterctl init file for the workload cluster
```

## Next state

Remove the `./mgmt-cluster` and the script steps that deploy those resources. Init config files will still be needed as CAPI/CAPA itself is not managed by CRS.
tmp cluster should get its place in the `./clusters` dir and have instance of flux pointing to it.
Create `pre-build` script that downloads CNI and Flux manifests and packages them in ConfigMaps for CRS to consume. They will be pre-built and committed to the repo for speed and history trail. (in the future building these CMs will be done in the CI for component upgrades)

Re-structure `./clusters` dir in such a way that it allows for multiple environments defitions and multiple clusters in that enviroment. In large organisations it makes sense to have separate mgmt cluster for each env, so this will be desired structure for this iteration:

```
clusters
├── staging
│   ├── blue
│   │   ├── infrastructure.yaml
│   │   └── tenants.yaml
│   └── mgmt
│       └── infrastructure.yaml
└── tmp-mgmt
    ├── flux-system
    │   ├── gotk-components.yaml // manifests that are generated with `flux install` and committed to the repo by user (not by flux)
    │   ├── gotk-sync.yaml       // not part of `flux install`, need to be manually created and pushed to repo (this file was provided when using `flux bootstrap`)
    │   └── kustomization.yaml
    └── infrastructure.yaml      // pointer to CAPI manifests of the perm AWS management cluster.
```
Note that clusters in `./clusters/staging` do not have `flux-system` this is because flux is installed as a CRS. However, I am not sure if CRS can handle upgrade (it seems that the whole feature is actually going to be deprecated)

`blue` is a workload cluster in `staging` environment. There could be other clusters here, if there are few clusters that have different purpose then each would have its blue/green flavours. both `blue` and `green` folders would exist only during transition phase, e.g when upgrading a cluster or implementing high risk cluster features that require the safety mechanism of blue/green rollout.
This flow will be implemented in the future, but it will look a lot like this:
1. Clone environment as described in: https://github.com/fluxcd/flux2-kustomize-helm-example#identical-environments
2. make required modifications in the clone
3. deploy, test, monitor
4. phase out the old cluster.
This will require federation and external DNS and maybe more stuff in order to glue the endpoints at the very top to make seemless transition from end-user POV.
