package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/redradrat/aws-iam-operator/iam"
)

func (pse PolicyStatementEffect) String() string {
	return string(pse)
}

func (psc PolicyStatementCondition) Normalize() map[string]map[string]string {
	out := make(map[string]map[string]string)

	for k, v := range psc {
		for ki, vi := range v {
			out[string(k)] = make(map[string]string)
			out[string(k)][string(ki)] = vi
		}
	}

	return out
}

func (p *Policy) GetStatus() *AWSObjectStatus {
	return &p.Status
}

func (p *Policy) RuntimeObject() runtime.Object {
	return p
}

func (p *Policy) Metadata() metav1.ObjectMeta {
	return p.ObjectMeta
}

func (p *Policy) Marshal() iam.PolicyDocument {
	policyDocument := iam.PolicyDocument{}

	var policyStatement []iam.StatementEntry
	for _, entry := range p.Spec.Statement {
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
