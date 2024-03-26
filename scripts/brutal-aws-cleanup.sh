#!/bin/bash

echo EXIT.
echo
echo Read the script and uncomment exit statement below.
echo
echo This script deletes all EC2 instances, NATs, EIPs, ELBs, Security groups VPCs by VPC_ID of the CAPI clusters or by Cluster API tag
echo
echo Make sure to delete the kind cluster before running this script, otherwise CAPI will be recreating the cluster.
exit 1

set -eoux pipefail

main() {

vpc_ids=$(aws ec2 describe-vpcs --filters "Name=tag:sigs.k8s.io/cluster-api-provider-aws/role,Values=*" --query "Vpcs[].VpcId" --output text)

for vpc_id in $vpc_ids; do
  echo "Processing VPC: $vpc_id"
  retry delete_compute_resources $vpc_id
  retry delete_vpc $vpc_id
done

} # end main

delete_compute_resources() {
  vpc_id=$1
  terminate_ec2_instances $vpc_id
  delete_nat_gateways $vpc_id
  delete_elbs $vpc_id
  release_ips $vpc_id
}

delete_vpc() {
  vpc_id=$1
  delete_subnets $vpc_id
  delete_route_tables $vpc_id
  delete_internet_gateways $vpc_id
  delete_security_groups $vpc_id

  echo "Deleting VPC: $vpc_id"
  aws ec2 delete-vpc --vpc-id $vpc_id
}

terminate_ec2_instances() {
  vpc_id=$1
  echo "Terminating running EC2 instances..."
  running_instances=$(aws ec2 describe-instances --filters "Name=instance-state-name,Values=running" "Name=tag:sigs.k8s.io/cluster-api-provider-aws/role,Values=*" --query "Reservations[].Instances[].InstanceId" --output=text)
  if [ -n "$running_instances" ]; then
    # /dev/null is neccessary to avoid script execution pause when the output is larger than the buffer
    # if that happens the terminal will wait for the user to scroll to the end of the output before proceeding.
    aws ec2 terminate-instances --instance-ids $running_instances > /dev/null
  fi
}

delete_nat_gateways() {
  vpc_id=$1
  echo "Deleting NAT gateways..."
  nat_gateways=$(aws ec2 describe-nat-gateways --filter "Name=vpc-id,Values=$vpc_id" --query "NatGateways[].NatGatewayId" --output=text)
  for n in ${nat_gateways[@]}; do
    aws ec2 delete-nat-gateway --nat-gateway-id $n
  done
}

delete_elbs() {
  echo "Deleting Elastic Load Balancers..."
  # TODO
  aws elb delete-load-balancer --load-balancer-name=cluster-mgmt-apiserver
  aws elb delete-load-balancer --load-balancer-name=cluster-01-apiserver
}


# Some time after NAT and LB are deleted and ENIs are deleted it will be possible to delete VPC from the console.
# The command will still hang though, if attempted to delete from CLI:
# $ aws ec2 delete-vpc --vpc-id vpc-<my-vpc-id>
# An error occurred (DependencyViolation) when calling the DeleteVpc operation: The vpc 'vpc-<id>' has dependencies and cannot be deleted.
# (This is the same error as seen in the CAPA logs in `k get events -n <cluster-ns>`
# In AWS console SGs, IGW, subnets and other dependencies are listed as a warning only, not a blocker.
# There is no flag on the CLI that can provide the same behaviour.
# Further reading on why deleting VPC is a tricky business: echo https://github.com/isovalent/aws-delete-vpc

delete_subnets() {
  vpc_id=$1
  subnet_ids=$(aws ec2 describe-subnets --filters "Name=vpc-id,Values=$vpc_id" "Name=tag:sigs.k8s.io/cluster-api-provider-aws/role,Values=*" --query "Subnets[].SubnetId" --output text)
  for subnet_id in $subnet_ids; do
    aws ec2 delete-subnet --subnet-id $subnet_id
  done
}

delete_route_tables() {
  vpc_id=$1
  route_table_ids=$(aws ec2 describe-route-tables --filters "Name=vpc-id,Values=$vpc_id" "Name=tag:sigs.k8s.io/cluster-api-provider-aws/role,Values=*" --query "RouteTables[].RouteTableId" --output text)
  for rtb_id in $route_table_ids; do
    aws ec2 delete-route-table --route-table-id $rtb_id
  done
}

delete_internet_gateways() {
  vpc_id=$1
  igw_ids=$(aws ec2 describe-internet-gateways --filters "Name=attachment.vpc-id,Values=$vpc_id" "Name=tag:sigs.k8s.io/cluster-api-provider-aws/role,Values=*" --query "InternetGateways[].InternetGatewayId" --output text)
  for igw_id in $igw_ids; do
    echo "Detaching and deleting internet gateway: $igw_id"
    aws ec2 detach-internet-gateway --internet-gateway-id $igw_id --vpc-id $vpc_id
    aws ec2 delete-internet-gateway --internet-gateway-id $igw_id
  done
}

# Function to remove all dependencies from each security group
clear_security_group_rules() {
  sg_id=$1

  echo "Processing security group: $sg_id"
  rules=$(aws ec2 describe-security-group-rules --filters "Name=group-id,Values=$sg_id" --query "SecurityGroupRules[?ReferencedGroupInfo.GroupId != null].{RuleId: SecurityGroupRuleId, ReferencedGroupId: ReferencedGroupInfo.GroupId}" --output text)

  while read -r rule_id referenced_group_id; do
    if [ -n "$rule_id" ] && [ -n "$referenced_group_id" ]; then
      echo "Removing rule $rule_id referencing $referenced_group_id from security group $sg_id"
      # To delete cleanly determine if it's an ingress or egress rule here, then revoke appropriately
      # But for "brutal" approach opportunistically trying both directions is good enough
      aws ec2 revoke-security-group-ingress --group-id $sg_id --security-group-rule-ids $referenced_group_id
      aws ec2 revoke-security-group-egress --group-id $sg_id --security-group-rule-ids $referenced_group_id
    fi
  done <<< "$rules"
}

# Function to delete security groups in a VPC
delete_security_groups() {
  vpc_id=$1
  sg_ids=$(aws ec2 describe-security-groups --filters "Name=vpc-id,Values=$vpc_id" "Name=tag:sigs.k8s.io/cluster-api-provider-aws/role,Values=*" --query "SecurityGroups[?GroupName != 'default'].GroupId" --output text)

  # First, clear all rules in the security groups
  for sg_id in $sg_ids; do
    echo "Clearing rules for security group: $sg_id"
    clear_security_group_rules $sg_id
  done

  # Now, delete the security groups
  for sg_id in $sg_ids; do
    echo "Deleting security group: $sg_id"
    aws ec2 delete-security-group --group-id $sg_id
  done
}

# TODO - delete by vpc id or tag
release_ips() {
  eips=$(aws ec2 describe-addresses --query "Addresses[].AllocationId" --output=text)
  for i in ${eips[@]}; do
    aws ec2 release-address --allocation-id $i
  done
}

retry() {
  local count=1
  local max=5
  local delay=10
  set +e
  while true; do
    "$@" && break || {
      if [[ $count -lt $max ]]; then
        ((count++))
        echo "Command failed. Attempt $count/$max:"
        sleep $delay;
      else
        echo "The command has failed after $count attempts."
        return 1
      fi
    }
  set -e
  done
}

main
