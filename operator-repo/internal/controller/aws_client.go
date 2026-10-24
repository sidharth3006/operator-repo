package controller

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

func awsClient(region string) (*ec2.Client, error) {
	// Load default AWS configuration
	// This automatically checks:
	// 1. Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
	// 2. AWS credentials file (~/.aws/credentials)
	// 3. IAM role (if running in AWS)
	// 4. Kubernetes secrets (when deployed as pod with env vars from secrets)
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	// create client
	client := ec2.NewFromConfig(cfg)
	return client, nil
}
