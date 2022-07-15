#!/bin/bash

set -eou pipefail
tempdir=$(mktemp -d)
REPO_ROOT=$(git rev-parse --show-toplevel)

for f in $(find $REPO_ROOT -name "kustomization.yaml"); do
  filename=$(basename $(dirname $f))
  cmd="kustomize build --load-restrictor=LoadRestrictionsNone $(dirname $f) > $tempdir/${filename}_${RANDOM}.yaml"
  echo $cmd
  eval $cmd
done

echo
echo Done
echo
echo $tempdir
