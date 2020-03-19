package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/redradrat/aws-iam-operator/iam"
)

func (arp *AssumeRolePolicy) RuntimeObject() runtime.Object {
	return arp
}

func (arp *AssumeRolePolicy) Metadata() metav1.ObjectMeta {
	return arp.ObjectMeta
}

func (arp *AssumeRolePolicy) Marshal() iam.PolicyDocument {
	policyDocument := iam.PolicyDocument{}

	var policyStatement []iam.StatementEntry
	for _, entry := range arp.Spec.Statement {
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
