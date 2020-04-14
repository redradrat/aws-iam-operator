package iam

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	awsarn "github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/iam"
)

func CreateRole(session client.ConfigProvider, rn string, roleDesc string, pd PolicyDocument) (*iam.CreateRoleOutput, error) {
	// Create a IAM service client.
	svc := iam.New(session)

	b, err := json.Marshal(&pd)
	if err != nil {
		return nil, err
	}

	result, err := svc.CreateRole(&iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(string(b)),
		Description:              aws.String(roleDesc),
		RoleName:                 aws.String(rn),
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func UpdateRole(session client.ConfigProvider, roleArn awsarn.ARN, roleDesc string) (*iam.UpdateRoleOutput, error) {
	// Create a IAM service client.
	svc := iam.New(session)

	roleName, err := RoleNamefromARN(roleArn)
	if err != nil {
		return nil, err
	}

	result, err := svc.UpdateRole(&iam.UpdateRoleInput{
		Description: aws.String(roleDesc),
		RoleName:    aws.String(roleName),
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DeleteRole(session client.ConfigProvider, roleArn awsarn.ARN) (*iam.DeleteRoleOutput, error) {
	// Create a IAM service client.
	svc := iam.New(session)

	roleName, err := RoleNamefromARN(roleArn)
	if err != nil {
		return nil, err
	}

	res, err := svc.DeleteRole(&iam.DeleteRoleInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		return nil, err
	}

	return res, nil
}

func GetRole(session client.ConfigProvider, roleName string) (*iam.GetRoleOutput, error) {

	// Create a IAM service client.
	svc := iam.New(session)

	result, err := svc.GetRole(&iam.GetRoleInput{
		RoleName: aws.String(roleName),
	})

	if err != nil {
		fmt.Println("Error", err)
		return nil, err
	}

	return result, nil
}

func RoleNamefromARN(arn awsarn.ARN) (string, error) {
	splitres := strings.Split(arn.Resource, "/")
	return splitres[len(splitres)-1], nil
}

type RoleInstance struct {
	Name           string
	Description    string
	PolicyDocument PolicyDocument
}

func NewRoleInstance(name string, description string, poldoc PolicyDocument) *RoleInstance {
	return &RoleInstance{Name: name, Description: description, PolicyDocument: poldoc}
}

// Reconcile creates or updates an AWS Role
func (r *RoleInstance) Create(session client.ConfigProvider) (awsarn.ARN, error) {
	var newarn awsarn.ARN
	out, err := CreateRole(session, r.Name, r.Description, r.PolicyDocument)
	if err != nil {
		return newarn, err
	}
	newarn, err = awsarn.Parse(aws.StringValue(out.Role.Arn))
	if err != nil {
		return newarn, err
	}
	return newarn, nil
}

func (r *RoleInstance) Update(session client.ConfigProvider, arn awsarn.ARN) (awsarn.ARN, error) {
	UpdateRole(session, arn, r.Description)
	return arn, nil
}

func (r *RoleInstance) Delete(session client.ConfigProvider, arn awsarn.ARN) error {
	DeleteRole(session, arn)
	return nil
}
