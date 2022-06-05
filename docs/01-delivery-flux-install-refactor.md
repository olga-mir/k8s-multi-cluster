# FluxCD on temporary cluster and Flux install on permanent clusters using CRS

## Intro

Capture decision points why certain things were done in certain ways and document delivery plan.

## Goals

* remove manual install of resources on the temporary mgmt cluster by using flux on that cluster too.
* replace `flux bootstrap` with `flux install` so that deploy keys with write permissions are not used.
* install flux by using output of `flux isnstall` as input for CAPI `ClusterResourceSet`

This should speed up cluster creation, simplify deploy script, remove manual step of ack giving permission to flux to write to repo during `flux bootstrap` command execution, remove the need for deploy keys with write perms (at the cost of externally managing flux version upgrades, e.g in [flux GA](https://github.com/fluxcd/flux2/tree/main/action#automate-flux-updates)

## Non-Goals

Important things that closely related to this delivery that need to be done, but descoped for simplicity for first iteration:

* generate CAPI cluster config. Continue using hard coded manifests, that were generated initially.
* kustomize CAPI cluster.yaml configs for workload clusters. This will be main selling point of using CAPI for cluster canary/lifecycle management, but it is a lot of work at this stage.
* rename cluster-dev to "identity-less" cluster in a certaing environment for blue/green deployments.
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

Remove the ./mgmt-cluster and the script steps that deploy those resources. Init config files will still be needed as CAPI/CAPA itself is not managed by CRS.
tmp cluster should get its place in the `./clusters` dir and have instance of flux pointing to it.
Create `pre-build` script that downloads CNI and Flux manifests and packages them in ConfigMaps for CRS to consume. They will be pre-built and committed to the repo for speed and history trail. (in the future building these CMs will be done in the CI for component upgrades)

