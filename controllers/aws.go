package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	awsclient "github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iamv1beta1 "github.com/redradrat/aws-iam-operator/api/v1beta1"
	"github.com/redradrat/aws-iam-operator/iam"
)

type AWSObject interface {
	Metadata() metav1.ObjectMeta
	GetStatus() *iamv1beta1.AWSObjectStatus
	RuntimeObject() runtime.Object
}

type ReconcileFunc func(session awsclient.ConfigProvider, name string) (string, error)
type DeleteFunc func(session awsclient.ConfigProvider, arn string) error

func reconcileAWSObject(obj AWSObject, ctx context.Context, sw client.StatusWriter, log logr.Logger, reconcileFunc ReconcileFunc, deleteFunc DeleteFunc) (string, error) {
	obj.GetStatus().State = iamv1beta1.SyncSyncState
	obj.GetStatus().LastSyncAttempt = time.Now().Format(time.RFC822Z)

	session, err := startReconciliation()
	if err != nil {
		return "", err
	}

	if obj.GetStatus().ARN != "" {
		err = deleteAWSObject(obj, ctx, sw, log, deleteFunc)
		if err != nil {
			obj.GetStatus().Message = err.Error()
			obj.GetStatus().State = iamv1beta1.ErrorSyncState
			err = sw.Update(ctx, obj.RuntimeObject())
			if err != nil {
				return "", err
			}
			return "", err
		}
	}

	arn, err := reconcileFunc(session, obj.Metadata().Name)
	if err != nil {
		obj.GetStatus().Message = err.Error()
		obj.GetStatus().State = iamv1beta1.ErrorSyncState
		err = sw.Update(ctx, obj.RuntimeObject())
		if err != nil {
			return "", err
		}
		return "", err
	}

	obj.GetStatus().ARN = arn
	obj.GetStatus().Message = "Successfully reconciled"
	obj.GetStatus().State = iamv1beta1.OkSyncState
	err = sw.Update(ctx, obj.RuntimeObject())
	if err != nil {
		return "", err
	}

	return arn, nil
}

func deleteAWSObject(obj AWSObject, ctx context.Context, sw client.StatusWriter, log logr.Logger, deleteFunc DeleteFunc) error {
	obj.GetStatus().State = iamv1beta1.SyncSyncState
	obj.GetStatus().LastSyncAttempt = time.Now().Format(time.RFC822Z)

	session, err := startReconciliation()

	arn := obj.GetStatus().ARN

	if arn != "" {
		if err = deleteFunc(session, arn); err != nil {
			obj.GetStatus().Message = err.Error()
			obj.GetStatus().State = iamv1beta1.ErrorSyncState
			err = sw.Update(ctx, obj.RuntimeObject())
			if err != nil {
				return err
			}
			return err
		}
	}

	log.Info(fmt.Sprintf("Deleted policy '%s'", arn))

	return nil
}

func ReconcilePolicy(policy *iamv1beta1.Policy, ctx context.Context, sw client.StatusWriter, log logr.Logger) error {
	df := func(session awsclient.ConfigProvider, arn string) error {
		_, err := iam.DeletePolicy(session, arn)
		if iam.IsErrAndFound(err) {
			return err
		}

		return nil
	}

	rf := func(session awsclient.ConfigProvider, name string) (string, error) {
		p := policy.Marshal()
		res, err := iam.CreatePolicy(session, policy.Name, p)
		if err != nil {
			return "", err
		}

		return *res.Policy.Arn, nil
	}

	arn, err := reconcileAWSObject(policy, ctx, sw, log, rf, df)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Created policy '%s'", arn))

	return nil
}

func DeletePolicy(policy *iamv1beta1.Policy, ctx context.Context, sw client.StatusWriter, log logr.Logger) error {
	df := func(session awsclient.ConfigProvider, arn string) error {
		_, err := iam.DeletePolicy(session, arn)
		if iam.IsErrAndFound(err) {
			return err
		}

		return nil
	}

	err := deleteAWSObject(policy, ctx, sw, log, df)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Deleted policy '%s'", policy.Status.ARN))

	return nil
}

