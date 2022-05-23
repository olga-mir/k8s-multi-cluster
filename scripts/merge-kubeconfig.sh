#!/bin/bash

set -eoux pipefail

workdir=$(pwd)

# backup previous kubeconfig, this is also needed for merge: can't read and redirect to the same place in one command
temp_kubeconfig=$HOME/.kube/config-$(date +%F_%H_%M_%S)
cp $HOME/.kube/config $temp_kubeconfig
KUBECONFIG=$workdir/target-mgmt.kubeconfig:$temp_kubeconfig kubectl config view --raw=true --merge=true > $HOME/.kube/config
