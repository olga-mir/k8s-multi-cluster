#!/bin/bash
set -eoux pipefail

REPO_ROOT=$(git rev-parse --show-toplevel)
KUBECONFIG=${KUBECONFIG:-$HOME/.kube/config}
CONTEXT_MGMT="cluster-mgmt-admin@cluster-mgmt"
KUBECTL_MGMT="kubectl --kubeconfig $KUBECONFIG --context $CONTEXT_MGMT"

tempdir=$(mktemp -d)
trap 'exit_handler $? $LINENO' EXIT

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

kind create cluster --config $tempdir/kind-bootstrap.yaml

# Install Flux.
kubectl apply -f $REPO_ROOT/platform-components/flux/v0.38.1/gotk-components.yaml

kubectl create secret generic flux-system -n flux-system \
  --from-file identity=$FLUX_KEY_PATH  \
  --from-file identity.pub=$FLUX_KEY_PATH.pub \
  --from-literal known_hosts="$GITHUB_KNOWN_HOSTS"

set +e
while ! kubectl wait crd kustomizations.kustomize.toolkit.fluxcd.io --for=condition=Established --timeout=5s; do sleep 5; done
kubectl wait crd gitrepositories.source.toolkit.fluxcd.io --for=condition=Established --timeout=10s
set -e

# This has to be applied separately because it depends on CRDs that were created in gotk-components.
kubectl apply -f $REPO_ROOT/clusters/tmp-mgmt/gotk-sync.yaml

clusterctl init \
  --core cluster-api:$CAPI_VERSION \
  --bootstrap kubeadm:$CAPI_VERSION \
  --control-plane kubeadm:$CAPI_VERSION \
  --infrastructure aws

############## ------ on AWS mgmt cluster ------

# Backup original kubeconfig file
cp $HOME/.kube/config $HOME/.kube/config-$(date +%F_%H_%M_%S)

# kubeconfig is available when this secret is ready: `k get secret mgmt-kubeconfig`
echo $(date '+%F %H:%M:%S') - Waiting for permanent management cluster kubeconfig to become available
sleep 90
while ! clusterctl get kubeconfig cluster-mgmt -n cluster-mgmt > $tempdir/kubeconfig; do
  echo $(date '+%F %H:%M:%S') re-try in 25s... && sleep 25
done

KUBECONFIG=$HOME/.kube/config:$tempdir/kubeconfig kubectl config view --raw=true --merge=true > $tempdir/merged-config
chmod 600 $tempdir/merged-config
mv $tempdir/merged-config $HOME/.kube/config

set +e
echo $(date '+%F %H:%M:%S') - Waiting for permanent management cluster to become responsive
while [ -z $($KUBECTL_MGMT get pod -n kube-system -l component=kube-apiserver -o name) ]; do sleep 15; done
set -e

clusterctl init --kubeconfig $KUBECONFIG --kubeconfig-context $CONTEXT_MGMT \
  --core cluster-api:$CAPI_VERSION \
  --bootstrap kubeadm:$CAPI_VERSION \
  --control-plane kubeadm:$CAPI_VERSION \
  --infrastructure aws

$KUBECTL_MGMT create secret generic flux-system -n flux-system \
  --from-file identity=$FLUX_KEY_PATH  \
  --from-file identity.pub=$FLUX_KEY_PATH.pub \
  --from-literal known_hosts="$GITHUB_KNOWN_HOSTS"

echo $(date '+%F %H:%M:%S')
set +e
while ! $KUBECTL_MGMT wait crd clusters.cluster.x-k8s.io --for=condition=Established; do sleep 15; done
set -e

flux --context kind-kind suspend kustomization flux-system

clusterctl move --kubeconfig $KUBECONFIG --kubeconfig-context kind-kind --to-kubeconfig=$KUBECONFIG --to-kubeconfig-context $CONTEXT_MGMT -n cluster-mgmt

# At this stage `kind` cluster can be safely deleted. Later a new temp cluster can be created to move the permanent
# management cluster to. But for the purpose of this project just keep this cluster running. It will be used later
# to delete mgmt and all workload clusters in parallel
# kind delete cluster

# To finalize workload clusters bootstrap follow `$REPO_ROOT/scripts/workload-cluster.sh -h` instructions

} # end main

exit_handler() {
  set +x
  if [ "$1" != "0" ]; then
    echo "LINE: $2 ERROR: $1"
  fi
  rm -rf $tempdir
}

main
