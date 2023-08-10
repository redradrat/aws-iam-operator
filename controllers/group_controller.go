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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/redradrat/cloud-objects/aws"
	"github.com/redradrat/cloud-objects/aws/iam"

	iamv1beta1 "github.com/redradrat/aws-iam-operator/api/v1beta1"
)

// GroupReconciler reconciles a Group object
type GroupReconciler struct {
	client.Client
	Log            logr.Logger
	Region         string
	Scheme         *runtime.Scheme
	ResourcePrefix string
}

// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=groups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=groups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=policyattachments,verbs=get;list;watch
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=policyattachments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=users,verbs=get;list;watch
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=users/status,verbs=get;update;patch

func (r *GroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("group", req.NamespacedName)

	var group iamv1beta1.Group
	err := r.Get(ctx, req.NamespacedName, &group)
	if err != nil {
		log.V(1).Info("unable to fetch Group")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Get our actual IAM Service to communicate with AWS; we don't need to continue without it
	iamsvc, err := IAMService(r.Region)
	if err != nil {
		return ctrl.Result{}, errWithStatus(&group, err, r.Status(), ctx)
	}

	// new group instance
	var ins *iam.GroupInstance
	groupName := r.ResourcePrefix + group.Name
	if group.Status.ARN != "" {
		parsedArn, err := aws.ARNify(group.Status.ARN)
		if err != nil {
			return ctrl.Result{}, errWithStatus(&group, fmt.Errorf("ARN in Group status is not valid/parsable"), r.Status(), ctx)
		}
		ins = iam.NewExistingGroupInstance(groupName, parsedArn[len(parsedArn)-1])
	} else {
		ins = iam.NewGroupInstance(groupName)
	}

	// return if only status/metadata updated
	if group.Status.ObservedGeneration == group.ObjectMeta.Generation && group.Status.State == iamv1beta1.OkSyncState {
		return ctrl.Result{}, nil
	}

	// the finalizer for deleting the actual aws resources
	groupsFinalizer := "group.aws-aws-iam.redradrat.xyz"

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

	// Now add all required users
	for _, user := range group.Spec.Users {
		// Get the User object
		userObj := iamv1beta1.User{}
		r.Client.Get(ctx, client.ObjectKey{Name: user.Name, Namespace: user.Namespace}, &userObj)
		if err != nil {
			return ctrl.Result{}, errWithStatus(&group, err, r.Status(), ctx)
		}

		// Err if ARN is not available in the user obj
		if userObj.Status.ARN == "" {
			return ctrl.Result{}, errWithStatus(&group, fmt.Errorf("referenced user resource '%s/%s' has not yet been created", user.Namespace, user.Name), r.Status(), ctx)
		}

		// parse the user arn
		parsedArn, err := aws.ARNify(userObj.Status.ARN)
		if err != nil {
			return ctrl.Result{}, errWithStatus(&group, fmt.Errorf("ARN in referenced User status is not valid/parsable"), r.Status(), ctx)
		}

		// Now add the user to our Group Instance
		if err = ins.AddUser(iamsvc, parsedArn[len(parsedArn)-1]); err != nil {
			return ctrl.Result{}, errWithStatus(&group, err, r.Status(), ctx)
		}
	}

	group.Status.ObservedGeneration = group.ObjectMeta.Generation
	if err := r.Status().Update(ctx, &group); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
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
