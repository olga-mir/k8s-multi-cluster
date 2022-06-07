#!/bin/bash
set -eou pipefail

# For more details please check docs/bootstrap-and-pivot.md doc in this repo

# Provide env vars and other settings in $workdir/mgmt-cluster/init-config-mgmt.yaml file
# (note that the content of the file is not validated)
# AWS_B64ENCODED_CREDENTIALS currently accepted only from env var only.
if [ -z "$AWS_B64ENCODED_CREDENTIALS" ] && \
   [ -z "$FLUX_KEY_PATH" ]; then
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

# Install Flux.
kubectl apply -f $workdir/clusters/tmp-mgmt/flux-system/gotk-components.yaml

# https://github.blog/changelog/2022-01-18-githubs-ssh-host-keys-are-now-published-in-the-api/
# curl -H "Accept: application/vnd.github.v3+json" -s https://api.github.com/meta | jq -r '.ssh_keys'
# select the one that starts with "ecdsa-sha2-nistp256"
GITHUB_KNOWN_HOSTS="github.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg="
kubectl create secret generic flux-system -n flux-system \
  --from-file identity=$FLUX_KEY_PATH  \
  --from-file identity.pub=$FLUX_KEY_PATH.pub \
  --from-literal known_hosts="$GITHUB_KNOWN_HOSTS"

set +e
while ! kubectl wait crd kustomizations.kustomize.toolkit.fluxcd.io --for=condition=Established --timeout=5s; do sleep 5; done
kubectl wait crd gitrepositories.source.toolkit.fluxcd.io --for=condition=Established --timeout=10s
set -e

# This has to be applied separately because it depends on CRDs that were created in gotk-components.
kubectl apply -f $workdir/clusters/tmp-mgmt/flux-system/gotk-sync.yaml

# cluster resource for permanent management cluster and the accompanying ClusterResourceSet
# are applied by flux. When the CRS is applied the permanent cluster should be ready to use.
clusterctl init --infrastructure aws --config $workdir/mgmt-cluster/init-config-mgmt.yaml

echo $(date '+%F %H:%M:%S')
sleep 120
while ! kubectl wait --for condition=ResourcesApplied=True clusterresourceset crs -n cluster-mgmt --timeout=10s; do
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

# by default `kind` creates its context in default location (~/.kube/config if $KUBECONFIG is not set)
clusterctl move --kubeconfig $HOME/.kube/config --kubeconfig-context kind-kind --to-kubeconfig=./target-mgmt.kubeconfig -n cluster-mgmt

# Now `mgmt` cluster lives on the AWS permanent management cluster:
# % k get clusters -A
# NAMESPACE      NAME   PHASE         AGE   VERSION
# cluster-dev    dev    Provisioned   50m
# cluster-mgmt   mgmt   Provisioned   56m

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
