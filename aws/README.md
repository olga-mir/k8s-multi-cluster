# AWS

This directory contains necessary pre-requisites for Cluster API on AWS.

Create necessary IAM permissions in one stack:
```
aws cloudformation deploy --template-file aws/cluster-api-provider-aws-sigs-k8s-io.yaml --stack-name cluster-api-provider-aws-sigs-k8s-io --capabilities CAPABILITY_NAMED_IAM
```
This is the same stack as deployed by `clusterawsadm bootstrap iam create-cloudformation-stack`
It was retrieved with aws cli after it was created by the clusterawsadm command: `aws cloudformation get-template --stack-name cluster-api-provider-aws-sigs-k8s-io --template-stage Original`

Deploy group and a user with the roles defined in previous stack + EKS managed policy.
This will be needed for `AWS_B64ENCODED_CREDENTIALS` at a later state to avoid encoding current creds as they are most likely too powerful.
(TODO - replace placeholder with AWS accound id before applying)
```
aws cloudformation deploy --template-file aws/iam-capi.yaml --stack-name capi-user --capabilities CAPABILITY_NAMED_IAM
```
