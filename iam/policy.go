package iam

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/iam"
)

func CreatePolicy(session client.ConfigProvider, pn string, pd PolicyDocument) (*iam.CreatePolicyOutput, error) {
	// Create a IAM service client.
	svc := iam.New(session)

	b, err := json.Marshal(&pd)
	if err != nil {
		return nil, err
	}

	result, err := svc.CreatePolicy(&iam.CreatePolicyInput{
		PolicyDocument: aws.String(string(b)),
		PolicyName:     aws.String(pn),
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DeletePolicy(session client.ConfigProvider, arn string) (*iam.DeletePolicyOutput, error) {
	// Create a IAM service client.
	svc := iam.New(session)

	res, err := svc.DeletePolicy(&iam.DeletePolicyInput{
		PolicyArn: &arn,
	})
	if err != nil {
		return nil, err
	}

	return res, nil
}

func GetPolicy(session client.ConfigProvider, arn string) (*iam.GetPolicyOutput, error) {

	// Create a IAM service client.
	svc := iam.New(session)

	result, err := svc.GetPolicy(&iam.GetPolicyInput{
		PolicyArn: &arn,
	})

	if err != nil {
		fmt.Println("Error", err)
		return nil, err
	}

	return result, nil
}
