#!/bin/bash

set -euo pipefail

# Collection of helper scripts to work with CAPI clusters.
# The scripts assume CAPI kubeconfig default naming conventions for contexts, clusters and users (e.g. <cluster-name>-admin@<cluster-name>)
# On top of it, this project assumes 1 cluster per namespace and namespace and cluster name are identical
# Run `./helpers.sh -h` to learn more.

REPO_ROOT=$(git rev-parse --show-toplevel)
tempdir=$(mktemp -d)
trap 'exit_handler $? $LINENO' EXIT
echo $tempdir >> tempdirs.txt

# Management cluster kube config and context
KUBECONFIG=${K8S_MULTI_KUBECONFIG-$REPO_ROOT/.kubeconfig}
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
    -c|--cluster)
      finalize_clusters ${2:-}
      ;;
    -k|--get-kube-config)
      get_and_merge_kubeconfig $2
      ;;
    -g|--generate-clusters-manifests)
      generate_clusters_manifests ${2:-}
      ;;
    -h|--help)
      show_help
      ;;
  esac
  shift
done

}

generate_clusters_manifests() {
  local cluster=$1
  # TODO for now only single cluster, name must be provided, but not checked
  # echo Generating manifests for all clusters defined in $REPO_ROOT/config
  echo Generating manifests for single cluster $cluster
  #for f in $REPO_ROOT/config/cluster*.env; do
  for f in $REPO_ROOT/config/$cluster.env; do
    set +x
    . $f
    set -x
    # CLUSTER_NAME comes from env file
    cluster_dir="$REPO_ROOT/clusters/$INITIALLY_MANAGED_BY/$CLUSTER_NAME"
    mkdir -p $cluster_dir
    envsubst < $REPO_ROOT/templates/aws/cluster.yaml > $cluster_dir/capi-cluster.yaml
    envsubst < $REPO_ROOT/templates/capi-workload-namespace.yaml > $cluster_dir/namespace.yaml
    envsubst < $REPO_ROOT/templates/platform.yaml > $cluster_dir/platform.yaml
    cp $REPO_ROOT/templates/kustomization.yaml $cluster_dir/kustomization.yaml

    # Add new cluster to mgmt cluster kustomization
    kustomization_file=$REPO_ROOT/clusters/$INITIALLY_MANAGED_BY/kustomization.yaml
    if [ -z "$(grep $CLUSTER_NAME $kustomization_file)" ]; then
      yq eval ". *+ {\"resources\":[\"$CLUSTER_NAME\"]}" $kustomization_file --inplace
    fi
  done

#  #if false; then
#  if :; then
#    git add $infra_dir
#    git add $cluster_dir
#    git add $REPO_ROOT/infrastructure/control-plane-cluster/kustomization.yaml
#    git commit -m "feat: add or update generated files for $CLUSTER_NAME"
#    git push origin $GITHUB_BRANCH
#  fi

#  # I don't want to give flux deploy key with write permissions, therefore 'bootstrap' is not an option
#  # 'flux install --export' does not have options to generate gotk-sync.yaml, so instead this will be
#  # instantiated from template
#  # This is only needed when adding a cluster for the first time to the repo. On the following invocations, flux is deployed by flux instance on a management cluster
#  cluster_dir=$REPO_ROOT/clusters/${CLUSTER_NAME}/flux-system
#  mkdir -p $cluster_dir
#  flux install --version=$FLUXCD_VERSION --export > $cluster_dir/gotk-components.yaml
#  envsubst < $REPO_ROOT/templates/gotk-sync.yaml > $cluster_dir/gotk-sync.yaml
#  generate_kustomizations $cluster_dir/kustomization.yaml clusters/$CLUSTER_NAME/kustomization.yaml
#
#
}

