#!/bin/bash

echo THIS IS LEGACY DEPLOYMENT WAY. NEW PROCESS IS DESCRIBED IN `go/README.md`

REPO_ROOT=$(git rev-parse --show-toplevel)

# User must explicitely provide their kubeconfig and accept that it can be changed by the script
# If it's not provided a new kubeconfig file will be generated in the repo root and previous one will be delete
# In this case user must configure kubectl to work with this file
KUBECONFIG=${K8S_MULTI_KUBECONFIG}

set -eoux pipefail

tempdir=$(mktemp -d)
trap 'exit_handler $? $LINENO' EXIT
echo $tempdir >> $REPO_ROOT/tempdirs.txt


main() {

set +x
. ${REPO_ROOT}/config/shared.env
. ${REPO_ROOT}/config/cluster-mgmt.env
set -x

# For more details about install process please check `<repo_root>/docs/bootstrap-and-pivot.md
# For config setup check out `<repo_root>/config/README.md`

set +x
if [ -z "$AWS_B64ENCODED_CREDENTIALS" ] && \
   [ -z "$FLUX_KEY_PATH" ]; then
  echo "Error required env variables are not set" && exit 1
fi
set -x

cat > $tempdir/kind-bootstrap.yaml << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: worker
EOF

set +u
if [ -z "$KUBECONFIG" ]; then
  KUBECONFIG=$REPO_ROOT/.kubeconfig
  echo "kubeconfig path was not provided, make sure to use this kubeconfig in your k commands: $KUBECONFIG"
  rm -f $KUBECONFIG
fi
set -u

############## ------ Setup kubeconfig ------

# cleanup entries from previous runs: https://github.com/olga-mir/k8s-multi-cluster/issues/18
# don't delete the whole file, a user maybe using their own kubeconfig
MGMT_CLUSTER_NAME=cluster-mgmt
set +e
kubectl config delete-user $MGMT_CLUSTER_NAME-admin
kubectl config delete-cluster $MGMT_CLUSTER_NAME
kubectl config delete-context $MGMT_CLUSTER_NAME-admin@$MGMT_CLUSTER_NAME
set -e

CONTEXT_MGMT="$MGMT_CLUSTER_NAME-admin@$MGMT_CLUSTER_NAME"
KUBECTL_MGMT="kubectl --kubeconfig $KUBECONFIG --context $CONTEXT_MGMT"
KUBECTL_TMP_MGMT="kubectl --kubeconfig $KUBECONFIG --context kind-kind"


kind create cluster --config $tempdir/kind-bootstrap.yaml --kubeconfig=$KUBECONFIG

# Install Flux. Flux is running in RO mode, and manifests are pre-generated.
# If this path doesn't exist, try upgrading to latest version:
# set the version in shared.env file, then run `./scripts/upgrade-components.sh`
$KUBECTL_TMP_MGMT apply -f $REPO_ROOT/k8s-platform/flux/v$FLUXCD_VERSION/gotk-components.yaml

$KUBECTL_TMP_MGMT create secret generic flux-system -n flux-system \
  --from-file identity=$FLUX_KEY_PATH  \
  --from-file identity.pub=$FLUX_KEY_PATH.pub \
  --from-literal known_hosts="$GITHUB_KNOWN_HOSTS"

set +e
while ! $KUBECTL_TMP_MGMT wait crd kustomizations.kustomize.toolkit.fluxcd.io --for=condition=Established --timeout=5s; do sleep 5; done
$KUBECTL_TMP_MGMT wait crd gitrepositories.source.toolkit.fluxcd.io --for=condition=Established --timeout=10s
set -e

# This has to be applied separately because it depends on CRDs that were created in gotk-components.
$KUBECTL_TMP_MGMT apply -f $REPO_ROOT/clusters/tmp-mgmt/flux-system/gotk-sync.yaml

clusterctl init \
  --kubeconfig=$KUBECONFIG \
  --kubeconfig-context=kind-kind \
  --core cluster-api:$CAPI_VERSION \
  --bootstrap kubeadm:$CAPI_VERSION \
  --control-plane kubeadm:$CAPI_VERSION \
  --infrastructure aws

############## ------ on AWS mgmt cluster ------

# save a copy, just in case
cp $KUBECONFIG ${KUBECONFIG}-$(date +%F_%H_%M_%S)

# kubeconfig is available when this secret is ready: `k get secret mgmt-kubeconfig`
echo $(date '+%F %H:%M:%S') - Waiting for permanent management cluster kubeconfig to become available
sleep 90
while ! clusterctl --kubeconfig $KUBECONFIG --kubeconfig-context kind-kind get kubeconfig cluster-mgmt -n cluster-mgmt > $tempdir/kubeconfig; do
  echo $(date '+%F %H:%M:%S') re-try in 25s... && sleep 25
done

KUBECONFIG=$KUBECONFIG:$tempdir/kubeconfig kubectl config view --raw=true --merge=true > $tempdir/merged-config
chmod 600 $tempdir/merged-config
mv $tempdir/merged-config $KUBECONFIG

set +e
echo $(date '+%F %H:%M:%S') - Waiting for permanent management cluster to become responsive
while [ -z $($KUBECTL_MGMT get pod -n kube-system -l component=kube-apiserver -o name) ]; do sleep 15; done
set -e

clusterctl init \
  --kubeconfig $KUBECONFIG \
  --kubeconfig-context $CONTEXT_MGMT \
  --core cluster-api:$CAPI_VERSION \
  --bootstrap kubeadm:$CAPI_VERSION \
  --control-plane kubeadm:$CAPI_VERSION \
  --infrastructure aws

# Flux on mgmt cluster is installed by Flux on tmp-mgmt cluster in clusters/tmp-mgmt/cluster-mgmt/platform.yaml
$KUBECTL_MGMT create secret generic flux-system -n flux-system \
  --from-file identity=$FLUX_KEY_PATH  \
  --from-file identity.pub=$FLUX_KEY_PATH.pub \
  --from-literal known_hosts="$GITHUB_KNOWN_HOSTS"

echo $(date '+%F %H:%M:%S')
set +e
while ! $KUBECTL_MGMT wait crd clusters.cluster.x-k8s.io --for=condition=Established; do sleep 15; done
set -e

flux --kubeconfig $KUBECONFIG --context kind-kind suspend kustomization flux-system

clusterctl move --kubeconfig $KUBECONFIG --kubeconfig-context kind-kind --to-kubeconfig=$KUBECONFIG --to-kubeconfig-context $CONTEXT_MGMT -n cluster-mgmt

# At this stage `kind` cluster can be safely deleted. Later a new temp cluster can be created to move the permanent
# management cluster to. But for the purpose of this project just keep this cluster running. It will be used later
# to delete mgmt and all workload clusters in parallel
# kind delete cluster

# To finalize workload clusters bootstrap follow `$REPO_ROOT/scripts/helper.sh -h` instructions

} # end main

exit_handler() {
  set +x
  if [ "$1" != "0" ]; then
    echo "LINE: $2 ERROR: $1"
  fi
  rm -rf $tempdir
}

main
