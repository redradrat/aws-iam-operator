package controllers

import (
	"context"
	"time"

	awssdk "github.com/aws/aws-sdk-go/aws"
	awsarn "github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iamv1beta1 "github.com/redradrat/aws-iam-operator/api/v1beta1"
	"github.com/redradrat/aws-iam-operator/aws"
)

type AWSObject interface {
	Metadata() metav1.ObjectMeta
	GetStatus() *iamv1beta1.AWSObjectStatus
	RuntimeObject() runtime.Object
}

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

type PreFunc func(obj AWSObject, c client.Client, ctx context.Context) error

func EmptyPreFunc(obj AWSObject, c client.Client, ctx context.Context) error {
	return nil
}

func CreateAWSObject(obj AWSObject, ins aws.Instance, preFunc PreFunc, c client.Client, ctx context.Context, sw client.StatusWriter) error {

	obj.GetStatus().State = iamv1beta1.SyncSyncState
	obj.GetStatus().LastSyncAttempt = time.Now().Format(time.RFC822Z)

	session, err := startReconciliation()
	if err != nil {
		return err
	}

	if err := preFunc(obj, c, ctx); err != nil {
		return err
	}
	arn, err := ins.Create(session)
	if err != nil {
		return err
	}

	obj.GetStatus().ARN = arn.String()
	obj.GetStatus().Message = "Successfully reconciled"
	obj.GetStatus().State = iamv1beta1.OkSyncState
	err = sw.Update(ctx, obj.RuntimeObject())
	if err != nil {
		return err
	}

	return nil
}

func DeleteAWSObject(obj AWSObject, ins aws.Instance, preFunc PreFunc, c client.Client, ctx context.Context) error {
	obj.GetStatus().State = iamv1beta1.SyncSyncState
	obj.GetStatus().LastSyncAttempt = time.Now().Format(time.RFC822Z)

	session, err := startReconciliation()
	if err != nil {
		return err
	}

	arn, err := awsarn.Parse(obj.GetStatus().ARN)
	if err != nil {
		return err
	}

	if err := preFunc(obj, c, ctx); err != nil {
		return err
	}
	if err = ins.Delete(session, arn); err != nil {
		return err
	}

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

func startReconciliation() (*session.Session, error) {
	session, err := session.NewSession(&awssdk.Config{
		Region: awssdk.String("eu-west-1")},
	)
	if err != nil {
		return nil, err
	}

	return session, nil
}
