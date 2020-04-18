package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (g *Group) GetStatus() *AWSObjectStatus {
	return &g.Status.AWSObjectStatus
}

func (g *Group) RuntimeObject() runtime.Object {
	return g
}

func (g *Group) Metadata() metav1.ObjectMeta {
	return g.ObjectMeta
}
