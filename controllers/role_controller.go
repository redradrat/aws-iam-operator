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

	"github.com/go-logr/logr"
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
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=roles/status,verbs=get;update;patch

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
	policiesFinalizer := "role.aws-iam.redradrat.xyz"

	// Check Deletion and finalizer
	if role.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !containsString(role.ObjectMeta.Finalizers, policiesFinalizer) {
			role.ObjectMeta.Finalizers = append(role.ObjectMeta.Finalizers, policiesFinalizer)
			if err := r.Update(context.Background(), &role); err != nil {
				log.Error(err, "unable to register finalizer for Role")
				return ctrl.Result{}, err
			}
		}
	} else {
		if containsString(role.ObjectMeta.Finalizers, policiesFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := DeleteRole(&role, ctx, r.Status(), log); err != nil {
				// retry
				log.Error(err, "unable to delete Role")
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			role.ObjectMeta.Finalizers = removeString(role.ObjectMeta.Finalizers, policiesFinalizer)
			if err := r.Update(context.Background(), &role); err != nil {
				log.Error(err, "unable to remove finalizer from Role")
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// RECONCILE THE RESOURCE
	err = ReconcileRole(&role, ctx, r.Client, r.Status(), log)
	if err != nil {
		log.Error(err, "unable to reconcile Role")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	truevar := true
	gvk, err := apiutil.GVKForObject(&role, r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create ServiceAccount for Role
	ownerRef := metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               role.GetName(),
		UID:                role.GetUID(),
		BlockOwnerDeletion: &truevar,
		Controller:         &truevar,
	}
	if err = createRoleServiceAccount(role, ctx, r.Client, ownerRef); err != nil {
		log.Error(err, "unable to create ServiceAccount for Role")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, nil
}

func (r *RoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&iamv1beta1.Role{}).
		Complete(r)
}
