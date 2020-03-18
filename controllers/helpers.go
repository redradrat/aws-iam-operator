package controllers

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iamv1beta1 "github.com/redradrat/aws-iam-operator/api/v1beta1"
)

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func createRoleServiceAccount(role iamv1beta1.Role, ctx context.Context, client client.Client, ownerRef metav1.OwnerReference) error {
	if role.Spec.CreateServiceAccount {
		sa := v1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      role.Name,
				Namespace: role.Namespace,
				Labels:    role.Labels,
				Annotations: map[string]string{
					"eks.amazonaws.com/role-arn": role.Status.ARN,
				},
				OwnerReferences: []metav1.OwnerReference{ownerRef},
			},
		}

		if err := client.Create(ctx, &sa); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}

	}

	return nil
}
