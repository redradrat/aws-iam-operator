package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	awsarn "github.com/aws/aws-sdk-go/aws/arn"
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

	if role.Spec.CreateServiceAccount {

	}

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
	thisarn, err := awsarn.Parse(arn)
	if err != nil {
		return err
	}
	splitres := strings.Split(thisarn.Resource, "/")

	_, err = iam.DeleteRole(session, splitres[len(splitres)-1])
	if iam.IsErrAndFound(err) {
		return err
	}

	return nil
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