func ReconcileRole(role *iamv1beta1.Role, ctx context.Context, sw client.StatusWriter, log logr.Logger) error {
	rf := func(session awsclient.ConfigProvider, name string) (string, error) {
		p := role.Marshal()
		res, err := iam.CreateRole(session, role.Name, p)
		if err != nil {
			return "", err
		}

		return *res.Role.Arn, nil
	}

	arn, err := reconcileAWSObject(role, ctx, sw, log, rf, deleteRoleFunc)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Created role '%s'", arn))

	return nil
}

func DeleteRole(role *iamv1beta1.Role, ctx context.Context, sw client.StatusWriter, log logr.Logger) error {
	err := deleteAWSObject(role, ctx, sw, log, deleteRoleFunc)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Deleted policy '%s'", role.Status.ARN))

	return nil
}

func deleteRoleFunc(session awsclient.ConfigProvider, arn string) error {
	_, err := iam.DeleteRole(session, arn)
	if iam.IsErrAndFound(err) {
		return err
	}

	return nil
}

func ReconcilePolicyAttachment(policyAttachment *iamv1beta1.PolicyAttachment, ctx context.Context, c client.Client, sw client.StatusWriter, log logr.Logger) error {
	rf := func(session awsclient.ConfigProvider, name string) (string, error) {

		policyArn, targetArn, err := getPolicyAttachmentARNs(policyAttachment, ctx, c)
		if err != nil {
			return "", err
		}

		switch policyAttachment.Spec.TargetReference.Type {
		case iamv1beta1.RoleTargetType:
			_, err := iam.CreateRolePolicyAttachment(session, policyArn, targetArn)
			if err != nil {
				return "", err
			}

		}

		return policyArn, nil
	}

	df := func(session awsclient.ConfigProvider, arn string) error {
		policyArn, targetArn, err := getPolicyAttachmentARNs(policyAttachment, ctx, c)
		if err != nil {
			return err
		}

		_, err = iam.DeleteRolePolicyAttachment(session, policyArn, targetArn)
		if iam.IsErrAndFound(err) {
			return err
		}

		return nil
	}

	arn, err := reconcileAWSObject(policyAttachment, ctx, sw, log, rf, df)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Created PolicyAttachment for Policy '%s'", arn))

	return nil
}

func DeletePolicyAttachment(policyAttachment *iamv1beta1.PolicyAttachment, ctx context.Context, c client.Client, sw client.StatusWriter, log logr.Logger) error {

	df := func(session awsclient.ConfigProvider, arn string) error {
		policyArn, targetArn, err := getPolicyAttachmentARNs(policyAttachment, ctx, c)
		if err != nil {
			return err
		}

		_, err = iam.DeleteRolePolicyAttachment(session, policyArn, targetArn)
		if iam.IsErrAndFound(err) {
			return err
		}

		return nil
	}

	err := deleteAWSObject(policyAttachment, ctx, sw, log, df)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Deleted PolicyAttachment for Policy '%s'", policyAttachment.Status.ARN))

	return nil
}

func getPolicyAttachmentARNs(policyAttachment *iamv1beta1.PolicyAttachment, ctx context.Context, c client.Client) (string, string, error) {
	var policyArn string
	var targetArn string

	polref := policyAttachment.Spec.PolicyReference
	tarref := policyAttachment.Spec.TargetReference
	policy := iamv1beta1.Policy{}
	if err := c.Get(ctx, client.ObjectKey{Name: polref.Name, Namespace: polref.Namespace}, &policy); err != nil {
		return "", "", err
	}

	policyArn = policy.Status.ARN

	switch policyAttachment.Spec.TargetReference.Type {
	case iamv1beta1.RoleTargetType:
		role := iamv1beta1.Role{}
		if err := c.Get(ctx, client.ObjectKey{Name: tarref.Name, Namespace: tarref.Namespace}, &role); err != nil {
			return "", "", err
		}
		targetArn = role.Status.ARN

	}

	if policyArn == "" {
		return "", "", fmt.Errorf("ARN is empty in status for policy reference")
	}
	if targetArn == "" {
		return "", "", fmt.Errorf("ARN is empty in status for target reference")
	}

	return policyArn, targetArn, nil
}

func startReconciliation() (*session.Session, error) {
	session, err := session.NewSession(&aws.Config{
		Region: aws.String("eu-west-1")},
	)
	if err != nil {
		return nil, err
	}

	return session, nil
}
