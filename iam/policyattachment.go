package iam

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/iam"
)

func CreateRolePolicyAttachment(session client.ConfigProvider, policyArn, roleArn string) (*iam.AttachRolePolicyOutput, error) {
	// Create a IAM service client.
	svc := iam.New(session)

	roleName, err := RoleNamefromARN(roleArn)
	if err != nil {
		return nil, err
	}

	result, err := svc.AttachRolePolicy(&iam.AttachRolePolicyInput{
		PolicyArn: aws.String(policyArn),
		RoleName:  aws.String(roleName),
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DeleteRolePolicyAttachment(session client.ConfigProvider, policyArn string, roleArn string) (*iam.DetachRolePolicyOutput, error) {
	// Create a IAM service client.
	svc := iam.New(session)

	roleName, err := RoleNamefromARN(roleArn)
	if err != nil {
		return nil, err
	}

	res, err := svc.DetachRolePolicy(&iam.DetachRolePolicyInput{
		PolicyArn: aws.String(policyArn),
		RoleName:  aws.String(roleName),
	})
	if err != nil {
		return nil, err
	}

	return res, nil
}
