package aws

import (
	"fmt"

	awsarn "github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/client"
)

type Instance interface {
	Create(session client.ConfigProvider) (awsarn.ARN, error)
	Update(session client.ConfigProvider, arn awsarn.ARN) (awsarn.ARN, error)
	Delete(session client.ConfigProvider, arn awsarn.ARN) error
}

// ARNify turns a list of string inputs into a list of parsed ARNs
func ARNify(input ...string) ([]awsarn.ARN, error) {
	var arns []awsarn.ARN
	for i, str := range input {
		if !awsarn.IsARN(str) {
			return arns, fmt.Errorf("input '%s' at index '%d' is not a valid ARN", str, i)
		}
		arn, err := awsarn.Parse(str)
		if err != nil {
			return arns, err
		}
		arns = append(arns, arn)
	}
	return arns, nil
}
