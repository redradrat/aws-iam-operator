package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/redradrat/cloud-objects/aws/iam"
)

func (r *Role) GetStatus() *AWSObjectStatus {
	return &r.Status.AWSObjectStatus
}

func (r *Role) RuntimeObject() runtime.Object {
	return r
}

func (r *Role) Metadata() metav1.ObjectMeta {
	return r.ObjectMeta
}

func (r *Role) Marshal() iam.PolicyDocument {
	policyDocument := iam.PolicyDocument{}

	var policyStatement []iam.StatementEntry
	for _, entry := range r.Spec.AssumeRolePolicy {
		policyStatement = append(policyStatement, iam.StatementEntry{
			Sid:       entry.Sid,
			Effect:    entry.Effect.String(),
			Principal: entry.Principal,
			Action:    entry.Actions,
			Resource:  entry.Resources,
			Condition: entry.Conditions.Normalize(),
		})
	}

	policyDocument = iam.PolicyDocument{
		Version:   PolicyVersion,
		Statement: policyStatement,
	}

	return policyDocument
}
