#!/bin/bash
set -eou pipefail

# For more details please check docs/bootstrap-and-pivot.md doc in this repo

# Provide env vars and other settings in $workdir/mgmt-cluster/init-config-mgmt.yaml file
# (note that the content of the file is not validated)
# AWS_B64ENCODED_CREDENTIALS currently accepted only from env var only.
if [ -z "$AWS_B64ENCODED_CREDENTIALS" ] && \
   [ -z "$FLUX_REPO_SSH" ] && \   # ssh format for 'this' repo, e.g. ssh://git@github.com/olga-mir/k8s-multi-cluster
   [ -z "$FLUX_BRANCH" ] && \     # branch in this repo for flux to sync
   [ -z "$FLUX_KEY_PATH" ]; then  # path to ssh key which is a deployment key for this repo
  echo "Error required env variables are not set" && exit 1
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
kubectl create namespace cluster-mgmt

clusterctl init --infrastructure aws --config $workdir/mgmt-cluster/init-config-mgmt.yaml

echo $(date '+%F %H:%M:%S')
set +e
while ! kubectl wait crd clusters.cluster.x-k8s.io --for=condition=Established --timeout=5s; do sleep 20; done
set -e

# deploy permanent mgmt cluster object using
# manifests that were created beforehand with `clusterctl generate cluster mgmt`

# applying cluster manifests immediatelly will fail because components webhooks are not yet ready to serve traffic
# it is easier to retry applying rather than checking on each component individually
echo $(date '+%F %H:%M:%S')
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

echo $(date '+%F %H:%M:%S')
sleep 120
while ! kubectl wait --for condition=ResourcesApplied=True clusterresourceset crs-calico -n cluster-mgmt --timeout=10s ; do
  echo $(date '+%F %H:%M:%S') waiting for management cluster to become ready && sleep 45
done


############## ------ on AWS mgmt cluster ------

# kubeconfig is available when this secret is ready: `k get secret mgmt-kubeconfig`
clusterctl get kubeconfig mgmt -n cluster-mgmt > $workdir/target-mgmt.kubeconfig
# Apart from being shorter and nicer, it is also required later for kubefed which breaks when there are special chars in context name
kubectl --kubeconfig=$workdir/target-mgmt.kubeconfig config rename-context mgmt-admin@mgmt mgmt

clusterctl init --infrastructure aws --kubeconfig $workdir/target-mgmt.kubeconfig --kubeconfig-context mgmt --config $workdir/mgmt-cluster/init-config-workload.yaml

echo $(date '+%F %H:%M:%S')
set +e
while ! kubectl --kubeconfig=$workdir/target-mgmt.kubeconfig wait crd clusters.cluster.x-k8s.io --for=condition=Established; do sleep 15; done
set -e

clusterctl move --to-kubeconfig=./target-mgmt.kubeconfig -n cluster-mgmt

# Now `mgmt` cluster lives on the AWS permanent management cluster:
# % k get clusters -A
# NAMESPACE     NAME   PHASE         AGE   VERSION
# cluster-dev   dev    Provisioned   50m
# default       mgmt   Provisioned   56m
# However for this setup, still keep the `kind` cluster because it will be useful to tear down the mgmt cluster.
# kind delete cluster


############## ------ FluxCD on AWS mgmt cluster ------

# repo per team
# https://fluxcd.io/docs/guides/repository-structure/#repo-per-team
# https://github.com/fluxcd/flux2-multi-tenancy

# *github* flux bootstrap uses PAT for auth. For using SSH follow *generic git server* instructions.

# use existing key or generate a new one according to docs:
# https://docs.github.com/en/authentication/connecting-to-github-with-ssh/generating-a-new-ssh-key-and-adding-it-to-the-ssh-agent
# add public key as a deployment key to the repo.

# Create SSH secret as described in: https://fluxcd.io/docs/components/source/gitrepositories/#ssh-authentication
# `k get secret flux-system -n flux-system` is the secret in the link above (ssh-credentials)

flux bootstrap git \
  --kubeconfig=$workdir/target-mgmt.kubeconfig --context mgmt \
  --url=$FLUX_REPO_SSH \
  --branch=$FLUX_BRANCH \
  --private-key-file=$FLUX_KEY_PATH \
  --path=clusters/mgmt


############## ------ Workload cluster bootstrap ------

# Once Flux is bootstrapped on the cluster it will apply cluster-dev CAPI definition and workload cluster will start provisioning
sleep 90
while ! kubectl --kubeconfig=$workdir/target-mgmt.kubeconfig wait --context mgmt --for condition=ResourcesApplied=True clusterresourceset crs-calico -n cluster-dev --timeout=15s ; do
  echo $(date '+%F %H:%M:%S') waiting for workload cluster to become ready
  sleep 15
done

clusterctl --kubeconfig=$workdir/target-mgmt.kubeconfig --kubeconfig-context mgmt get kubeconfig dev -n cluster-dev > $workdir/dev.kubeconfig
kubectl --kubeconfig=$workdir/dev.kubeconfig config rename-context dev-admin@dev dev

flux bootstrap git \
  --kubeconfig=$workdir/dev.kubeconfig --context dev \
  --url=$FLUX_REPO_SSH \
  --branch=$FLUX_BRANCH \
  --private-key-file=$FLUX_KEY_PATH \
  --path=clusters/cluster-dev
