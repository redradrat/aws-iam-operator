package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (g *Group) GetStatus() *AWSObjectStatus {
	return &g.Status.AWSObjectStatus
}

func (g *Group) RuntimeObject() client.Object {
	return g
}

func (g *Group) Metadata() metav1.ObjectMeta {
	return g.ObjectMeta
}
