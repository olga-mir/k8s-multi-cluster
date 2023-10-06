#!/bin/bash

echo "NOTE: this script does not safely upgrade components in the cluster"
echo "it only updates the code base with updated versions (before installation)"

REPO_ROOT=$(git rev-parse --show-toplevel)


# FLUXCD_VERSION comes from the env file
# TODO: mgmt and workload clusters can have different versions
set +x
. ${REPO_ROOT}/config/shared.env
#set -x

FLUX_PATH=$REPO_ROOT/k8s-platform/flux/v$FLUXCD_VERSION
mkdir -p $FLUX_PATH
flux install --version=v$FLUXCD_VERSION --export > $FLUX_PATH/gotk-components.yaml
cat > $FLUX_PATH/kustomization.yaml << EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - gotk-components.yaml
EOF


echo "Review and push:"
echo "git add $FLUX_PATH"
echo "git commit -am \"chore: upgrade flux to v$FLUXCD_VERSION\""

echo
echo "When new version tested remove previous version folder from source control"
