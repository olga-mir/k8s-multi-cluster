#!/bin/bash
workdir=$(pwd)

echo "This script will delete all the clusters that can be found in management cluster"
echo "in 'mgmt' kubeconfig context by deleting (delete is performed by CAPI)"
echo "Press ^C if this is not intentional. Sleeping 5s"
sleep 5

CTX_MGMT=cluster-mgmt-admin@cluster-mgmt

set -x
echo Suspend CAPI sources reconciliation.
flux --context $CTX_MGMT suspend kustomization infrastructure

KUBECTL_MGMT="kubectl --context $CTX_MGMT"
echo $(date '+%F %H:%M:%S') Moving all clusters back to 'kind' cluster
clusters=$($KUBECTL_MGMT get clusters -A --no-headers=true -o name)
for line in $clusters; do
  cluster=$(echo $line |  cut -d'/' -f 2)
  clusterctl move --kubeconfig-context $CTX_MGMT --to-kubeconfig=$HOME/.kube/config --to-kubeconfig-context=kind-kind -n $cluster
done
sleep 15

echo "---- Switching to 'kind' cluster"
flux --context kind-kind suspend kustomization infrastructure

KUBECTL_KIND="kubectl --context kind-kind"
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
  kubectl config delete-user $cluster-admin
  kubectl config delete-cluster $cluster
done

kind delete cluster
