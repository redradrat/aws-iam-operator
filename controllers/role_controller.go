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

	"github.com/go-logr/logr"
	"github.com/redradrat/cloud-objects/aws"
	"github.com/redradrat/cloud-objects/aws/iam"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	iamv1beta1 "github.com/redradrat/aws-iam-operator/api/v1beta1"
)

// RoleReconciler reconciles a Role object
type RoleReconciler struct {
	client.Client
	Log            logr.Logger
	Scheme         *runtime.Scheme
	ResourcePrefix string
}

// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=roles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=policyattachments,verbs=get;list;watch
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=policyattachments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=assumerolepolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=assumerolepolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets/status,verbs=get;update;patch

func (r *RoleReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("role", req.NamespacedName)

	var role iamv1beta1.Role
	err := r.Get(ctx, req.NamespacedName, &role)
	if err != nil {
		log.V(1).Info("unable to fetch Role")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// return if only status/metadata updated
	if role.Status.ObservedGeneration == role.ObjectMeta.Generation && role.Status.State == iamv1beta1.OkSyncState {
		return ctrl.Result{}, nil
	} else {
		role.Status.ObservedGeneration = role.ObjectMeta.Generation
		r.Status().Update(ctx, &role)
	}

	// the finalizer for deleting the actual aws resources
	rolesFinalizer := "role.aws-iam.redradrat.xyz"

	// Get our actual IAM Service to communicate with AWS; we don't need to continue without it
	iamsvc, err := IAMService()
	if err != nil {
		return ctrl.Result{}, errWithStatus(&role, err, r.Status(), ctx)
	}

	// get the policy doc
	polDoc, err := getPolicyDoc(&role, r.Client, ctx)
	if err != nil {
		return ctrl.Result{}, errWithStatus(&role, err, r.Status(), ctx)
	}

	// new role instance
	var ins *iam.RoleInstance
	roleName := r.ResourcePrefix + role.Name
	if role.Status.ARN != "" {
		parsedArn, err := aws.ARNify(role.Status.ARN)
		if err != nil {
			return ctrl.Result{}, errWithStatus(&role, fmt.Errorf("ARN in Role status is not valid/parsable"), r.Status(), ctx)
		}
		ins = iam.NewExistingRoleInstance(roleName, role.Spec.Description, polDoc, parsedArn[len(parsedArn)-1])
	} else {
		ins = iam.NewRoleInstance(roleName, role.Spec.Description, polDoc)
	}

	cleanupFunc := roleCleanup(r, ctx, role)

	// Check Deletion and finalizer
	if role.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !containsString(role.ObjectMeta.Finalizers, rolesFinalizer) {
			role.ObjectMeta.Finalizers = append(role.ObjectMeta.Finalizers, rolesFinalizer)
			if err := r.Update(context.Background(), &role); err != nil {
				log.Error(err, "unable to register finalizer for Role")
				return ctrl.Result{}, err
			}
		}
	} else {
		if containsString(role.ObjectMeta.Finalizers, rolesFinalizer) {
			// our finalizer is present, so lets handle any external dependency

			// delete the actual AWS Object and pass the cleanup function
			statusUpdater, err := DeleteAWSObject(iamsvc, ins, cleanupFunc)
			// we got a StatusUpdater function returned... let's execute it
			statusUpdater(ins, &role, ctx, r.Status(), log)
			if err != nil {
				// we had an error during AWS Object deletion... so we return here to retry
				log.Error(err, "unable to delete Role")
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			role.ObjectMeta.Finalizers = removeString(role.ObjectMeta.Finalizers, rolesFinalizer)
			if err := r.Update(context.Background(), &role); err != nil {
				log.Error(err, "unable to remove finalizer from Role")
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// RECONCILE THE RESOURCE

	// if there is already an ARN in our status, then we recreate the object completely
	// (because AWS only supports description updates)
	if role.Status.ARN != "" {
		// delete the actual AWS Object and pass the cleanup function
		statusUpdater, err := DeleteAWSObject(iamsvc, ins, cleanupFunc)
		// we got a StatusUpdater function returned... let's execute it
		statusUpdater(ins, &role, ctx, r.Status(), log)
		if err != nil {
			// we had an error during AWS Object deletion... so we return here to retry
			log.Error(err, "error while deleting Role during reconciliation")
			return ctrl.Result{}, err
		}
	}

	statusUpdater, err := CreateAWSObject(iamsvc, ins, DoNothingPreFunc)
	statusUpdater(ins, &role, ctx, r.Status(), log)
	if err != nil {
		log.Error(err, "error while creating Role during reconciliation")
		return ctrl.Result{}, err
	}

	log.Info(fmt.Sprintf("Created Role '%s'", role.Status.ARN))

	truevar := true
	gvk, err := apiutil.GVKForObject(&role, r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}

	ownerRef := metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               role.GetName(),
		UID:                role.GetUID(),
		BlockOwnerDeletion: &truevar,
		Controller:         &truevar,
	}
	// Create ServiceAccount for Role
	if err = createRoleServiceAccount(role, ctx, r.Client, ownerRef); err != nil {
		log.Error(err, "unable to create ServiceAccount for Role")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, nil
}

// Returns a function, that does everything necessary before we can delete our actual Role (cleanup)
func roleCleanup(r *RoleReconciler, ctx context.Context, role iamv1beta1.Role) func() error {
	return func() error {
		attachments := iamv1beta1.PolicyAttachmentList{}
		if err := r.List(ctx, &attachments); err != nil {
			return err
		}
		for _, att := range attachments.Items {
			if att.Spec.TargetReference.Type == iamv1beta1.RoleTargetType {
				if att.Spec.TargetReference.Name == role.Name && att.Spec.TargetReference.Namespace == role.Namespace {
					err := fmt.Errorf(fmt.Sprintf("cannot delete Role due to existing PolicyAttachment '%s/%s'", att.Name, att.Namespace))
					return err
				}
			}
		}
		return nil
	}
}

func (r *RoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&iamv1beta1.Role{}).
		Complete(r)
}

func getPolicyDoc(role *iamv1beta1.Role, c client.Client, ctx context.Context) (iam.PolicyDocument, error) {
	var p iam.PolicyDocument
	if len(role.Spec.AssumeRolePolicy) != 0 {
		if !reflect.DeepEqual(role.Spec.AssumeRolePolicyReference, iamv1beta1.ResourceReference{}) {
			err := fmt.Errorf("only one specification of AssumeRolePolicy and AssumeRolePolicyReference is allowed")
			return p, err
		}
		p = role.Marshal()
	}
	if len(role.Spec.AssumeRolePolicy) == 0 {
		if reflect.DeepEqual(role.Spec.AssumeRolePolicyReference, iamv1beta1.ResourceReference{}) {
			err := fmt.Errorf("specification of either AssumeRolePolicy or AssumeRolePolicyReference is mandatory")
			return p, err
		}
		var assumeRolePolicy iamv1beta1.AssumeRolePolicy
		arpr := role.Spec.AssumeRolePolicyReference
		if err := c.Get(ctx, client.ObjectKey{Name: arpr.Name, Namespace: arpr.Namespace}, &assumeRolePolicy); err != nil {
			return p, err
		}
		p = assumeRolePolicy.Marshal()
	}
	return p, nil
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
