#!/bin/bash

set -euo pipefail

# Collection of helper scripts to work with CAPI clusters.
# The scripts assume CAPI kubeconfig default naming conventions for contexts, clusters and users (e.g. <cluster-name>-admin@<cluster-name>)
# On top of it, this project assumes 1 cluster per namespace and namespace and cluster name are identical
# Run `./helpers.sh -h` to learn more.

REPO_ROOT=$(git rev-parse --show-toplevel)
tempdir=$(mktemp -d)
trap 'exit_handler $? $LINENO' EXIT

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
      get_and_merge_kubeconfig $CLUSTER_NAME_ARG
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
  echo Generating manifests for all clusters defined in $REPO_ROOT/config
  for f in $REPO_ROOT/config/cluster*.env; do
    set +x
    . $f
    set -x
    cluster_repo="$REPO_ROOT/clusters/$INITIALLY_MANAGED_BY/$CLUSTER_NAME"
    mkdir -p $cluster_repo
    envsubst < $REPO_ROOT/templates/aws/cluster.yaml > $cluster_repo/capi-cluster.yaml
  done
}

# Retrieve kubeconfig from `cluster-mgmt` for a workload cluster
# the kubeconfig is merged into kubeconfig pointed to by $KUBECONFIG or the default kubeconfig location
# backup of the kubeconfig is taken before this operation
get_and_merge_kubeconfig() {
  echo Retrieving kubeconfig for $cluster

  local cluster=$1
  echo $(date '+%F %H:%M:%S') - Waiting for $cluster kubeconfig to become available
  while ! clusterctl --kubeconfig=$KUBECONFIG --kubeconfig-context $CONTEXT_MGMT get kubeconfig $cluster -n $cluster > $tempdir/$cluster-config ; do
    echo $(date '+%F %H:%M:%S') re-try in 25s... && sleep 25
  done

  # get workload cluster kubeconfig and merge it to the main one
  cp $HOME/.kube/config $HOME/.kube/config-$(date +%F_%H_%M_%S)
  KUBECONFIG=$HOME/.kube/config:$tempdir/$cluster-config kubectl config view --raw=true --merge=true > $tempdir/merged-config
  chmod 600 $tempdir/merged-config
  mv $tempdir/merged-config $HOME/.kube/config
}

finalize_cluster() {
  local cluster=$1
  echo Finalizing cluster $cluster in $cluster namespace

  get_and_merge_kubeconfig $cluster

  CONTEXT_WORKLOAD="$cluster-admin@$cluster"
  KUBECTL_WORKLOAD="kubectl --kubeconfig $KUBECONFIG --context $CONTEXT_WORKLOAD"

  set +e
  echo $(date '+%F %H:%M:%S') - Waiting for workload cluster to become responsive
  while [ -z $($KUBECTL_WORKLOAD get pod -n kube-system -l component=kube-apiserver -o name) ]; do sleep 25; done
  set -e

  set +x
  . $REPO_ROOT/config/$cluster.env
  set -x

  # TODO - add wait for namespace instead of this sleep
  sleep 60

  # on clusters that already existed in the git repo before deploying
  # flux is installed by flux instance on a management cluster, but secret for now is installed manually
  # to avoid storing even encrypted secrets in public github repo.
  $KUBECTL_WORKLOAD create secret generic flux-system -n flux-system \
    --from-file identity=$FLUX_KEY_PATH  \
    --from-file identity.pub=$FLUX_KEY_PATH.pub \
    --from-literal known_hosts="$GITHUB_KNOWN_HOSTS"
}

exit_handler() {
  set +x
  if [ "$1" != "0" ]; then
    echo "LINE: $2 ERROR: $1"
  fi
  rm -rf $tempdir
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
