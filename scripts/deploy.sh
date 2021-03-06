#!/bin/bash
set -eoux pipefail

REPO_ROOT=$(git rev-parse --show-toplevel)
tempdir=$(mktemp -d)
KUBECTL_MGMT="kubectl --kubeconfig $REPO_ROOT/target-mgmt.kubeconfig --context mgmt"

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
kubectl apply -f $REPO_ROOT/clusters/tmp-mgmt/flux-system/gotk-components.yaml

kubectl create secret generic flux-system -n flux-system \
  --from-file identity=$FLUX_KEY_PATH  \
  --from-file identity.pub=$FLUX_KEY_PATH.pub \
  --from-literal known_hosts="$GITHUB_KNOWN_HOSTS"

set +e
while ! kubectl wait crd kustomizations.kustomize.toolkit.fluxcd.io --for=condition=Established --timeout=5s; do sleep 5; done
kubectl wait crd gitrepositories.source.toolkit.fluxcd.io --for=condition=Established --timeout=10s
set -e

# This has to be applied separately because it depends on CRDs that were created in gotk-components.
kubectl apply -f $REPO_ROOT/clusters/tmp-mgmt/flux-system/gotk-sync.yaml

# cluster resource for permanent management cluster and the accompanying ClusterResourceSet
# are applied by flux. When the CRS is applied the permanent cluster should be ready to use.

clusterctl init \
  --core cluster-api:$CAPI_VERSION \
  --bootstrap kubeadm:$CAPI_VERSION \
  --control-plane kubeadm:$CAPI_VERSION \
  --infrastructure aws


############## ------ on AWS mgmt cluster ------

# kubeconfig is available when this secret is ready: `k get secret mgmt-kubeconfig`
echo $(date '+%F %H:%M:%S') - Waiting for permanent management cluster kubeconfig to become available
sleep 90
while ! clusterctl get kubeconfig cluster-mgmt -n cluster-mgmt > $REPO_ROOT/target-mgmt.kubeconfig ; do
  echo $(date '+%F %H:%M:%S') re-try in 25s... && sleep 25
done

chmod go-r $REPO_ROOT/target-mgmt.kubeconfig
kubectl --kubeconfig=$REPO_ROOT/target-mgmt.kubeconfig config rename-context cluster-mgmt-admin@cluster-mgmt mgmt

set +e
echo $(date '+%F %H:%M:%S') - Waiting for permanent management cluster to become responsive
while [ -z $($KUBECTL_MGMT get pod -n kube-system -l component=kube-apiserver -o name) ]; do sleep 15; done
set -e

kas=$($KUBECTL_MGMT get pod -n kube-system -l component=kube-apiserver -o name)
sleep 10 # a little more time for IP to be set in status
export K8S_SERVICE_HOST=$($KUBECTL_MGMT get $kas -n kube-system --template '{{.status.podIP}}')
export K8S_SERVICE_PORT='6443'

# envsubst in heml values.yaml: https://github.com/helm/helm/issues/10026
envsubst < ${REPO_ROOT}/templates/cni/cilium-values-${CILIUM_VERSION}.yaml | \
  helm install cilium cilium/cilium --version $CILIUM_VERSION \
  --kubeconfig $REPO_ROOT/target-mgmt.kubeconfig \
  --namespace kube-system -f -

# check cilium setup: https://docs.cilium.io/en/v1.9/gettingstarted/k8s-install-connectivity-test/

clusterctl init --kubeconfig $REPO_ROOT/target-mgmt.kubeconfig --kubeconfig-context mgmt \
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

flux --context kind-kind suspend kustomization infrastructure

# by default `kind` creates its context in default location (~/.kube/config if $KUBECONFIG is not set)
clusterctl move --kubeconfig $HOME/.kube/config --kubeconfig-context kind-kind --to-kubeconfig=./target-mgmt.kubeconfig -n cluster-mgmt

# Now `mgmt` cluster lives on the AWS permanent management cluster:
# % k get clusters -A
# NAMESPACE      NAME   PHASE         AGE   VERSION
# cluster-dev    dev    Provisioned   50m
# cluster-mgmt   mgmt   Provisioned   56m

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
  rm -f $tempdir/kind-bootstrap.yaml
  echo Files used for this installation are stored in $tempdir for debug
  echo Remember to rm -rf if they are not needed
}

main
