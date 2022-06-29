#!/bin/bash

echo EXIT.
echo Read the script and uncomment exit statement below.
exit 1

# Quick delete all resources that make up the clusters provisioned in this account. (did I say it is brutal)
# CAPI delete hangs for too long on deleting VPC even though the VPC can be released if tried manually via console.

set -eoux pipefail

running_instances=$(aws ec2 describe-instances --filters "Name=instance-state-name,Values=running" --query "Reservations[].Instances[].InstanceId" --output=text)
if [ -n "$running_instances" ]; then
  aws ec2 terminate-instances --instance-ids $running_instances
fi

nat_gateways=$(aws ec2 describe-nat-gateways --query "NatGateways[].NatGatewayId" --output=text)
for n in ${nat_gateways[@]}; do
  aws ec2 delete-nat-gateway --nat-gateway-id $n
done

aws elb delete-load-balancer --load-balancer-name=mgmt-apiserver
sleep 60

eips=$(aws ec2 describe-addresses --query "Addresses[].AllocationId" --output=text)
for i in ${eips[@]}; do
  aws ec2 release-address --allocation-id $i
done

echo Done.

echo You may need to delete VPC manually from the console

# Some time after NAT and LB are deleted and ENIs are deleted it will be possible to simply delete VPC from the console.
# The command will still hang though, if attempted to delete from CLI:
# $ aws ec2 delete-vpc --vpc-id vpc-<my-vpc-id>
# An error occurred (DependencyViolation) when calling the DeleteVpc operation: The vpc 'vpc-0158ac36bf386c091' has dependencies and cannot be deleted.
# (This is the same error as seen in the CAPA logs in `k get events -n <cluster-ns>`
# In AWS console SGs, IGW, subnets and other dependencies are listed as a warning only, not a blocker.
# They are deleted as part of VPC deletion.
# There is no flag on the CLI that can provide the same behaviour.

# List of resources that were left in VPC after running this script
#    sg- / mgmt-bastion
#    sg- / mgmt-apiserver-lb
#    sg- / mgmt-lb
#    sg- / mgmt-node
#    sg- / mgmt-controlplane
#    igw- / mgmt-igw
#    subnet- / mgmt-subnet-public-ap-southeast-2a
#    subnet- / mgmt-subnet-private-ap-southeast-2a
#    rtb- / mgmt-rt-private-ap-southeast-2a
#    rtb- / mgmt-rt-public-ap-southeast-2a
