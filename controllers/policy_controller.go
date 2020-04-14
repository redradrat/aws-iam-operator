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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iamv1beta1 "github.com/redradrat/aws-iam-operator/api/v1beta1"
	"github.com/redradrat/aws-iam-operator/aws"
)

// PolicyReconciler reconciles a Policy object
type PolicyReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=policies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=policies/status,verbs=get;update;patch

func (r *PolicyReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("policy", req.NamespacedName)

	var policy iamv1beta1.Policy
	err := r.Get(ctx, req.NamespacedName, &policy)
	if err != nil {
		log.V(1).Info("unable to fetch Policy")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// return if only status/metadata updated
	if policy.Status.ObservedGeneration == policy.ObjectMeta.Generation && policy.Status.State == aws.OkSyncState {
		return ctrl.Result{}, nil
	} else {
		policy.Status.ObservedGeneration = policy.ObjectMeta.Generation
		r.Status().Update(ctx, &policy)
	}

	// the finalizer for deleting the actual aws resources
	policiesFinalizer := "policy.aws-iam.redradrat.xyz"

	// Check Deletion and finalizer
	if policy.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !containsString(policy.ObjectMeta.Finalizers, policiesFinalizer) {
			policy.ObjectMeta.Finalizers = append(policy.ObjectMeta.Finalizers, policiesFinalizer)
			if err := r.Update(context.Background(), &policy); err != nil {
				log.Error(err, "unable to register finalizer for Policy")
				return ctrl.Result{}, err
			}
		}
	} else {
		if containsString(policy.ObjectMeta.Finalizers, policiesFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := DeletePolicy(&policy, ctx, r.Client, r.Status(), log); err != nil {
				// retry
				log.Error(err, "unable to delete Policy")
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			policy.ObjectMeta.Finalizers = removeString(policy.ObjectMeta.Finalizers, policiesFinalizer)
			if err := r.Update(context.Background(), &policy); err != nil {
				log.Error(err, "unable to remove finalizer from Policy")
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// RECONCILE THE RESOURCE
	err = ReconcilePolicy(&policy, ctx, r.Status(), log)
	if err != nil {
		log.Error(err, "unable to reconcile Policy")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, nil
}

func (r *PolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&iamv1beta1.Policy{}).
		Complete(r)
}
