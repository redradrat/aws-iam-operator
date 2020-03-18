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

func CreateRole(session client.ConfigProvider, rn string, pd PolicyDocument) (*iam.CreateRoleOutput, error) {
	// Create a IAM service client.
	svc := iam.New(session)

	b, err := json.Marshal(&pd)
	if err != nil {
		return nil, err
	}

	result, err := svc.CreateRole(&iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(string(b)),
		RoleName:                 aws.String(rn),
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DeleteRole(session client.ConfigProvider, roleArn string) (*iam.DeleteRoleOutput, error) {
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

func RoleNamefromARN(arn string) (string, error) {
	thisarn, err := awsarn.Parse(arn)
	if err != nil {
		return "", err
	}
	splitres := strings.Split(thisarn.Resource, "/")
	return splitres[len(splitres)-1], nil
}
