#!/bin/bash
set -eou pipefail

# For more details please check docs/bootstrap-and-pivot.md doc in this repo

# Setup config environment variables, and AWS_B64ENCODED_CREDENTIALS
# run `clusterctl generate cluster --list-variables aws` to get the list of variables
# values except creds will be stored in init-config files, for now check they are set, remove after testing
if [ -z "$AWS_CONTROL_PLANE_MACHINE_TYPE" ] || \
   [ -z "$AWS_NODE_MACHINE_TYPE" ] || \
   [ -z "$AWS_SSH_KEY_NAME" ] || \
   [ -z "$KUBERNETES_VERSION" ] || \
   [ -z "$AWS_B64ENCODED_CREDENTIALS" ]; then
  exit 1
fi

set -x

workdir=$(pwd)

trap "rm -f bootstrap.yaml" EXIT
cat > bootstrap.yaml << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: worker
EOF

kind create cluster --config bootstrap.yaml

clusterctl init --infrastructure aws --config $workdir/mgmt-cluster/init-config-mgmt.yaml

set +e
while ! kubectl get clusters; do
  sleep 15
done
set -e
echo
echo \"No resources found in default namespace\" is expected
echo

# deploy permanent mgmt cluster object in `default` ns in temp cluster
# use manifests that were created before with `clusterctl generate cluster mgmt`
# and committed to the repo

# applying cluster manifests immediatelly will fail because components webhooks are not yet ready to serve traffic
# it is easier to retry applying rather than checking on each component individually
retries=8
set +e
kubectl apply -f $workdir/mgmt-cluster/cluster.yaml 2>/dev/null
while [ $? -ne 0 ]; do
  echo Failed to apply cluster config, re-trying
  sleep 15
  if [[ $retries -eq 0 ]]; then
    # if retries are exhausted, there might be a genuine error, run the command one more time without swallowing the error
    kubectl apply -f $workdir/mgmt-cluster/cluster.yaml
    echo "Failed to apply cluster config, aborting."
    exit 1
  fi
  ((retries--))
  kubectl apply -f $workdir/mgmt-cluster/cluster.yaml 2>/dev/null
done
set -e

kubectl apply -f $workdir/mgmt-cluster/cm-calico-v3.21.yaml

############## ------ on AWS mgmt cluster ------

sleep 240

while ! kubectl wait --for condition=ResourcesApplied=True clusterresourceset crs-calico --timeout=30s ; do
  echo $(date '+%F %H:%M:%S') waiting for management cluster to become ready
  sleep 30
done

# kubeconfig is available when this secret is ready: `k get secret mgmt-kubeconfig`
clusterctl get kubeconfig mgmt > $workdir/target-mgmt.kubeconfig
clusterctl init --infrastructure aws --kubeconfig $workdir/target-mgmt.kubeconfig --kubeconfig-context mgmt-admin@mgmt --config $workdir/mgmt-cluster/init-config-workload.yaml

set +e
while ! kubectl --kubeconfig=$workdir/target-mgmt.kubeconfig get clusters; do
  sleep 15
done
set -e
echo \"No resources found in default namespace\" expected

clusterctl move --kubeconfig=$workdir/target-mgmt.kubeconfig --kubeconfig-context mgmt-admin@mgmt --to-kubeconfig=./target-mgmt.kubeconfig

# kind delete cluster
# and now what? how do you manage the permanent management cluster? keep it now for simplicity
