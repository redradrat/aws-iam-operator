package iam

import (
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/iam"
)

func IsErrAndFound(err error) bool {
	if err != nil {
		if err.(awserr.Error).Code() != iam.ErrCodeNoSuchEntityException {
			return true
		}
	}
	return false
}

type PolicyDocument struct {
	Version   string           `json:"Version,omitempty"`
	Statement []StatementEntry `json:"Statement,omitempty"`
}

type StatementEntry struct {
	Sid       string                       `json:"Sid,omitempty"`
	Effect    string                       `json:"Effect,omitempty"`
	Principal map[string]string            `json:"Principal,omitempty"`
	Action    []string                     `json:"Action,omitempty"`
	Resource  []string                     `json:"Resource,omitempty"`
	Condition map[string]map[string]string `json:"Condition,omitempty"`
}
