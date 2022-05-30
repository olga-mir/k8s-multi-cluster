#!/bin/bash

# Helper script to clean up turtles-all-the-way-down clusters.
# This script assumes it runs on the same setup as deployed by other scripts in this repo.
# The following contexts are expected to be available in kubeconfig (use ./scripts/merge-kubeconfig.sh) to merge all in one

# this script assumes these contexts are available and the management chain is kind -> mgmt -> dev.
# % k config get-contexts
# CURRENT   NAME              CLUSTER     AUTHINFO     NAMESPACE
#           dev-admin@dev     dev         dev-admin
#           kind-kind         kind-kind   kind-kind
# *         mgmt-admin@mgmt   mgmt        mgmt-admin

if [[ -z "$ACCEPT_CLEANUP_T_AND_C" ]]; then
  echo "Please make sure you understand what is being deleted by this script."
  echo "Set ACCEPT_CLEANUP_T_AND_C to any non zero value and re-run."
  exit 1
fi

# finds cluster object in current kubectl context. Expects only one object.
find_and_delete_cluster() {
  ns_and_name=$(kubectl get cluster -A --no-headers=true | awk 'NF=2')
  [[ -z "$ns_and_name" ]] && echo "No clusters found"
  kubectl delete cluster -n $ns_and_name
}

main() {
  echo Suspend CAPI sources reconciliation.

  echo "---- Switching to mgmt cluster"
  kubectl config use-context mgmt-admin@mgmt
  flux suspend kustomization infrastructure
  find_and_delete_cluster

  echo "---- Switching to 'kind' cluster to delete the management 'mgmt' cluster"
  kubectl config use-context kind-kind
  find_and_delete_cluster

  kind delete cluster
}

main
