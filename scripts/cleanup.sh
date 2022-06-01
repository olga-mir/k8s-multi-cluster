#!/bin/bash
set -x
workdir=$(pwd)

# This script assumes it runs on the same setup as deployed by other scripts in this repo.
# The following contexts are expected to be available in kubeconfig (use ./scripts/merge-kubeconfig.sh) to merge all in one
# CURRENT   NAME            CLUSTER     AUTHINFO     NAMESPACE
# *         dev             dev         dev-admin
#           kind-kind       kind-kind   kind-kind
#           mgmt            mgmt        mgmt-admin

if [[ -z "$ACCEPT_CLEANUP_T_AND_C" ]]; then
  echo "ERROR"
  echo "ERROR  Please make sure you understand what is being deleted by this script."
  echo "ERROR  Set ACCEPT_CLEANUP_T_AND_C to any non zero value and re-run."
  exit 1
fi

echo "---- Switching to mgmt cluster"
kubectl config use-context mgmt

echo Suspend CAPI sources reconciliation.
flux suspend kustomization infrastructure

echo $(date '+%F %H:%M:%S')
clusterctl move --to-kubeconfig=$HOME/.kube/config --to-kubeconfig-context=kind-kind -n cluster-dev
clusterctl move --to-kubeconfig=$HOME/.kube/config --to-kubeconfig-context=kind-kind -n cluster-mgmt

echo "---- Switching to 'kind' cluster"
kubectl config use-context kind-kind
kubectl delete cluster mgmt -n cluster-mgmt &
kubectl delete cluster dev -n cluster-dev &

echo $(date '+%F %H:%M:%S')
sleep 600

while kubectl get cluster mgmt -n cluster-mgmt; do
  sleep 60
done

echo $(date '+%F %H:%M:%S')
while kubectl get cluster dev -n cluster-dev; do
  sleep 30
done

kind delete cluster
