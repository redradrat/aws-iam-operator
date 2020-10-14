/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/redradrat/cloud-objects/aws"
	"github.com/redradrat/cloud-objects/aws/iam"

	iamv1beta1 "github.com/redradrat/aws-iam-operator/api/v1beta1"
)

const (
	LoginSecretSuffix        = "-login"
	LoginSecretUserKey       = "username"
	LoginSecretPassKey       = "password"
	AccesskeySecretSuffix    = "-accesskey"
	AccesskeySecretIdKey     = "id"
	AccesskeySecretSecretKey = "secret"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Log            logr.Logger
	Region         string
	Scheme         *runtime.Scheme
	ResourcePrefix string
}

// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=users,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=users/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=policyattachments,verbs=get;list;watch
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=policyattachments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts/status,verbs=get;update;patch

func (r *UserReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("user", req.NamespacedName)

	var user iamv1beta1.User
	err := r.Get(ctx, req.NamespacedName, &user)
	if err != nil {
		log.V(1).Info("unable to fetch User")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// return if only status/metadata updated
	if user.Status.ObservedGeneration == user.ObjectMeta.Generation && user.Status.State == iamv1beta1.OkSyncState {
		return ctrl.Result{}, nil
	} else {
		user.Status.ObservedGeneration = user.ObjectMeta.Generation
		r.Status().Update(ctx, &user)
	}

	// the finalizer for deleting the actual aws resources
	usersFinalizer := "user.aws-iam.redradrat.xyz"

	// Get our actual IAM Service to communicate with AWS; we don't need to continue without it
	iamsvc, err := IAMService(r.Region)
	if err != nil {
		return ctrl.Result{}, errWithStatus(&user, err, r.Status(), ctx)
	}

	// new user instance
	userName := r.ResourcePrefix + user.Name
	var ins *iam.UserInstance
	if user.Status.ARN != "" {
		parsedArn, err := aws.ARNify(user.Status.ARN)
		if err != nil {
			return ctrl.Result{}, errWithStatus(&user, fmt.Errorf("ARN in User status is not valid/parsable"), r.Status(), ctx)
		}
		ins = iam.NewExistingUserInstance(userName, user.Spec.CreateLoginProfile, user.Status.LoginProfileCreated, user.Spec.CreateProgrammaticAccess, user.Status.ProgrammaticAccessCreated, parsedArn[len(parsedArn)-1])
	} else {
		ins = iam.NewUserInstance(userName, user.Spec.CreateLoginProfile, user.Spec.CreateProgrammaticAccess)
	}

	cleanupFunc := userCleanup(r, ctx, user)

	// Check Deletion and finalizer
	if user.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !containsString(user.ObjectMeta.Finalizers, usersFinalizer) {
			user.ObjectMeta.Finalizers = append(user.ObjectMeta.Finalizers, usersFinalizer)
			if err := r.Update(context.Background(), &user); err != nil {
				log.Error(err, "unable to register finalizer for User")
				return ctrl.Result{}, err
			}
		}
	} else {
		if containsString(user.ObjectMeta.Finalizers, usersFinalizer) {
			// our finalizer is present, so lets handle any external dependency

			// delete the actual AWS Object and pass the cleanup function
			statusUpdater, err := DeleteAWSObject(iamsvc, ins, cleanupFunc)
			// we got a StatusUpdater function returned... let's execute it
			statusUpdater(ins, &user, ctx, r.Status(), log)
			if err != nil {
				// we had an error during AWS Object deletion... so we return here to retry
				log.Error(err, "unable to delete User")
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			user.ObjectMeta.Finalizers = removeString(user.ObjectMeta.Finalizers, usersFinalizer)
			if err := r.Update(context.Background(), &user); err != nil {
				log.Error(err, "unable to remove finalizer from User")
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// RECONCILE THE RESOURCE

	loginSecret := user.Name + LoginSecretSuffix
	accessKeySecret := user.Name + AccesskeySecretSuffix

	// if there is already an ARN in our status, then we recreate the object completely
	// (because AWS only supports description updates)
	if user.Status.ARN != "" {
		// User already exists; we need to update it
		statusUpdater, err := UpdateAWSObject(iamsvc, ins, DoNothingPreFunc)
		statusUpdater(ins, &user, ctx, r.Status(), log)
		if err != nil {
			log.Error(err, "error while updating User during reconciliation")
			return ctrl.Result{}, err
		}
	} else {
		// User does not yet exist, let's create it
		statusUpdater, err := CreateAWSObject(iamsvc, ins, DoNothingPreFunc)
		statusUpdater(ins, &user, ctx, r.Status(), log)
		if err != nil {
			log.Error(err, "error while creating User during reconciliation")
			return ctrl.Result{}, err
		}
	}

	// Create Secret if Login Profile
	if user.Spec.CreateLoginProfile {
		if !user.Status.LoginProfileCreated {
			data := map[string]string{LoginSecretUserKey: ins.LoginProfileCredentials().Username(), LoginSecretPassKey: ins.LoginProfileCredentials().Password()}
			sec := userSecret(data, loginSecret, user.Namespace)
			if err = ctrl.SetControllerReference(&user, sec, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err = r.Client.Create(ctx, sec); err != nil {
				return ctrl.Result{}, err
			}
			user.Status.LoginProfileCreated = true
			user.Status.LoginProfileSecret = v1.SecretReference{Name: sec.Name, Namespace: sec.Namespace}
			r.Status().Update(ctx, &user)
		}
	} else {
		sec := &v1.Secret{}
		if err = r.Client.Get(ctx, client.ObjectKey{Name: loginSecret, Namespace: user.Namespace}, sec); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
		if !errors.IsNotFound(err) {
			if err = r.Client.Delete(ctx, sec); client.IgnoreNotFound(err) != nil {
				return ctrl.Result{}, err
			}
			user.Status.LoginProfileCreated = false
			user.Status.LoginProfileSecret = v1.SecretReference{}
			r.Status().Update(ctx, &user)
		}
	}

	if user.Spec.CreateProgrammaticAccess {
		if !user.Status.ProgrammaticAccessCreated {
			data := map[string]string{AccesskeySecretIdKey: ins.AccessKey().Id(), AccesskeySecretSecretKey: ins.AccessKey().Secret()}
			sec := userSecret(data, accessKeySecret, user.Namespace)
			if err = ctrl.SetControllerReference(&user, sec, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err = r.Client.Create(ctx, sec); err != nil {
				return ctrl.Result{}, err
			}
			user.Status.ProgrammaticAccessCreated = true
			user.Status.ProgrammaticAccessSecret = v1.SecretReference{Name: sec.Name, Namespace: sec.Namespace}
			r.Status().Update(ctx, &user)
		}
	} else {
		sec := &v1.Secret{}
		if err = r.Client.Get(ctx, client.ObjectKey{Name: accessKeySecret, Namespace: user.Namespace}, sec); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
		if !errors.IsNotFound(err) {
			if err = r.Client.Delete(ctx, sec); client.IgnoreNotFound(err) != nil {
				return ctrl.Result{}, err
			}
			user.Status.ProgrammaticAccessCreated = false
			user.Status.ProgrammaticAccessSecret = v1.SecretReference{}
			r.Status().Update(ctx, &user)
		}
	}

	log.Info(fmt.Sprintf("Created User '%s'", user.Status.ARN))
	return ctrl.Result{}, nil
}

func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&iamv1beta1.User{}).
		Complete(r)
}

// Returns a function, that does everything necessary before we can delete our actual User (cleanup)
func userCleanup(r *UserReconciler, ctx context.Context, user iamv1beta1.User) func() error {
	return func() error {
		attachments := iamv1beta1.PolicyAttachmentList{}
		if err := r.List(ctx, &attachments); err != nil {
			return err
		}
		for _, att := range attachments.Items {
			if att.Spec.TargetReference.Type == iamv1beta1.UserTargetType {
				if att.Spec.TargetReference.Name == user.Name && att.Spec.TargetReference.Namespace == user.Namespace {
					err := fmt.Errorf(fmt.Sprintf("cannot delete User due to existing PolicyAttachment '%s/%s'", att.Name, att.Namespace))
					return err
				}
			}
		}

		return nil
	}
}

func userSecret(data map[string]string, name, namespace string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		StringData: data,
		Type:       v1.SecretTypeOpaque,
	}
}
