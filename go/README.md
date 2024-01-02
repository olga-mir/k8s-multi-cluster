# Multi Cluster Demo

## Purpose

MultiCluster-Demo is a Kubernetes management application designed to demo the deployment and operation of multiple Kubernetes clusters. This tool leverages Cluster API for cluster lifecycle management and FluxCD for simplifying installation of basic kubernetes components.

The purpose of this project is to allow to build flexible cluster topologies and run experiments with them

## Features

Cluster Management: Automate the creation and management of Kubernetes clusters using Cluster API.
GitOps Integration: Utilize FluxCD for GitOps-based continuous delivery and configuration management.
Multi-Cluster Support: Manage multiple Kubernetes clusters, both for development and production environments.
Flexible Configuration: Define cluster configurations and operational scenarios via YAML files.
Cilium CNI with advanced features: Cilium Mesh, Gateway and more

The project has 2 main parts to it - `build` and `run`.

`build` builds desired cluster topology, as defined in [./go/config.yaml](./go/config.yaml)
When the clusters are ready, then `run` can perform implemented scenarios. e.g. immutable cluster upgrade, cluster failover and so on.

## Prerequisites

Most config for the project is denifed in config files. 

- [./go/config.yaml](./go/config.yaml): Custom config file specific for this project. 
- [templates/clusterctl.yaml](../templates/clusterctl.yaml): Cluster API config file. Not implemented yet.

Other data that can't be committed to public repo, but required for the project is stored in environment variables. Following variables must be set:

- `K8S_MULTI_KUBECONFIG`: path to kubeconfig file, configs will be added and removed from this file so make sure there is no clash with existing names or provide a designated empty config for this project.
- `AWS_B64ENCODED_CREDENTIALS`: if using AWS then provide credentials. This is required for Cluster API.
- `FLUXCD_KEY_PATH`: Path to SSH key for FluxCD. Currently the same key is shared among all clusters. Key per cluster will be implemented in future.

## Usage

Project is managed with `Taskfile`

- Build the project:

```bash
$ task build-app
```

- Build the clusters (this task will re-build the app if necessary):

```bash
$ task run-demo-build
```

- Run scenarios

NOT IMPLEMENTED YET

```bash
$ task run-demo-run
```