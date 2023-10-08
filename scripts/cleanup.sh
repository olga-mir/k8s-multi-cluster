#!/bin/bash

echo "This script will delete all the clusters that can be found in management cluster"
echo "in 'mgmt' kubeconfig context by deleting (delete is performed by CAPI)"
echo "Press ^C if this is not intentional. Sleeping 5s"
sleep 5

REPO_ROOT=$(git rev-parse --show-toplevel)
KUBECONFIG=${K8S_MULTI_KUBECONFIG-$REPO_ROOT/.kubeconfig}

CTX_MGMT=cluster-mgmt-admin@cluster-mgmt
KUBECTL_MGMT="kubectl --kubeconfig=$KUBECONFIG --context $CTX_MGMT"

set -x
echo Suspend CAPI sources reconciliation.
flux --kubeconfig=$KUBECONFIG --context $CTX_MGMT suspend kustomization flux-system

echo $(date '+%F %H:%M:%S') Moving all clusters back to 'kind' cluster
clusters=$($KUBECTL_MGMT get clusters -A --no-headers=true -o name)
for line in $clusters; do
  cluster=$(echo $line |  cut -d'/' -f 2)
  clusterctl move --kubeconfig=$KUBECONFIG --kubeconfig-context $CTX_MGMT --to-kubeconfig=$KUBECONFIG --to-kubeconfig-context=kind-kind -n $cluster
done
sleep 15

echo "---- Switching to 'kind' cluster"
flux --kubeconfig=$KUBECONFIG --context kind-kind suspend kustomization flux-system

KUBECTL_KIND="kubectl --kubeconfig=$KUBECONFIG --context kind-kind"
clusters=$($KUBECTL_KIND get clusters -A --no-headers=true -o name)
for line in $clusters; do
  cluster=$(echo $line |  cut -d'/' -f 2)
  $KUBECTL_KIND delete cluster $cluster -n $cluster &
done

echo $(date '+%F %H:%M:%S')
sleep 300

for line in $clusters; do
  cluster=$(echo $line |  cut -d'/' -f 2)
  while $KUBECTL_KIND get cluster $cluster -n $cluster; do
    sleep 60
  done
  kubectl --kubeconfig=$KUBECONFIG config delete-user $cluster-admin
  kubectl --kubeconfig=$KUBECONFIG config delete-cluster $cluster
  kubectl --kubeconfig=$KUBECONFIG config delete-context ${cluster}-admin@$cluster
done

kind delete cluster
