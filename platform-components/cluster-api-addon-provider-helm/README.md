# Cluster API Addon Provider for Helm (CAAPH)

https://github.com/Jont828/cluster-api-addon-provider-helm

# Generate manifests:

Clone CAAPH repo, build & push image to your registry according to quick start instructions.

`deploy` target from the repo's Makefile deploys all the necessary manifests, to get the manifests you need to get the "dry-run" version of this target by either adding a new target or creating a script similar to:

```bash
REGISTRY=<your-registry>
IMG=<your-image-with-tag>

MANIFESTS_FILE="manifests.yaml"

echo "# This file is generated from https://github.com/Jont828/cluster-api-addon-provider-helm
# Apache License 2.0: https://github.com/Jont828/cluster-api-addon-provider-helm/blob/48e927746a93c0b7f6b295ad810152ad48616663/LICENSE" > $MANIFESTS_FILE
echo "---" >> $MANIFESTS_FILE
cd config/manager && kustomize edit set image controller=${REGISTRY}/${IMG}
cd ../../ && kustomize build config/default >> $MANIFESTS_FILE
```
