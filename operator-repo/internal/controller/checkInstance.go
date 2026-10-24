package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	computev1 "github.com/sidharth3006/operator-repo/api/v1"
)

func checkEC2InstanceExists(ctx context.Context, instanceId string, ec2Instance *computev1.EC2Instance) (bool, *ec2types.Instance, error) {
	// create the client for ec2 instance
	fmt.Println("Check instance", instanceId)
	ec2Client, err := awsClient(ec2Instance.Spec.Region)
	if err != nil {
		return false, nil, err
	}

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceId},
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"running"},
			},
		},
	}

	result, err := ec2Client.DescribeInstances(ctx, input)

	if err != nil {
		if strings.Contains(err.Error(), "InvalidInstanceID.NotFound") {
			return false, nil, nil
		}
		return false, nil, err
	}

	fmt.Println("Length of Reservations are", len(result.Reservations))

	if len(result.Reservations) == 0 {
		return false, nil, nil
	}

	return true, &result.Reservations[0].Instances[0], nil
}
