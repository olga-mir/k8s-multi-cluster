# Cluster API Addon Provider for Helm (CAAPH)

https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm

https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm/blob/main/docs/quick-start.md

# Generate manifests

Clone CAAPH repo, build & push image to your registry according to quick start instructions.

`deploy` target from the CAAPH repo's Makefile deploys all the necessary manifests. But for this project we need manifests stored in git so that they can be picked up by GitOps. 
There is currently no official image provided by CAAPH so you'll need to build your own and then generate the manifests. Generating manifests can be done by creating a "dry-run" version of the `deploy` target from the CAAPH repo (remove `kubectl apply` part) or run this script **from kubernetes-sigs/cluster-api-addon-provider-helm repo root**

`./scripts/generate-caaph-manifests.sh <your-registry> <your-image-with-tag>`
