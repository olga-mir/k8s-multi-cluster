#!/bin/bash
set -x
workdir=$(pwd)

# This script assumes it runs on the same setup as deployed by other scripts in this repo.
# The following contexts are expected to be available in kubeconfig (use ./scripts/merge-kubeconfig.sh) to merge all in one

# % k config get-contexts
# CURRENT   NAME              CLUSTER     AUTHINFO     NAMESPACE
#           dev-admin@dev     dev         dev-admin
#           kind-kind         kind-kind   kind-kind
# *         mgmt-admin@mgmt   mgmt        mgmt-admin

if [[ -z "$ACCEPT_CLEANUP_T_AND_C" ]]; then
  echo "ERROR"
  echo "ERROR  Please make sure you understand what is being deleted by this script."
  echo "ERROR  Set ACCEPT_CLEANUP_T_AND_C to any non zero value and re-run."
  exit 1
fi

echo "---- Switching to mgmt cluster"
kubectl config use-context mgmt-admin@mgmt

echo Suspend CAPI sources reconciliation.
flux suspend kustomization infrastructure

echo $(date '+%F %H:%M:%S')
kubectl delete cluster dev -n cluster-dev

echo $(date '+%F %H:%M:%S')
#clusterctl move --kubeconfig=$workdir/target-mgmt.kubeconfig --kubeconfig-context mgmt-admin@mgmt --to-kubeconfig=$HOME/.kube/config --to-kubeconfig-context=kind-kind
clusterctl move --to-kubeconfig=$HOME/.kube/config --to-kubeconfig-context=kind-kind

echo "---- Switching to 'kind' cluster"
kubectl config use-context kind-kind
kubectl delete cluster mgmt -n cluster-mgmt
