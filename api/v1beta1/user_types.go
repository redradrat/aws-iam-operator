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

// UserSpec defines the desired state of User
type UserSpec struct {
	// CreateLoginProfile triggers the creation of Login Profile in AWS and creates a user/pass secret
	CreateLoginProfile bool `json:"createLoginProfile,omitempty"`

	// CreateProgrammaticAccess triggers the creation of API creds in AWS and creates a cred secret
	CreateProgrammaticAccess bool `json:"createProgrammaticAccess,omitempty"`
}

type UserStatus struct {
	AWSObjectStatus `json:",inline"`

	// +kubebuilder:validation:optional
	//
	// LoginProfileCreated holds info about whether or not a LoginProfile has been created for this user
	LoginProfileCreated bool `json:"loginProfileCreated,omitempty"`

	// +kubebuilder:validation:optional
	//
	// LoginProfileSecret holds the reference to the created LoginProfile Secret
	LoginProfileSecret v1.SecretReference `json:"loginProfileSecret,omitempty"`

	// +kubebuilder:validation:optional
	//
	// ProgrammaticAccessCreated holds info about whether or not programmatic access credentials have been created for this user
	ProgrammaticAccessCreated bool `json:"programmaticAccessCreated,omitempty"`

	// +kubebuilder:validation:optional
	//
	// ProgrammaticAccessSecret holds the reference to the created LoginProfile Secret
	ProgrammaticAccessSecret v1.SecretReference `json:"programmaticAccessSecret,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=users,shortName=iamuser
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ARN",type=string,JSONPath=`.status.arn`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.message`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Last Sync",type=string,JSONPath=`.status.lastSyncAttempt`
//
// User is the Schema for the users API
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec,omitempty"`
	Status UserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UserList contains a list of User
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}
