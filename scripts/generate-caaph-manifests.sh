#!/bin/bash

REPO_ROOT=$(git rev-parse --show-toplevel)

REGISTRY=$1
IMG=$2

MANIFESTS_FILE="manifests.yaml"

cat > $MANIFESTS_FILE << EOF
# This manifests are generated from
# Apache License 2.0: https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm/blob/main/LICENSE
EOF

echo "---" >> $MANIFESTS_FILE
cd config/manager && kustomize edit set image controller=${REGISTRY}/${IMG}
cd ../../ && kustomize build config/default >> $MANIFESTS_FILE
