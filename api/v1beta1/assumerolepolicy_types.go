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

type AssumeRolePolicyStatementEntry struct {
	PolicyStatementEntry `json:",inline"`

	//+kubebuilder:validation:Required
	//
	// Principal denotes an account, user, role, or federated user to which you would
	// like to allow or deny access with a resource-based policy
	Principal map[string]string `json:"principal,omitempty"`
}

type AssumeRolePolicyStatement []AssumeRolePolicyStatementEntry

// AssumeRolePolicySpec defines the desired state of AssumeRolePolicy
type AssumeRolePolicySpec struct {

	//+kubebuilder:validation:Required
	//
	// Statements holds the list of all the policy statement entries
	Statement AssumeRolePolicyStatement `json:"statement,omitempty"`
}

// AssumeRolePolicyStatus defines the observed state of AssumeRolePolicy
type AssumeRolePolicyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// AssumeRolePolicy is the Schema for the assumerolepolicies API
type AssumeRolePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AssumeRolePolicySpec   `json:"spec,omitempty"`
	Status AssumeRolePolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AssumeRolePolicyList contains a list of AssumeRolePolicy
type AssumeRolePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AssumeRolePolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AssumeRolePolicy{}, &AssumeRolePolicyList{})
}
