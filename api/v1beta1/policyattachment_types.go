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

type ResourceReference struct {

	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`

	// +kubebuilder:validation:Required
	Namespace string `json:"namespace,omitempty"`
}

type TargetType string

const (
	RoleTargetType  TargetType = "Role"
	UserTargetType  TargetType = "User"
	GroupTargetType TargetType = "Group"
)

type TargetReference struct {

	// +kubebuilder:validation:Required
	//
	// Type specifies the target type of the Refrence e.g. User/Role/Group
	Type TargetType `json:"type,omitempty"`

	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`

	// +kubebuilder:validation:Required
	Namespace string `json:"namespace,omitempty"`
}

// PolicyAttachmentSpec defines the desired state of PolicyAttachment
type PolicyAttachmentSpec struct {

	// +kubebuilder:validation:Required
	//
	// PolicyReference refrences the Policy resource to attach to another resource
	PolicyReference ResourceReference `json:"policy,omitempty"`

	// +kubebuilder:validation:Required
	//
	// Attachments holds all defined attachments
	TargetReference TargetReference `json:"target,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=policyattachments,shortName=iampolicyattachment
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ARN",type=string,JSONPath=`.status.arn`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.message`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Last Sync",type=string,JSONPath=`.status.lastSyncAttempt`

// PolicyAttachment is the Schema for the policyattachments API
type PolicyAttachment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolicyAttachmentSpec `json:"spec,omitempty"`
	Status AWSObjectStatus      `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PolicyAttachmentList contains a list of PolicyAttachment
type PolicyAttachmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PolicyAttachment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PolicyAttachment{}, &PolicyAttachmentList{})
}
