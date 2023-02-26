#!/bin/bash

set -euo pipefail

# Collection of helper scripts to work with CAPI clusters.
# The scripts assume CAPI kubeconfig default naming conventions for contexts, clusters and users (e.g. <cluster-name>-admin@<cluster-name>)
# On top of it, this project assumes 1 cluster per namespace and namespace and cluster name are identical
# Run `./helpers.sh -h` to learn more.

REPO_ROOT=$(git rev-parse --show-toplevel)
tempdir=$(mktemp -d)

# Management cluster kube config and context
KUBECONFIG=${KUBECONFIG:-$HOME/.kube/config}
CONTEXT_MGMT="cluster-mgmt-admin@cluster-mgmt"
KUBECTL_MGMT="kubectl --kubeconfig $KUBECONFIG --context $CONTEXT_MGMT"
echo Management cluster kubectl config: $KUBECTL_MGMT

set +x
. $REPO_ROOT/config/shared.env
set -x

main() {

set -x

while [[ $# -gt 0 ]]; do
  case $1 in
    -c|--cluster-name)
      CLUSTER_NAME_ARG=$2; shift
      ;;
    -k|--get-kube-config)
      CLUSTER_NAME_ARG=$2; shift
      ;;
    -g|--generate-clusters-manifests)
      generate_clusters_manifests
      ;;
    -h|--help)
      show_help
      ;;
    *)
      show_help
      ;;
  esac
  shift
done

}

generate_clusters_manifests() {
  for f in $REPO_ROOT/config/cluster*.env; do
    set +x
    . $f
    set -x
    cluster_repo="$REPO_ROOT/clusters/$INITIALLY_MANAGED_BY/$CLUSTER_NAME"
    mkdir -p $cluster_repo
    envsubst < $REPO_ROOT/templates/aws/cluster.yaml > $cluster_repo/$CLUSTER_NAME.yaml
  done
}

show_help() {
  set +x
  echo "Collection of helper scripts to work with CAPI clusters."
  echo "The scripts assume CAPI kubeconfig default naming conventions for contexts, clusters and users (e.g. <cluster-name>-admin@<cluster-name>)"
  echo "On top of it, this project assumes 1 cluster per namespace and namespace and cluster name are identical"
  echo 
  echo "Examples"
  echo "\`./helpers.sh -g\` Generate CAPI cluster manifests for each cluster defined in $REPO_ROOT/config"
  echo
  echo "Usage:"
  echo
  echo "-c|--cluster-name - "
  echo
  echo ""
  echo ""
  echo ""
  exit 0
}

main "$@"
