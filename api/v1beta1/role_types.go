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

// RoleSpec defines the desired state of Role
type RoleSpec struct {

	// +kubebuilder:validation:Optional
	//
	// AssumeRolePolicy holds the Trust Policy statement for the role
	AssumeRolePolicy AssumeRolePolicyStatement `json:"assumeRolePolicy,omitempty"`

	// +kubebuilder:validation:Optional
	//
	// AssumeRolePolicyReference references a Policy resource to use as AssumeRolePolicy
	AssumeRolePolicyReference ResourceReference `json:"assumeRolePolicyRef,omitempty"`

	// CreateServiceAccount triggers the creation of an annotated ServiceAccount for the created role
	CreateServiceAccount bool `json:"createServiceAccount,omitempty"`

	// AddIRSAPolicy adds the assume-role-policy statement to the trust policy.
	AddIRSAPolicy bool `json:"addIRSAPolicy,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable
	// MaxSessionDuration specifies the maximum duration a session with this role assumed can last
	MaxSessionDuration *int64 `json:"maxSessionDuration,omitempty"`

	// +kubebuilder:validation:Optional
	//
	// Description holds the description string for the Role
	Description string `json:"description,omitempty"`

	// +kubebuilder:validation:Optional
	//
	// AWSRoleName is the name of the role to create. If not specified, metadata.name will be used
	AWSRoleName string `json:"awsRoleName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=roles,shortName=iamrole
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ARN",type=string,JSONPath=`.status.arn`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.message`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Last Sync",type=string,JSONPath=`.status.lastSyncAttempt`
//
// Role is the Schema for the roles API
type Role struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RoleSpec   `json:"spec,omitempty"`
	Status RoleStatus `json:"status,omitempty"`
}

type RoleStatus struct {
	AWSObjectStatus             `json:",inline"`
	ReadAssumeRolePolicyVersion string `json:"ReadAssumeRolePolicyVersion"`
}

// +kubebuilder:object:root=true

// RoleList contains a list of Role
type RoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Role `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Role{}, &RoleList{})
}
