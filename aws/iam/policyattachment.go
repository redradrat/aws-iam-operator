package iam

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	awsarn "github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/iam"
)

func CreateRolePolicyAttachment(session client.ConfigProvider, policyArn, roleArn awsarn.ARN) (*iam.AttachRolePolicyOutput, error) {
	// Create a IAM service client.
	svc := iam.New(session)

	roleName, err := RoleNamefromARN(roleArn)
	if err != nil {
		return nil, err
	}

	result, err := svc.AttachRolePolicy(&iam.AttachRolePolicyInput{
		PolicyArn: aws.String(policyArn.String()),
		RoleName:  aws.String(roleName),
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DeleteRolePolicyAttachment(session client.ConfigProvider, policyArn, roleArn awsarn.ARN) (*iam.DetachRolePolicyOutput, error) {
	// Create a IAM service client.
	svc := iam.New(session)

	roleName, err := RoleNamefromARN(roleArn)
	if err != nil {
		return nil, err
	}

	res, err := svc.DetachRolePolicy(&iam.DetachRolePolicyInput{
		PolicyArn: aws.String(policyArn.String()),
		RoleName:  aws.String(roleName),
	})
	if err != nil {
		return nil, err
	}

	return res, nil
}

const (
	RoleAttachmentType AttachmentType = "role"
)

type AttachmentType string

type PolicyAttachmentInstance struct {
	PolicyRef awsarn.ARN
	Type      AttachmentType
	Ref       awsarn.ARN
}

func NewPolicyAttachmentInstance(policyRef awsarn.ARN, attType AttachmentType, ref awsarn.ARN) *PolicyAttachmentInstance {
	return &PolicyAttachmentInstance{PolicyRef: policyRef, Ref: ref, Type: attType}
}

//  Create attaches the referenced policy on referenced target type and returns the target ARN
func (pa *PolicyAttachmentInstance) Create(session client.ConfigProvider) (awsarn.ARN, error) {
	var newarn awsarn.ARN
	switch pa.Type {
	case RoleAttachmentType:
		_, err := CreateRolePolicyAttachment(session, pa.PolicyRef, pa.Ref)
		if err != nil {
			return newarn, err
		}
		newarn = pa.Ref
	default:
		return newarn, fmt.Errorf("PolicyAttachent not supported for specified type '%s'", pa.Type)
	}

	return newarn, nil
}

// Update for PolicyAttachmentInstance doesn't do anything and returns the target ARN
func (pa *PolicyAttachmentInstance) Update(session client.ConfigProvider, arn awsarn.ARN) (awsarn.ARN, error) {
	// PolicyAttachment not updateable
	return arn, nil
}

// Delete removes the referenced Policy from referenced target type and returns the target ARN
func (pa *PolicyAttachmentInstance) Delete(session client.ConfigProvider, arn awsarn.ARN) error {
	switch pa.Type {
	case RoleAttachmentType:
		_, err := DeleteRolePolicyAttachment(session, pa.PolicyRef, arn)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("PolicyAttachent not supported for specified type '%s'", pa.Type)
	}
	return nil
}
