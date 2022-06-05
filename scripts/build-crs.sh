#!/bin/bash
# https://pkg.go.dev/sigs.k8s.io/cluster-api@v1.1.4/exp/addons/api/v1alpha3#ClusterResourceSetSpec

set -eoux pipefail
workdir=$(pwd)

# For now every cluster use the same version of addons.
CILIUM_VERSION="1.11.5"
crs_cm_cillium_file=$workdir/crs-cm-cilium-${CILIUM_VERSION}.yaml

main() {
  ### Cilium
  helm repo add cilium https://helm.cilium.io

  # disable Hubble because otherwise the manifest contains secrets with CAs and private keys.
  # I don't understand this now and I don't want to commit clear text secret manifests or deal with secrets right now.
  helm template cilium cilium/cilium --version $CILIUM_VERSION --namespace kube-system --set hubble.enabled=false > $workdir/cilium-${CILIUM_VERSION}.yaml

  kubectl create configmap crs-cm-cilium-${CILIUM_VERSION} --from-file=$workdir/cilium-${CILIUM_VERSION}.yaml --dry-run=client -o yaml > $workdir/infrastructure/crs-base/crs-cm-cillium-${CILIUM_VERSION}.yaml

  ## Flux
  #flux install --export > $workdir/flux-system.yaml
  #generate_sync $workdir/tmp-mgmt-flux-sync.yaml "./clusters/staging/mgmt"
}

generate_sync() {
  output=$1
  sync_path=$2
  echo Generate sync manifest with sync path $sync_path storing to $output

  # This manifest is produced by flux if using `git bootstrap`.
  # Currently this project uses `flux install` so we need to provide this
  cat > $output << EOF
---
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: GitRepository
metadata:
  name: flux-system
  namespace: flux-system
spec:
  interval: 1m0s
  ref:
    branch: $FLUX_BRANCH
  secretRef:
    name: flux-system
  url: $FLUX_REPO_SSH
---
apiVersion: kustomize.toolkit.fluxcd.io/v1beta2
kind: Kustomization
metadata:
  name: flux-system
  namespace: flux-system
spec:
  interval: 1m0s
  path: $sync_path
  prune: true
  sourceRef:
    kind: GitRepository
    name: flux-system
EOF
}

cleanup() {
  if [ "$1" != "0" ]; then
    echo "Line $2: Error $1"
  fi
#   rm $workdir/cilium-${CILIUM_VERSION}.yaml
#   rm $workdir/crs-cm-cilium-${CILIUM_VERSION}.yaml
}

trap 'cleanup $? $LINENO' EXIT
main
