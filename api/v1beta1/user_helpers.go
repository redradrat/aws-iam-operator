package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (u *User) GetStatus() *AWSObjectStatus {
	return &u.Status.AWSObjectStatus
}

func (u *User) RuntimeObject() client.Object {
	return u
}

func (u *User) Metadata() metav1.ObjectMeta {
	return u.ObjectMeta
}
