package iam

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	awsarn "github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
)

func CreatePolicy(session client.ConfigProvider, polName, polDesc string, pd PolicyDocument) (*iam.CreatePolicyOutput, error) {
	// Create a IAM service client.
	svc := iam.New(session)

	return CreatePolicyWithoutSvc(svc, session, polName, polDesc, pd)
}

func CreatePolicyWithoutSvc(svc iamiface.IAMAPI, session client.ConfigProvider, polName, polDesc string, pd PolicyDocument) (*iam.CreatePolicyOutput, error) {
	b, err := json.Marshal(&pd)
	if err != nil {
		return nil, err
	}

	result, err := svc.CreatePolicy(&iam.CreatePolicyInput{
		PolicyDocument: aws.String(string(b)),
		Description:    aws.String(polDesc),
		PolicyName:     aws.String(polName),
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func UpdatePolicy(session client.ConfigProvider, policyArn awsarn.ARN, pd PolicyDocument) (*iam.CreatePolicyVersionOutput, error) {
	// Create a IAM service client.
	svc := iam.New(session)

	return UpdatePolicyWithoutSvc(svc, session, policyArn, pd)
}

func UpdatePolicyWithoutSvc(svc iamiface.IAMAPI, session client.ConfigProvider, policyArn awsarn.ARN, pd PolicyDocument) (*iam.CreatePolicyVersionOutput, error) {
	b, err := json.Marshal(&pd)
	if err != nil {
		return nil, err
	}

	result, err := svc.CreatePolicyVersion(&iam.CreatePolicyVersionInput{
		PolicyDocument: aws.String(string(b)),
		PolicyArn:      aws.String(policyArn.String()),
		SetAsDefault:   aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DeletePolicy(session client.ConfigProvider, arn awsarn.ARN) (*iam.DeletePolicyOutput, error) {
	// Create a IAM service client.
	svc := iam.New(session)

	return DeletePolicyWithoutSvc(svc, session, arn)
}

func DeletePolicyWithoutSvc(svc iamiface.IAMAPI, session client.ConfigProvider, arn awsarn.ARN) (*iam.DeletePolicyOutput, error) {
	res, err := svc.DeletePolicy(&iam.DeletePolicyInput{
		PolicyArn: aws.String(arn.String()),
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

type PolicyInstance struct {
	Name           string
	Description    string
	PolicyDocument PolicyDocument
}

func NewPolicyInstance(policyRef awsarn.ARN, attType AttachmentType, ref awsarn.ARN) *PolicyAttachmentInstance {
	return &PolicyAttachmentInstance{PolicyRef: policyRef, Ref: ref, Type: attType}
}

//  Create attaches the referenced policy on referenced target type and returns the target ARN
func (p *PolicyInstance) Create(session client.ConfigProvider) (awsarn.ARN, error) {
	var newarn awsarn.ARN
	out, err := CreatePolicy(session, p.Name, p.Description, p.PolicyDocument)
	if err != nil {
		return newarn, err
	}
	newarn, err = awsarn.Parse(aws.StringValue(out.Policy.Arn))
	if err != nil {
		return newarn, err
	}
	return newarn, nil
}

// Update for PolicyInstance creates a new Policy version an sets it as active; then returns the arn
func (p *PolicyInstance) Update(session client.ConfigProvider, arn awsarn.ARN) (awsarn.ARN, error) {
	var newarn awsarn.ARN
	_, err := UpdatePolicy(session, arn, p.PolicyDocument)
	if err != nil {
		return newarn, err
	}
	newarn = arn
	return arn, nil
}

// Delete removes the referenced Policy from referenced target type and returns the target ARN
func (p *PolicyInstance) Delete(session client.ConfigProvider, arn awsarn.ARN) error {
	DeletePolicy(session, arn)
	return nil
}
