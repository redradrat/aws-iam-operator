package v1beta1

import (
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/redradrat/cloud-objects/aws/iam"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (pa *PolicyAttachment) GetStatus() *AWSObjectStatus {
	return &pa.Status
}

func (pa *PolicyAttachment) RuntimeObject() client.Object {
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
	case UserTargetType:
		attachmentType = iam.UserAttachmentType
	case GroupTargetType:
		attachmentType = iam.GroupAttachmentType
	default:
		return attachmentType, fmt.Errorf("unsupported TargetType '%s'", targetType)
	}
	return attachmentType, nil
}
