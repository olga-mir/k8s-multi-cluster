#!/bin/bash

set -e pipefail

workdir=$(pwd)

if [[ -z "$ACCEPT_KUBECONFIG_MERGE_T_AND_C" ]]; then
  echo "Please make sure you understand what this script is doing."
  echo "A kubeconfig at $HOME/.kube/config is saved to the same path with a timestamp suffix."
  echo "Then additional kubeconfigs are merged together with the original kubeconfig and the result is stored at $HOME/.kube/config"
  echo "This operation should be safe and in case other entries existed in original kubeconfig they will be preserved."
  echo "Set ACCEPT_KUBECONFIG_MERGE_T_AND_C to any non zero value and re-run."
  exit 1
fi

set -x

# backup previous kubeconfig, this is also needed for merge: can't read and redirect to the same place in one command
temp_kubeconfig=$HOME/.kube/config-$(date +%F_%H_%M_%S)
cp $HOME/.kube/config $temp_kubeconfig
KUBECONFIG=$workdir/target-mgmt.kubeconfig:$workdir/cluster-01.kubeconfig:$temp_kubeconfig kubectl config view --raw=true --merge=true > $HOME/.kube/config

# clusterctl get kubeconfig dev -n cluster-dev > dev.kubeconfig
