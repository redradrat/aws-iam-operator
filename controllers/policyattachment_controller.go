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
)

// PolicyAssignmentReconciler reconciles a PolicyAssignment object
type PolicyAttachmentReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=iam.redradrat.xyz,resources=policyassignments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=iam.redradrat.xyz,resources=policyassignments/status,verbs=get;update;patch

func (r *PolicyAttachmentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("policyattachment", req.NamespacedName)

	var policyattachment iamv1beta1.PolicyAttachment
	err := r.Get(ctx, req.NamespacedName, &policyattachment)
	if err != nil {
		log.V(1).Info("unable to fetch PolicyAttachment")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// return if only status/metadata updated
	if policyattachment.Status.ObservedGeneration == policyattachment.ObjectMeta.Generation && policyattachment.Status.State == iamv1beta1.OkSyncState {
		return ctrl.Result{}, nil
	} else {
		policyattachment.Status.ObservedGeneration = policyattachment.ObjectMeta.Generation
		r.Status().Update(ctx, &policyattachment)
	}

	// the finalizer for deleting the actual aws resources
	policyAttachmentFinalizer := "policyattachment.aws-iam.redradrat.xyz"

	// Check Deletion and finalizer
	if policyattachment.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !containsString(policyattachment.ObjectMeta.Finalizers, policyAttachmentFinalizer) {
			policyattachment.ObjectMeta.Finalizers = append(policyattachment.ObjectMeta.Finalizers, policyAttachmentFinalizer)
			if err := r.Update(context.Background(), &policyattachment); err != nil {
				log.Error(err, "unable to register finalizer for PolicyAttachment")
				return ctrl.Result{}, err
			}
		}
	} else {
		if containsString(policyattachment.ObjectMeta.Finalizers, policyAttachmentFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := DeletePolicyAttachment(&policyattachment, ctx, r.Client, r.Status(), log); err != nil {
				// retry
				log.Error(err, "unable to delete PolicyAttachment")
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			policyattachment.ObjectMeta.Finalizers = removeString(policyattachment.ObjectMeta.Finalizers, policyAttachmentFinalizer)
			if err := r.Update(context.Background(), &policyattachment); err != nil {
				log.Error(err, "unable to remove finalizer from PolicyAttachment")
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// RECONCILE THE RESOURCE
	err = ReconcilePolicyAttachment(&policyattachment, ctx, r.Client, r.Status(), log)
	if err != nil {
		log.Error(err, "unable to reconcile PolicyAttachment")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *PolicyAttachmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&iamv1beta1.PolicyAttachment{}).
		Complete(r)
}
