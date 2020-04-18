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
	"reflect"
	"time"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/redradrat/cloud-objects/aws"
	"github.com/redradrat/cloud-objects/aws/iam"

	iamv1beta1 "github.com/redradrat/aws-iam-operator/api/v1beta1"
)

const RequeueInterval time.Duration = 20 * time.Second

// GroupReconciler reconciles a Group object
type GroupReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=groups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=groups/status,verbs=get;update;patch

func (r *GroupReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("group", req.NamespacedName)

	var group iamv1beta1.Group
	err := r.Get(ctx, req.NamespacedName, &group)
	if err != nil {
		log.V(1).Info("unable to fetch Group")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if len(group.Spec.UserReferences) == 0 && len(group.Spec.UserSelector) == 0 {
		return ctrl.Result{}, fmt.Errorf("neither userReferences nor userSelector defined")
	}

	if len(group.Spec.UserReferences) != 0 && len(group.Spec.UserSelector) != 0 {
		return ctrl.Result{}, fmt.Errorf("both userReferences and userSelector defined")
	}

	// Get our actual IAM Service to communicate with AWS; we don't need to continue without it
	iamsvc, err := IAMService()
	if err != nil {
		return ctrl.Result{}, errWithStatus(&group, err, r.Status(), ctx)
	}

	// new group instance
	var ins *iam.GroupInstance
	if group.Status.ARN != "" {
		parsedArn, err := aws.ARNify(group.Status.ARN)
		if err != nil {
			return ctrl.Result{}, errWithStatus(&group, fmt.Errorf("ARN in Group status is not valid/parsable"), r.Status(), ctx)
		}
		ins = iam.NewExistingGroupInstance(group.Name, parsedArn[len(parsedArn)-1])
	} else {
		ins = iam.NewGroupInstance(group.Name)
	}

	var users []v1.ObjectReference
	if len(group.Spec.UserReferences) == 0 {
		for _, usr := range group.Spec.UserReferences {
			users = append(users, v1.ObjectReference{Name: usr.Name, Namespace: usr.Namespace})
		}
	} else {
		usrList := iamv1beta1.UserList{}
		if err = r.List(ctx, &usrList, client.MatchingLabels(group.Spec.UserSelector)); err != nil {
			ErrorStatusUpdater(err.Error())(ins, &group, ctx, r.Status(), log)
			return ctrl.Result{}, err
		}
		for _, usr := range usrList.Items {
			users = append(users, v1.ObjectReference{Name: usr.Name, Namespace: usr.Namespace})
		}
	}

	// return if only status/metadata updated
	if reflect.DeepEqual(users, group.Status.UsersAdded) && group.Status.ObservedGeneration == group.ObjectMeta.Generation && group.Status.State == iamv1beta1.OkSyncState {
		return ctrl.Result{}, nil
	} else {
		group.Status.ObservedGeneration = group.ObjectMeta.Generation
		r.Status().Update(ctx, &group)
	}

	// the finalizer for deleting the actual aws resources
	groupsFinalizer := "group.aws-iam.redradrat.xyz"

	cleanupFunc := groupCleanup(r, ctx, group)

	// Check Deletion and finalizer
	if group.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !containsString(group.ObjectMeta.Finalizers, groupsFinalizer) {
			group.ObjectMeta.Finalizers = append(group.ObjectMeta.Finalizers, groupsFinalizer)
			if err := r.Update(context.Background(), &group); err != nil {
				log.Error(err, "unable to register finalizer for Group")
				return ctrl.Result{}, err
			}
		}
	} else {
		if containsString(group.ObjectMeta.Finalizers, groupsFinalizer) {
			// our finalizer is present, so lets handle any external dependency

			// delete the actual AWS Object and pass the cleanup function
			statusUpdater, err := DeleteAWSObject(iamsvc, ins, cleanupFunc)
			// we got a StatusUpdater function returned... let's execute it
			statusUpdater(ins, &group, ctx, r.Status(), log)
			if err != nil {
				// we had an error during AWS Object deletion... so we return here to retry
				log.Error(err, "unable to delete Group")
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			group.ObjectMeta.Finalizers = removeString(group.ObjectMeta.Finalizers, groupsFinalizer)
			if err := r.Update(context.Background(), &group); err != nil {
				log.Error(err, "unable to remove finalizer from Group")
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// RECONCILE THE RESOURCE

	// if there is already an ARN in our status, then we recreate the object completely
	// (because AWS only supports description updates)
	if group.Status.ARN != "" {
		// Delete the actual AWS Object and pass the cleanup function
		statusWriter, err := DeleteAWSObject(iamsvc, ins, cleanupFunc)
		statusWriter(ins, &group, ctx, r.Status(), log)
		if err != nil {
			// we had an error during AWS Object deletion... so we return here to retry
			log.Error(err, "error while deleting Group during reconciliation")
			return ctrl.Result{}, err
		}
	}

	statusWriter, err := CreateAWSObject(iamsvc, ins, DoNothingPreFunc)
	statusWriter(ins, &group, ctx, r.Status(), log)
	if err != nil {
		log.Error(err, "error while creating Group during reconciliation")
		return ctrl.Result{}, err
	}

	// Add all referenced Users to the Group
	for _, ref := range users {
		usr := iamv1beta1.User{}
		if err = r.Client.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: ref.Namespace}, &usr); err != nil {
			ErrorStatusUpdater(err.Error())(ins, &group, ctx, r.Status(), log)
			return ctrl.Result{}, err
		}
		usrarn, err := arn.Parse(usr.Status.ARN)
		if err != nil {
			ErrorStatusUpdater(err.Error())(ins, &group, ctx, r.Status(), log)
			return ctrl.Result{}, err
		}
		if err = ins.AddUser(iamsvc, usrarn); err != nil {
			ErrorStatusUpdater(err.Error())(ins, &group, ctx, r.Status(), log)
			return ctrl.Result{}, err
		}
	}

	group.Status.UsersAdded = users
	if err = r.Status().Update(ctx, &group); err != nil {
		ErrorStatusUpdater(err.Error())(ins, &group, ctx, r.Status(), log)
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: RequeueInterval}, nil
}

func (r *GroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&iamv1beta1.Group{}).
		Complete(r)
}

// Returns a function, that does everything necessary before we can delete our actual User (cleanup)
func groupCleanup(r *GroupReconciler, ctx context.Context, group iamv1beta1.Group) func() error {
	return func() error {
		attachments := iamv1beta1.PolicyAttachmentList{}
		if err := r.List(ctx, &attachments); err != nil {
			return err
		}
		for _, att := range attachments.Items {
			if att.Spec.TargetReference.Type == iamv1beta1.GroupTargetType {
				if att.Spec.TargetReference.Name == group.Name && att.Spec.TargetReference.Namespace == group.Namespace {
					err := fmt.Errorf(fmt.Sprintf("cannot delete Group due to existing PolicyAttachment '%s/%s'", att.Name, att.Namespace))
					return err
				}
			}
		}

		return nil
	}
}
