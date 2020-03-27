package iam_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	awsiam "github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/redradrat/aws-iam-operator/api/v1beta1"
	iam "github.com/redradrat/aws-iam-operator/iam"
)

///////////
// MOCKS //
///////////

type mockIAMClient struct {
	iamiface.IAMAPI
	t *testing.T
}

const (
	MockPolicyName = "thisismypolicy"
	MockPolicyArn  = ""
)

func (m *mockIAMClient) CreatePolicy(input *awsiam.CreatePolicyInput) (*awsiam.CreatePolicyOutput, error) {
	// Check if input values are still as we want them to be
	assert.Equal(m.t, *input.PolicyName, MockPolicyName)
	marshaledPolicy := getMockPolicy().Marshal()
	b, err := json.Marshal(&marshaledPolicy)
	assert.NoError(m.t, err)
	assert.True(m.t, reflect.DeepEqual(input.PolicyDocument, aws.String(string(b))))

	out := getMockCreatePolicyOutput(input)
	return out, nil
}

func (m *mockIAMClient) DeletePolicy(input *awsiam.DeletePolicyInput) (*awsiam.DeletePolicyOutput, error) {
	// Check if input values are still as we want them to be
	assert.True(m.t, arn.IsARN(*input.PolicyArn))
	assert.Equal(m.t, input.PolicyArn, aws.String(getMockPolicyArn()))

	return &awsiam.DeletePolicyOutput{}, nil
}

///////////
// TESTS //
///////////

func TestCreatePolicyWithoutSvc(t *testing.T) {
	// Setup Test
	mockSvc := &mockIAMClient{t: t}

	session, err := session.NewSession(&aws.Config{
		Region: aws.String("eu-west-1")},
	)
	assert.NoError(t, err)

	pd := getMockPolicy().Marshal()
	pn := "thisismypolicy"
	b, err := json.Marshal(&pd)
	assert.NoError(t, err)
	out, err := iam.CreatePolicyWithoutSvc(mockSvc, session, pn, pd)
	assert.True(t, reflect.DeepEqual(getMockCreatePolicyOutput(&awsiam.CreatePolicyInput{
		Description:    nil,
		Path:           nil,
		PolicyDocument: aws.String(string(b)),
		PolicyName:     aws.String(pn),
	}), out))
	assert.Nil(t, out.Policy.Description)
	assert.Nil(t, out.Policy.Path)
}

func TestDeletePolicyWithoutSvc(t *testing.T) {
	// Setup Test
	mockSvc := &mockIAMClient{t: t}

	session, err := session.NewSession(&aws.Config{
		Region: aws.String("eu-west-1")},
	)
	assert.NoError(t, err)

	assert.NoError(t, err)
	_, err = iam.DeletePolicyWithoutSvc(mockSvc, session, getMockPolicyArn())
	assert.NoError(t, err)

}

/////////////
// HELPERS //
/////////////

func getMockPolicyArn() string {
	return fmt.Sprintf("arn:aws:iam::123456789012:policy/%s", MockPolicyName)
}

func getMockCreatePolicyOutput(input *awsiam.CreatePolicyInput) *awsiam.CreatePolicyOutput {
	date := time.Date(2016, 11, 5, 7, 50, 22, 8, time.UTC)
	return &awsiam.CreatePolicyOutput{
		Policy: &awsiam.Policy{
			Arn:                           aws.String(getMockPolicyArn()),
			AttachmentCount:               aws.Int64(0),
			CreateDate:                    aws.Time(date),
			DefaultVersionId:              nil,
			Description:                   input.Description,
			IsAttachable:                  nil,
			Path:                          input.Path,
			PermissionsBoundaryUsageCount: nil,
			PolicyId:                      aws.String("AROA1234567890EXAMPLE"),
			PolicyName:                    aws.String(MockPolicyName),
			UpdateDate:                    nil,
		},
	}
}

func getMockPolicy() *v1beta1.Policy {
	return &v1beta1.Policy{
		ObjectMeta: v1.ObjectMeta{
			Name:      "testpolicy",
			Namespace: "testnamespace",
		},
		Spec: v1beta1.PolicySpec{
			Statement: []v1beta1.PolicyStatementEntry{
				v1beta1.PolicyStatementEntry{
					Sid:    "",
					Effect: v1beta1.AllowPolicyStatementEffect,
					Actions: []string{
						"god:Mode",
					},
					Resources: []string{
						"*",
					},
					Conditions: nil,
				},
			},
		},
	}
}
