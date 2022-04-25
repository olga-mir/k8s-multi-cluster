#!/bin/bash

set -eou pipefail
workdir=$(pwd)

# For more details please check docs/bootstrap-and-pivot.md doc in this repo

trap "rm -f bootstrap.yaml" EXIT
cat > bootstrap.yaml << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: worker
EOF

kind create cluster --config bootstrap.yaml

clusterctl init --infrastructure aws

# Setup config environment variables, and AWS_B64ENCODED_CREDENTIALS
# run `clusterctl generate cluster --list-variables aws` to get the list of variables
if [ -z "$AWS_CONTROL_PLANE_MACHINE_TYPE" ] || \
   [ -z "$AWS_NODE_MACHINE_TYPE" ] || \
   [ -z "$AWS_SSH_KEY_NAME" ] || \
   [ -z "$KUBERNETES_VERSION" ] || \
   [ -z "$AWS_B64ENCODED_CREDENTIALS" ]; then
  # unreachable code due to set -u. needs better 'trapping' to provide an error message
  exit 1
fi

set +e
set -x
while ! kubectl get clusters; do
  sleep 15
done
set -e
set +x
echo \"No resources found in default namespace\" expected

# deploy permanent mgmt cluster object in `default` ns in temp cluster
# clusterctl generate cluster mgmt > mgmt.yaml

kubectl apply -f $workdir/mgmt.yaml

while ! kubectl wait kubeadmcontrolplane mgmt-control-plane --for jsonpath='{.status.initialized}'=true --timeout=30s; do
  echo waiting for control plane to become initialized
done

clusterctl get kubeconfig mgmt > $workdir/target-mgmt.kubeconfig

# backup previous kubeconfig - as necessary
# cp $HOME/.kube/config $HOME/.kube/config-$(date +%F_%H_%M_%S)

KUBECONFIG=$HOME/.kube/config:$workdir/kind.kubeconfig:$workdir/target-mgmt.kubeconfig kubectl config view --merge=true > $HOME/.kube/config

##############
############## ------ on AWS mgmt cluster ------
##############

kubectl config use-context mgmt-admin@mgmt

kubectl apply -f https://docs.projectcalico.org/v3.21/manifests/calico.yaml
#TODO https://projectcalico.docs.tigera.io/v3.22/manifests/calico.yaml

clusterctl init --infrastructure aws

set +e
set -x
while ! kubectl get clusters; do
  sleep 15
done
set -e
set +x
echo \"No resources found in default namespace\" expected

clusterctl move --to-kubeconfig=./target-mgmt.kubeconfig

kind delete cluster
