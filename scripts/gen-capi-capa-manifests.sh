REPO_ROOT=$(git rev-parse --show-toplevel)

# details: https://github.com/olga-mir/k8s-multi-cluster/pull/14
# and https://github.com/olga-mir/k8s-multi-cluster/pull/14/files#diff-22504d1229f26e0a9e24913fa3066f2a5309cd68df4f572e7f008364c8373114R20

# version is hardcoded in kustomization file.
capi_bundle=$REPO_ROOT/infrastructure/base/capi-bundle-v1.3.0.yaml
echo "# This file is generated from Cluster API manifests https://github.com/kubernetes-sigs/cluster-api
# with Feature Gates environment variables replaced.
# LICENSE: https://github.com/kubernetes-sigs/cluster-api/blob/1eedede7730c7d8f7f7247f4f2d5b63fc5b4f545/LICENSE" > $capi_bundle
kustomize build --load-restrictor=LoadRestrictionsNone $REPO_ROOT/scripts/capi >> $capi_bundle

export AWS_B64ENCODED_CREDENTIALS="Cg=="
capa_bundle=$REPO_ROOT/infrastructure/base/capa-bundle-v1.5.1.yaml
echo "# This file is generated from Cluster API AWS provider manifests
# LICENSE: https://github.com/kubernetes-sigs/cluster-api-provider-aws/blob/ef5baabb770749c300c2bc893585b6f2d96d07ee/LICENSE" > $capa_bundle
clusterctl generate provider --infrastructure aws:v1.5.1 | yq e '(. | select(.kind != "Secret"))' >> $capa_bundle

