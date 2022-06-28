#!/bin/bash
set -eoux pipefail

workdir=$(pwd)
tempdir=$(mktemp -d)
KUBECTL_MGMT="kubectl --kubeconfig $workdir/target-mgmt.kubeconfig --context mgmt"
KUBECTL_WORKLOAD="kubectl --kubeconfig $workdir/dev.kubeconfig --context dev"
CAPI_VERSION="v1.2.0-beta.0"

trap 'exit_handler $? $LINENO' EXIT

main() {

# For more details please check docs/bootstrap-and-pivot.md doc in this repo

# Provide env vars and other settings in $workdir/mgmt-cluster/init-config-mgmt.yaml file
# (note that the content of the file is not validated)
# AWS_B64ENCODED_CREDENTIALS currently accepted only from env var only.
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
kubectl apply -f $workdir/clusters/tmp-mgmt/flux-system/gotk-components.yaml

# https://github.blog/changelog/2022-01-18-githubs-ssh-host-keys-are-now-published-in-the-api/
# curl -H "Accept: application/vnd.github.v3+json" -s https://api.github.com/meta | jq -r '.ssh_keys'
# select the one that starts with "ecdsa-sha2-nistp256"
GITHUB_KNOWN_HOSTS="github.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg="
kubectl create secret generic flux-system -n flux-system \
  --from-file identity=$FLUX_KEY_PATH  \
  --from-file identity.pub=$FLUX_KEY_PATH.pub \
  --from-literal known_hosts="$GITHUB_KNOWN_HOSTS"

set +e
while ! kubectl wait crd kustomizations.kustomize.toolkit.fluxcd.io --for=condition=Established --timeout=5s; do sleep 5; done
kubectl wait crd gitrepositories.source.toolkit.fluxcd.io --for=condition=Established --timeout=10s
set -e

# This has to be applied separately because it depends on CRDs that were created in gotk-components.
kubectl apply -f $workdir/clusters/tmp-mgmt/flux-system/gotk-sync.yaml

# cluster resource for permanent management cluster and the accompanying ClusterResourceSet
# are applied by flux. When the CRS is applied the permanent cluster should be ready to use.

clusterctl init \
  --config $workdir/mgmt-cluster/init-config-mgmt.yaml \
  --core cluster-api:$CAPI_VERSION \
  --bootstrap kubeadm:$CAPI_VERSION \
  --control-plane kubeadm:$CAPI_VERSION \
  --infrastructure aws

############## ------ on AWS mgmt cluster ------


# kubeconfig is available when this secret is ready: `k get secret mgmt-kubeconfig`
echo $(date '+%F %H:%M:%S') - Waiting for permanent management cluster kubeconfig to become available
sleep 60
while ! clusterctl get kubeconfig mgmt -n cluster-mgmt > $workdir/target-mgmt.kubeconfig ; do
  echo $(date '+%F %H:%M:%S') re-try in 15s... && sleep 15
done

kubectl --kubeconfig=$workdir/target-mgmt.kubeconfig config rename-context mgmt-admin@mgmt mgmt

set +e
echo $(date '+%F %H:%M:%S') - Waiting for permanent management cluster to become responsive
while [ -z $($KUBECTL_MGMT get pod -n kube-system -l component=kube-apiserver -o name) ]; do sleep 10; done
set -e
kas=$($KUBECTL_MGMT get pod -n kube-system -l component=kube-apiserver -o name)
controlPlaneHost=$($KUBECTL_MGMT get $kas -n kube-system --template '{{.status.podIP}}')
controlPlanePort='6443'

# https://github.com/cilium/cilium/blob/master/install/kubernetes/cilium/values.yaml
CILIUM_VERSION=1.11.6
helm repo add cilium https://helm.cilium.io
helm template cilium cilium/cilium --version $CILIUM_VERSION \
    --namespace kube-system \
    --set kubeProxyReplacement=strict \
    --set k8sServiceHost=$controlPlaneHost \
    --set k8sServicePort=$controlPlanePort \
    --set ipam.mode='cluster-pool' \
    --set ipam.operator.clusterPoolIPv4PodCIDRList={192.168.0.0/16} \
    --set ipam.operator.clusterPoolIPv4MaskSize=24 \
    --set bpf.masquerade=true \
    --set bpf.hostLegacyRouting=false > $tempdir/cilium-mgmt-$CILIUM_VERSION.yaml
$KUBECTL_MGMT apply -f $tempdir/cilium-mgmt-$CILIUM_VERSION.yaml

sleep 30
# check cilium setup: https://docs.cilium.io/en/v1.9/gettingstarted/k8s-install-connectivity-test/

clusterctl init --kubeconfig $workdir/target-mgmt.kubeconfig --kubeconfig-context mgmt \
  --config $workdir/mgmt-cluster/init-config-workload.yaml \
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

############## ------ Workload cluster bootstrap ------

# Once Flux is bootstrapped on the cluster it will apply cluster-dev CAPI definition and workload cluster will start provisioning
sleep 90
while ! kubectl --kubeconfig=$workdir/target-mgmt.kubeconfig wait --context mgmt --for condition=ResourcesApplied=True clusterresourceset crs -n cluster-dev --timeout=15s ; do
  echo $(date '+%F %H:%M:%S') waiting for workload cluster to become ready
  sleep 15
done

clusterctl --kubeconfig=$workdir/target-mgmt.kubeconfig --kubeconfig-context mgmt get kubeconfig dev -n cluster-dev > $workdir/dev.kubeconfig
kubectl --kubeconfig=$workdir/dev.kubeconfig config rename-context dev-admin@dev dev

# no need to bootstrap flux, because it is applied as part of the CRS

# this is not applied as a CRS because the API server IP is known at runtime
# but babysitting each cluster in bash script defeating the purpose of CAPI.
# Alternatives:
# let it install via CRS without KAS IP and kubectl apply it once it is known
# this is stinky because it is still bash babysitting, also because it relies
# on the fact that CRS is not going to implement anything other than 'ApplyOnce'
# BYO infra including ELB and provide that ELB name for KAS location as part of CRS.
# Not tested if ELB DNS name will be good enough for `k8sServiceHost`
# and BYO infra is a lot of work
set +e
echo $(date '+%F %H:%M:%S') - Waiting for workload cluster to become responsive
while [ -z $($KUBECTL_DEV get pod -n kube-system -l component=kube-apiserver -o name) ]; do sleep 10; done
set -e
kas=$($KUBECTL_MGMT get pod -n kube-system -l component=kube-apiserver -o name)
controlPlaneHost=$($KUBECTL_DEV get $kas -n kube-system --template '{{.status.podIP}}')
controlPlanePort='6443'

helm template cilium cilium/cilium --version $CILIUM_VERSION \
    --namespace kube-system \
    --set kubeProxyReplacement=strict \
    --set k8sServiceHost=$controlPlaneHost \
    --set k8sServicePort=$controlPlanePort \
    --set ipam.mode='cluster-pool' \
    --set ipam.operator.clusterPoolIPv4PodCIDRList={192.168.0.0/16} \
    --set ipam.operator.clusterPoolIPv4MaskSize=24 \
    --set bpf.masquerade=true \
    --set bpf.hostLegacyRouting=false > $tempdir/cilium-workload-$CILIUM_VERSION.yaml
$KUBECTL_DEV apply -f $tempdir/cilium-workload-$CILIUM_VERSION.yaml

} # end main

exit_handler() {
  set +x
  if [ "$1" != "0" ]; then
    echo "LINE: $2 ERROR: $1"
  fi
  rm -f $tempdir/kind-bootstrap.yaml
  echo Files used for this installation are stored in $tempdir for debug
  echo Remember to rm -rf if they are not needed
  echo
  echo kubeconfig files:
  echo $workdir/target-mgmt.kubeconfig
  echo $workdir/dev.kubeconfig
}

main
