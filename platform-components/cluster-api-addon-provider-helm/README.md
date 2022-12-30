# Cluster API Addon Provider for Helm (CAAPH)

https://github.com/Jont828/cluster-api-addon-provider-helm

# Generate manifests:

Clone CAAPH repo, build & push image to your registry according to quick start instructions.

To generate manifests you need to run "dry-run" deploy commands from the Makefile (`grep "kubectl apply" Makefile`) then extract and update or create dry-run versions of these tragets. 
Script version will look something like:

```
REGISTRY=<your-registry>
IMG=<your-image-with-tag>

MANIFESTS_FILE="manifests.yaml"

echo "# This manifests are generated from https://github.com/Jont828/cluster-api-addon-provider-helm
# Apache License 2.0: https://github.com/Jont828/cluster-api-addon-provider-helm/blob/48e927746a93c0b7f6b295ad810152ad48616663/LICENSE" > $MANIFESTS_FILE
echo "---" >> $MANIFESTS_FILE
kustomize build config/crd >> $MANIFESTS_FILE
echo "---" >> $MANIFESTS_FILE
cd config/manager && kustomize edit set image controller=${REGISTRY}/${IMG}
cd ../../ && kustomize build config/default >> $MANIFESTS_FILE
```
