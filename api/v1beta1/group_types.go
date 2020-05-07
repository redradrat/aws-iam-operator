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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GroupSpec defines the desired state of Group
type GroupSpec struct {

	// Users holds the list of all Users to be added the group
	// +kubebuilder:validation:optional
	Users []v1.ObjectReference `json:"users,omitempty"`
}

type GroupStatus struct {
	AWSObjectStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=groups,shortName=iamgroup
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ARN",type=string,JSONPath=`.status.arn`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.message`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Last Sync",type=string,JSONPath=`.status.lastSyncAttempt`
//
// Group is the Schema for the roles API
type Group struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GroupSpec   `json:"spec,omitempty"`
	Status GroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GroupList contains a list of Group
type GroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Group `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Group{}, &GroupList{})
}
