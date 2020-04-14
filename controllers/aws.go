package controllers

import (
	"context"
	"fmt"
	"reflect"
	"time"

	awssdk "github.com/aws/aws-sdk-go/aws"
	awsclient "github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iamv1beta1 "github.com/redradrat/aws-iam-operator/api/v1beta1"
	"github.com/redradrat/aws-iam-operator/aws/iam"
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
			return "", errWithStatus(obj, err, sw, ctx)
		}
	}

	arn, err := reconcileFunc(session, obj.Metadata().Name)
	if err != nil {
		return "", errWithStatus(obj, err, sw, ctx)
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
			return errWithStatus(obj, err, sw, ctx)
		}
	}

	log.Info(fmt.Sprintf("Deleted policy '%s'", arn))

	return nil
}

func errWithStatus(obj AWSObject, err error, sw client.StatusWriter, ctx context.Context) error {
	origerr := err
	obj.GetStatus().Message = origerr.Error()
	obj.GetStatus().State = iamv1beta1.ErrorSyncState
	if err = sw.Update(ctx, obj.RuntimeObject()); err != nil {
		return err
	}
	return origerr
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

func DeletePolicy(policy *iamv1beta1.Policy, ctx context.Context, c client.Client, sw client.StatusWriter, log logr.Logger) error {
	attachments := iamv1beta1.PolicyAttachmentList{}
	if err := c.List(ctx, &attachments); err != nil {
		return err
	}
	for _, att := range attachments.Items {
		if att.Spec.PolicyReference.Name == policy.Name && att.Spec.PolicyReference.Namespace == policy.Namespace {
			err := fmt.Errorf(fmt.Sprintf("cannot delete policy due to existing PolicyAttachment '%s/%s'", policy.Name, policy.Namespace))
			return errWithStatus(policy, err, sw, ctx)
		}
	}

	df := func(session awsclient.ConfigProvider, arn string) error {
		_, err := iam.DeletePolicy(session, arn)
		if iam.IsErrAndFound(err) {
			return err
		}

		return nil
	}

	if err := deleteAWSObject(policy, ctx, sw, log, df); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Deleted policy '%s'", policy.Status.ARN))

	return nil
}

func ReconcileRole(role *iamv1beta1.Role, ctx context.Context, c client.Client, sw client.StatusWriter, log logr.Logger) error {

	var p iam.PolicyDocument
	if len(role.Spec.AssumeRolePolicy) != 0 {
		if !reflect.DeepEqual(role.Spec.AssumeRolePolicyReference, iamv1beta1.ResourceReference{}) {
			err := fmt.Errorf("only one specification of AssumeRolePolicy and AssumeRolePolicyReference is allowed")
			return errWithStatus(role, err, sw, ctx)
		}
		p = role.Marshal()
	}
	if len(role.Spec.AssumeRolePolicy) == 0 {
		if reflect.DeepEqual(role.Spec.AssumeRolePolicyReference, iamv1beta1.ResourceReference{}) {
			err := fmt.Errorf("specification of either AssumeRolePolicy or AssumeRolePolicyReference is mandatory")
			return errWithStatus(role, err, sw, ctx)
		}
		var assumeRolePolicy iamv1beta1.AssumeRolePolicy
		arpr := role.Spec.AssumeRolePolicyReference
		if err := c.Get(ctx, client.ObjectKey{Name: arpr.Name, Namespace: arpr.Namespace}, &assumeRolePolicy); err != nil {
			return errWithStatus(role, err, sw, ctx)
		}
		p = assumeRolePolicy.Marshal()
	}

	rf := func(session awsclient.ConfigProvider, name string) (string, error) {
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

func DeleteRole(role *iamv1beta1.Role, ctx context.Context, c client.Client, sw client.StatusWriter, log logr.Logger) error {
	attachments := iamv1beta1.PolicyAttachmentList{}
	if err := c.List(ctx, &attachments); err != nil {
		return err
	}
	for _, att := range attachments.Items {
		if att.Spec.TargetReference.Name == role.Name && att.Spec.TargetReference.Namespace == role.Namespace {
			err := fmt.Errorf(fmt.Sprintf("cannot delete role due to existing PolicyAttachment '%s/%s'", role.Name, role.Namespace))
			return errWithStatus(role, err, sw, ctx)
		}
	}

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
	roles := iamv1beta1.RoleList{}
	if err := c.List(ctx, &roles); err != nil {
		return err
	}
	policies := iamv1beta1.PolicyList{}
	if err := c.List(ctx, &policies); err != nil {
		return err
	}

	foundrole := false
	foundpolicy := false
	for _, role := range roles.Items {
		tarref := policyAttachment.Spec.TargetReference
		if tarref.Name == role.Name && tarref.Namespace == role.Namespace {
			foundrole = true
		}
	}
	for _, policy := range policies.Items {
		polref := policyAttachment.Spec.PolicyReference
		if polref.Name == policy.Name && polref.Namespace == policy.Namespace {
			foundpolicy = true
		}
	}
	if !(foundrole == true && foundpolicy == true) {
		err := fmt.Errorf(fmt.Sprintf("defined references do not exist for PolicyAttachment '%s/%s", policyAttachment.Name, policyAttachment.Namespace))
		return errWithStatus(policyAttachment, err, sw, ctx)
	}

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
	session, err := session.NewSession(&awssdk.Config{
		Region: awssdk.String("eu-west-1")},
	)
	if err != nil {
		return nil, err
	}

	return session, nil
}
