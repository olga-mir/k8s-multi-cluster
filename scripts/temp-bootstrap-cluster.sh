#!/bin/bash

set -eou pipefail

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

if [ -z "$AWS_CONTROL_PLANE_MACHINE_TYPE" ] || \
   [ -z "$AWS_NODE_MACHINE_TYPE" ] || \
   [ -z "$AWS_SSH_KEY_NAME" ] || \
   [ -z "$KUBERNETES_VERSION" ] || \
   [ -z "$AWS_B64ENCODED_CREDENTIALS" ]; then
  # unreachable code due to set -u. needs better 'trapping' to provide an error message
  exit 1
fi

clusterctl init --infrastructure aws

# Setup config environment variables, and AWS_B64ENCODED_CREDENTIALS
# run `clusterctl generate cluster --list-variables aws` to get the list of variables

clusterctl generate cluster mgmt > mgmt.yaml
