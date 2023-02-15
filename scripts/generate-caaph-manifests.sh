#!/bin/bash
set -eoux pipefail

echo "MUST RUN FROM kubernetes-sigs/cluster-api-addon-provider-helm REPOSITORY!"

REGISTRY=$1
IMG=$2

MANIFESTS_FILE="manifests.yaml"

cat > $MANIFESTS_FILE << EOF
# This manifest is generated from https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm
# Apache License 2.0: https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm/blob/main/LICENSE
# You must regenerate to use your own image
EOF
echo "---" >> $MANIFESTS_FILE
cd config/manager && kustomize edit set image controller=${REGISTRY}/${IMG}
cd ../../ && kustomize build config/default >> $MANIFESTS_FILE
