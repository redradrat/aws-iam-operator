/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type PolicyStatementEffect string

const (
	AllowPolicyStatementEffect PolicyStatementEffect = "Allow"
	DenyPolicyStatementEffect  PolicyStatementEffect = "Deny"
)

const (
	PolicyVersion string = "2012-10-17"
)

// PolicyStatementConditionOperator is the operator for following comparison
// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_condition_operators.html
type PolicyStatementConditionOperator string

// PolicyStatementConditionKey is the key in the Condition comparison
// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_condition-keys.html
type PolicyStatementConditionKey string

type PolicyStatementConditionComparison map[PolicyStatementConditionKey]string

type PolicyStatementCondition map[PolicyStatementConditionOperator]PolicyStatementConditionComparison

type PolicyStatementEntry struct {

	//+kubebuilder:validation:Optional
	//
	// Sid is an optional Statement ID to identify a Statement
	Sid string `json:"sid,omitempty"`

	//+kubebuilder:validation:Required
	//
	// Effect holds the desired effect the statement should ensure
	Effect PolicyStatementEffect `json:"effect,omitempty"`

	//+kubebuilder:validation:Optional
	//
	// Principal denotes an account, user, role, or federated user to which you would
	// like to allow or deny access with a resource-based policy
	Principal map[string]string `json:"principal,omitempty"`

	//+kubebuilder:validation:Required
	//
	// Actions holds the desired effect the statement should ensure
	Actions []string `json:"actions,omitempty"`

	//+kubebuilder:validation:Optional
	//
	// Resources denotes an a list of resources to which the actions apply.
	// If you do not set this value, then the resource to which the action
	// applies is the resource to which the policy is attached to
	Resources []string `json:"resources,omitempty"`

	//+kubebuilder:validation:Optional
	//
	// Conditions specifies the circumstances under which the policy grants permission
	Conditions PolicyStatementCondition `json:"conditions,omitempty"`
}

type PolicyStatement []PolicyStatementEntry

// PolicySpec defines the desired state of Policy
type PolicySpec struct {

	//+kubebuilder:validation:Required
	//
	// Statements holds the list of all the policy statement entries
	Statement PolicyStatement `json:"statement,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=policies,shortName=iampolicy
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ARN",type=string,JSONPath=`.status.arn`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.message`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Last Sync",type=string,JSONPath=`.status.lastSyncAttempt`
// Policy is the Schema for the policies API
type Policy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolicySpec      `json:"spec,omitempty"`
	Status AWSObjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PolicyList contains a list of Policy
type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Policy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Policy{}, &PolicyList{})
}
