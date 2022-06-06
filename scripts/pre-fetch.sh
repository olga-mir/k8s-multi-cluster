#!/bin/bash
# https://pkg.go.dev/sigs.k8s.io/cluster-api@v1.1.4/exp/addons/api/v1alpha3#ClusterResourceSetSpec

set -eoux pipefail
workdir=$(pwd)
temp_dir=$(mktemp -d)
echo Temp dir currently is not removed in the end for debug purposes. $temp_dir

# For now every cluster use the same version of addons.
CILIUM_VERSION="1.11.5"
FLUXCD_VERSION="0.30.1"
crs_cm_cilium_file=$workdir/crs-cm-cilium-${CILIUM_VERSION}.yaml

main() {
  ### Cilium
  helm repo add cilium https://helm.cilium.io

  # disable Hubble because otherwise the manifest contains secrets with CAs and private keys.
  # I don't understand this now and I don't want to commit clear text secret manifests or deal with secrets right now.
  helm template cilium cilium/cilium --version $CILIUM_VERSION --namespace kube-system --set hubble.enabled=false > $temp_dir/cilium-${CILIUM_VERSION}.yaml

  kubectl create configmap crs-cm-cilium-${CILIUM_VERSION} --from-file=$temp_dir/cilium-${CILIUM_VERSION}.yaml --dry-run=client -o yaml > $workdir/infrastructure/base/crs-cm-cilium-${CILIUM_VERSION}.yaml

  ## Flux
  # This script can be used for upgrading flux, not only installing it, so
  # flux-system folder and the gotk-sync files are created outside of this script
  flux install --version=$FLUXCD_VERSION --export > $temp_dir/gotk-components.yaml

  cp $temp_dir/gotk-components.yaml $workdir/clusters/tmp-mgmt/flux-system
  cp $temp_dir/gotk-components.yaml $workdir/clusters/staging/mgmt/flux-system
  cp $temp_dir/gotk-components.yaml $workdir/clusters/staging/blue/flux-system

  # tmp management cluster is the only one that gets flux as gotk-components manifests
  # all other clusters are created by CAPI and will have flux manifests packaged inside a CRS

  # ok, this feels silly, but this is how I understand it now:
  # flux is running in read-only mode (by choice. can't use bootstrap if installing via CRS?)
  # but it syncs from a repo which needs to include flux-system too.
  # so flux manifests are stored twice - once as plain manifest at the final state path
  # and once as a payload inside a configmap for the CRS.

  flux_yaml=$temp_dir/flux-combined.yaml
  cp $workdir/clusters/staging/mgmt/flux-system/gotk-components.yaml $flux_yaml
  echo "---" >> $flux_yaml
  cat $workdir/clusters/staging/mgmt/flux-system/gotk-sync.yaml >> $flux_yaml

  # now we can put this in CM. (k create cm accepts --from-<whatever> multiple times,
  # but it creates a separate data entry for each occurence, that's why concatenating file was necessary
  kubectl create configmap crs-cm-flux-${FLUXCD_VERSION}-mgmt --from-file=$flux_yaml -n cluster-mgmt --dry-run=client -o yaml > $workdir/infrastructure/base/crs-cm-flux-mgmt.yaml
  # (this should not be base path - cluster specific things are baked into CM payload and are not accessible to kustomize. facepalm emoji goes here)
}

cleanup() {
  if [ "$1" != "0" ]; then
    echo "Line $2: Error $1"
  fi
  #rm -rf $temp_dir
}

trap 'cleanup $? $LINENO' EXIT
main
