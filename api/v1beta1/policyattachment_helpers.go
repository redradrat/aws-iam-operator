package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (pa *PolicyAttachment) GetStatus() *AWSObjectStatus {
	return &pa.Status
}

func (pa *PolicyAttachment) RuntimeObject() runtime.Object {
	return pa
}

func (pa *PolicyAttachment) Metadata() metav1.ObjectMeta {
	return pa.ObjectMeta
}
