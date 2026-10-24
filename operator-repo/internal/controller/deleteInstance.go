package controller

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	computev1 "github.com/sidharth3006/operator-repo/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func deleteEc2Instance(ctx context.Context, ec2Instance *computev1.EC2Instance) (bool, error) {
	l := log.FromContext(ctx)

	l.Info("Deleting EC2 instance", "instanceID", ec2Instance.Status.InstanceId)

	// create the client for ec2 instance
	ec2Client, err := awsClient(ec2Instance.Spec.Region)
	if err != nil {
		l.Error(err, "Failed to create AWS client")
		return false, err
	}

	// Terminate the instance
	terminateResult, err := ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{ec2Instance.Status.InstanceId},
	})

	if err != nil {
		l.Error(err, "Failed to terminate EC2 instance")
		return false, err
	}

	l.Info("Instance termination initiated",
		"instanceID", ec2Instance.Status.InstanceId,
		"currentState", terminateResult.TerminatingInstances[0].CurrentState.Name)

	// Use the AWS SDK v2 waiter to efficiently wait for instance termination
	// The waiter uses exponential backoff and is more efficient than manual polling
	waiter := ec2.NewInstanceTerminatedWaiter(ec2Client)

	// Configure waiter options
	maxWaitTime := 5 * time.Minute // Maximum time to wait for termination
	waitParams := &ec2.DescribeInstancesInput{
		InstanceIds: []string{ec2Instance.Status.InstanceId},
	}

	l.Info("Waiting for instance to be terminated",
		"instanceID", ec2Instance.Status.InstanceId,
		"maxWaitTime", maxWaitTime)

	// Wait for the instance to be terminated
	err = waiter.Wait(ctx, waitParams, maxWaitTime)

	if err != nil {
		l.Error(err, "Failed while waiting for instance termination",
			"instanceID", ec2Instance.Status.InstanceId)
		return false, err
	}

	l.Info("EC2 instance successfully terminated", "instanceID", ec2Instance.Status.InstanceId)
	return true, nil
}
