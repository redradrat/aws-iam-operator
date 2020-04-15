package v1beta1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/redradrat/aws-iam-operator/aws/iam"
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

func (pa *PolicyAttachment) GetAttachmentType() (iam.AttachmentType, error) {
	var attachmentType iam.AttachmentType
	targetType := pa.Spec.TargetReference.Type
	switch targetType {
	case RoleTargetType:
		attachmentType = iam.RoleAttachmentType
	default:
		return attachmentType, fmt.Errorf("unsupported TargetType '%s'", targetType)
	}
	return attachmentType, nil
}
