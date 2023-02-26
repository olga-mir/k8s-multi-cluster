#!/bin/bash

KUBECONFIG=${1:-~/.kube/config}

report() {
  ctx=$1
  kubectx $ctx
  KUBECTL="kubectl --kubeconfig $KUBECONFIG --context $ctx"
  echo -e "\n===== $ctx =====\n"
  set -x
  $KUBECTL get clusters -A
  $KUBECTL get po -A | grep -E "flux-system|cilium"
  flux get all -A
  set +x
}

report cluster-mgmt-admin@cluster-mgmt

clusters=$(kubectl --kubeconfig $KUBECONFIG --context cluster-mgmt-admin@cluster-mgmt get clusters -A --no-headers=true -o name)
for line in $clusters; do
  cluster=$(echo $line |  cut -d'/' -f 2)
  if [ "$cluster" != "cluster-mgmt" ]; then
    report $cluster-admin@$cluster
  fi
done
