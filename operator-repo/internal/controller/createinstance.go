package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	computev1 "github.com/sidharth3006/operator-repo/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func createEc2Instance(ec2Instance *computev1.EC2Instance) (createdInstanceInfo *computev1.CreatedInstanceInfo, err error) {

	log := log.Log.WithName("createEc2Instance")

	log.Info("=== Starting EC2 instance creation ===",
		"amiId", ec2Instance.Spec.AMIID,
		"instanceType", ec2Instance.Spec.InstanceType,
		"region", ec2Instance.Spec.Region,
	)

	//create the client for ec2 instance
	ec2Client, err := awsClient(ec2Instance.Spec.Region)
	if err != nil {
		return nil, err
	}

	// create the input to run the instances

	runInput := &ec2.RunInstancesInput{
		ImageId:      aws.String(ec2Instance.Spec.AMIID),
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		InstanceType: ec2types.InstanceType(ec2Instance.Spec.InstanceType),
		KeyName:      aws.String(ec2Instance.Spec.KeyPair),
		SubnetId:     aws.String(ec2Instance.Spec.Subnet),
	}

	log.Info("=== Calling AWS RunInstances API ===")
	// run the instances

	result, err := ec2Client.RunInstances(context.TODO(), runInput)

	if err != nil {
		log.Error(err, "Failed to create EC2 instance")
		return nil, err
	}

	if len(result.Instances) == 0 {
		log.Error(nil, "No instances returned")
		return nil, fmt.Errorf("no instances returned")
	}

	//Till here, the instance is created and we have
	// Instance ID,private dns, IP , instance type and iamge id.

	inst := result.Instances[0]

	log.Info("=== EC2 Instance created successfully ===", "instanceID", *inst.InstanceId)

	// Now we need to wait for the instance to be running and then get the public ip and dns
	log.Info("=== Waiting for instance to be running ===")

	runWaiter := ec2.NewInstanceRunningWaiter(ec2Client)

	maxWaitTime := 3 * time.Minute

	err = runWaiter.Wait(context.TODO(), &ec2.DescribeInstancesInput{
		InstanceIds: []string{*inst.InstanceId},
	}, maxWaitTime)
	if err != nil {
		log.Error(err, "Failed to wait for instance to be running")
		return nil, err
	}
	// After creating the instance, we waited and now we describe to
	// 1. Get the public IP and dns as it takes some time for it
	// 2. Getting the state of the instance
	// We do this so we can send the instance's state to the status of CR. For user to see with kubectl get ec2Instances

	log.Info("=== Calling AWS DescribeInstances API to get instance details ===")
	describeInput := &ec2.DescribeInstancesInput{
		InstanceIds: []string{*inst.InstanceId},
	}

	describeResult, err := ec2Client.DescribeInstances(context.TODO(), describeInput)
	if err != nil {
		log.Error(err, "Failed to describe instances")
		return nil, err
	}

	fmt.Println("Describe Result", "public ip", *describeResult.Reservations[0].Instances[0].PublicIpAddress, "state", describeResult.Reservations[0].Instances[0].State.Name)

	fmt.Printf("Private IP of the instance: %v", derefString(inst.PrivateIpAddress))
	fmt.Printf("State of the instance :%v", describeResult.Reservations[0].Instances[0].State.Name)
	fmt.Printf("Private DNS of the instance: %v", derefString(inst.PrivateDnsName))
	fmt.Printf("Public DNS of the instance: %v", derefString(inst.PublicDnsName))
	fmt.Printf("Public IP of the instance: %v", derefString(inst.PublicIpAddress))
	fmt.Printf("Image ID of the instance: %v", derefString(inst.ImageId))
	fmt.Printf("Key nme of the instance: %v", derefString(inst.KeyName))

	// Get the instance details safely (public IP/DNS might be nil for private subnets)
	instance := describeResult.Reservations[0].Instances[0]
	createdInstanceInfo = &computev1.CreatedInstanceInfo{
		InstanceId: *inst.InstanceId,
		State:      string(instance.State.Name),
		PublicIp:   derefString(instance.PublicIpAddress),
	}

	log.Info("=== EC2 Instance Creation Completed ===",
		"instanceID", createdInstanceInfo.InstanceId,
		"state", createdInstanceInfo.State,
		"publicIP", createdInstanceInfo.PublicIp,
	)
	return createdInstanceInfo, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
