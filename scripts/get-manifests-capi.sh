#!/bin/bash
set -eoux pipefail

REPO_ROOT=$(git rev-parse --show-toplevel)

# version is hardcoded in kustomization file.
capi_bundle=$REPO_ROOT/infrastructure/base/capi-bundle-v1.3.0.yaml
echo "# This file is generated from Cluster API manifests https://github.com/kubernetes-sigs/cluster-api
# with Feature Gates environment variables replaced.
# LICENSE: https://github.com/kubernetes-sigs/cluster-api/blob/1eedede7730c7d8f7f7247f4f2d5b63fc5b4f545/LICENSE" > $capi_bundle
kustomize build --load-restrictor=LoadRestrictionsNone $REPO_ROOT/scripts/capi >> $capi_bundle

export AWS_B64ENCODED_CREDENTIALS="Cg=="
capa_bundle=$REPO_ROOT/infrastructure/base/capa-bundle-v1.5.1.yaml
echo "# This file is generated from Cluster API AWS provider manifests
# LICENSE: https://github.com/kubernetes-sigs/cluster-api-provider-aws/blob/ef5baabb770749c300c2bc893585b6f2d96d07ee/LICENSE" > $capa_bundle
clusterctl generate provider --infrastructure aws:v1.5.1 | yq e '(. | select(.kind != "Secret"))' >> $capa_bundle


# currently there is no GitOps way to install CAPI.
# CAPI release manifests contain templated values that are `envsubst` by clusterctl
# crafting patch is manual and makes feature-flags toggling obscure and in
# general is error-prone for upgrades.
# there is a lot of demand for GitOps'ing CAPI ecosystem installation and many people DIY'ed
#
# https://kubernetes.slack.com/archives/C8TSNPY4T/p166617731500927

# NOTE: CAPI version is HARDCODED, there are no guarantees for upgrading the version manifests.


# Another way is to use releases, but it is not straight forward either, there is no way to query by semver.
# Get 'latest' and work our the version from the url paths.
# release_id is not semver: https://docs.github.com/en/rest/releases/releases#get-a-release
# urls=$(curl -sL https://api.github.com/repos/kubernetes-sigs/cluster-api/releases/latest | jq -r '.assets[].browser_download_url' | grep -E ".*yaml$")
# urls will be in the form
# https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.2.4/bootstrap-components.yaml
# among the files there will be some misc that don't need to be applied to cluster like `metadata.yaml`, `clusterclass-quick-start.yaml`
# and a few more. Files that are CAPI k8s resources and `cluster-api-components.yaml` which is the aggregate of all CAPI resources in one manifest
# It has feature gates in it and can't be applied as is to the cluster (hence that complecated kustomization patch above ^)