# Retrieve kubeconfig from `cluster-mgmt` for a workload cluster
# the kubeconfig is merged into kubeconfig pointed to by $KUBECONFIG or the default kubeconfig location
# backup of the kubeconfig is taken before this operation
get_and_merge_kubeconfig() {
  echo Retrieving kubeconfig for $cluster

  local cluster=$1
  wait_for $cluster
  clusterctl --kubeconfig=$KUBECONFIG --kubeconfig-context $CONTEXT_MGMT get kubeconfig $cluster -n $cluster > $tempdir/$cluster-config

  # echo $(date '+%F %H:%M:%S') - Waiting for $cluster kubeconfig to become available
  # while ! clusterctl --kubeconfig=$KUBECONFIG --kubeconfig-context $CONTEXT_MGMT get kubeconfig $cluster -n $cluster > $tempdir/$cluster-config ; do
  #   echo $(date '+%F %H:%M:%S') re-try in 25s... && sleep 25
  # done

  # get workload cluster kubeconfig and merge it to the main one
  cp $KUBECONFIG $KUBECONFIG-$(date +%F_%H_%M_%S)
  KUBECONFIG=$KUBECONFIG:$tempdir/$cluster-config kubectl config view --raw=true --merge=true > $tempdir/merged-config
  chmod 600 $tempdir/merged-config
  mv $tempdir/merged-config $KUBECONFIG
}

# installs Flux secret on the provided cluster,
# retrieves its kubeconfig and merges it to working kubeconfig
# if necessary wait for the cluster and components to be ready
finalize_cluster() {
  local cluster=$1
  echo Finalizing cluster $cluster in $cluster namespace

  #wait_for $cluster
  get_and_merge_kubeconfig $cluster

  set +x
  . $REPO_ROOT/config/$cluster.env
  set -x

  CONTEXT_WORKLOAD="$cluster-admin@$cluster"
  KUBECTL_WORKLOAD="kubectl --kubeconfig $KUBECONFIG --context $CONTEXT_WORKLOAD"

  # on clusters that already existed in the git repo before deploying
  # flux is installed by flux instance on a management cluster, but secret for now is installed manually
  # to avoid storing even encrypted secrets in public github repo.
  $KUBECTL_WORKLOAD create secret generic flux-system -n flux-system \
    --from-file identity=$FLUX_KEY_PATH  \
    --from-file identity.pub=$FLUX_KEY_PATH.pub \
    --from-literal known_hosts="$GITHUB_KNOWN_HOSTS"
}

wait_for() {
  echo $(date '+%F %H:%M:%S') - Waiting for $cluster control plane

  cluster=$1
  set +e
  $($KUBECTL_MGMT wait cluster $cluster -n $cluster --for=condition=ControlPlaneReady) --timeout=2s
  if [ $? != 0 ]; then
    while [ -z $($KUBECTL_MGMT wait cluster $cluster -n $cluster --for=condition=ControlPlaneReady) ]; do sleep 15; done
  fi
  set -e
}

finalize_clusters() {
  set +u
  if [ ! -z "$1" ]; then
    finalize_cluster $1
    set -u
  else
    set -u
    clusters=$($KUBECTL_MGMT get clusters -A --no-headers=true -o name)
    for line in $clusters; do
      cluster=$(echo $line |  cut -d'/' -f 2)
      if [ "$cluster" != "cluster-mgmt" ]; then
        finalize_cluster $cluster
      fi
    done
  fi
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
  cat << EOF

Collection of helper scripts to work with CAPI clusters.
The scripts assume CAPI kubeconfig default naming conventions for contexts, clusters and users (e.g. <cluster-name>-admin@<cluster-name>)
On top of it, this project assumes 1 cluster per namespace and namespace and cluster name are identical

Examples:

\`./helpers.sh -g\` Generate CAPI cluster manifests for each cluster defined in $REPO_ROOT/config
\`./helpers.sh -c\` Wait for all workload clusters to be ready, retrive kubeconfig and install remaining bits (flux secret)

Usage:

  -c|--cluster [cluster-name] - finalize a workload cluster if cluster_name is provided or all clusters when no cluster name provided.
    Since flux secret is not managed in GitOps, it needs to be provided separately.

  -g|--generate-clusters-manifests - generate CAPI manifests for all clusters from the template.

EOF
}

main "$@"
