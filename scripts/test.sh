#!/bin/bash

set -eou pipefail
tempdir=$(mktemp -d)
workdir=$(pwd)
count=0

for f in $(find $workdir -name "kustomization.yaml"); do
  filename=$(basename $(dirname $f))
  cmd="kustomize build --load-restrictor=LoadRestrictionsNone $(dirname $f) > $tempdir/${filename}_${count}.yaml"
  echo $cmd
  eval $cmd
  count=$((count+1))
done

#rm -rf $tempdir
