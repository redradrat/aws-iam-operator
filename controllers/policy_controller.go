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

	"github.com/aws/aws-sdk-go/aws/awserr"
	awsiam "github.com/aws/aws-sdk-go/service/iam"

	"github.com/go-logr/logr"
	"github.com/redradrat/cloud-objects/aws"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/redradrat/cloud-objects/aws/iam"

	iamv1beta1 "github.com/redradrat/aws-iam-operator/api/v1beta1"
)

// PolicyReconciler reconciles a Policy object
type PolicyReconciler struct {
	client.Client
	Log            logr.Logger
	Region         string
	Scheme         *runtime.Scheme
	ResourcePrefix string
}

// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=policies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=policies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=policyattachments,verbs=get;list;watch
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=policyattachments/status,verbs=get;update;patch

func (r *PolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("policy", req.NamespacedName)

	var policy iamv1beta1.Policy
	err := r.Get(ctx, req.NamespacedName, &policy)
	if err != nil {
		log.V(1).Info("unable to fetch Policy")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// return if only status/metadata updated
	if policy.Status.ObservedGeneration == policy.ObjectMeta.Generation && policy.Status.State == iamv1beta1.OkSyncState {
		return ctrl.Result{}, nil
	}

	// Get our actual IAM Service to communicate with AWS; we don't need to continue without it
	iamsvc, err := IAMService(r.Region)
	if err != nil {
		return ctrl.Result{}, err
	}

	// the finalizer for deleting the actual aws resources
	policiesFinalizer := "policy.aws-iam.redradrat.xyz"

	// now let's instantiate our PolicyInstance
	var ins *iam.PolicyInstance
	policyName := r.ResourcePrefix + policy.Name
	if policy.Status.ARN != "" {
		parsedArn, err := aws.ARNify(policy.Status.ARN)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("ARN in Role status is not valid/parsable")
		}
		ins = iam.NewExistingPolicyInstance(policyName, policy.Spec.Description, policy.Marshal(), parsedArn[len(parsedArn)-1])
	} else {
		ins = iam.NewPolicyInstance(policyName, policy.Spec.Description, policy.Marshal())
	}

	cleanupFunc := policyCleanup(r, ctx, &policy)

	// Check Deletion and finalizer
	if policy.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !containsString(policy.ObjectMeta.Finalizers, policiesFinalizer) {
			policy.ObjectMeta.Finalizers = append(policy.ObjectMeta.Finalizers, policiesFinalizer)
			if err := r.Update(context.Background(), &policy); err != nil {
				log.Error(err, "unable to register finalizer for Policy")
				return ctrl.Result{}, errWithStatus(ctx, &policy, err, r.Status())
			}
		}
	} else {
		if containsString(policy.ObjectMeta.Finalizers, policiesFinalizer) {
			// our finalizer is present, so lets handle any external dependency

			// delete the actual AWS Object and pass the cleanup function
			statusWriter, err := DeleteAWSObject(iamsvc, ins, cleanupFunc)
			statusWriter(ctx, ins, &policy, r.Status(), log)
			if err != nil {
				// we had an error during AWS Object deletion... so we return here to retry
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

	// if there is already an ARN in our status, then we update the object
	statusWriter, err := CreateAWSObject(iamsvc, ins, DoNothingPreFunc)
	statusWriter(ctx, ins, &policy, r.Status(), log)
	if err != nil {
		// If already exists, we just update the status
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == awsiam.ErrCodeEntityAlreadyExistsException {
				// Update the actual AWS Object and pass the DoNothing function
				statusWriter, err := UpdateAWSObject(iamsvc, ins, DoNothingPreFunc)
				statusWriter(ctx, ins, &policy, r.Status(), log)
				if err != nil {
					if theError, ok := err.(awserr.Error); ok {
						if theError.Code() == awsiam.ErrCodeLimitExceededException {
							// We should delete oldest version
							versionID, err := GetOldestPolicyVersion(iamsvc, ins.ARN().String())
							if err == nil {
								_, err := DeletePolicyVersion(iamsvc, ins.ARN().String(), versionID)

								if err != nil {
									log.Error(err, "error while deleting Policy version during reconciliation")
									return ctrl.Result{}, err
								}
							}
						}
					} else {
						// we had an error during AWS Object update... so we return here to retry
						log.Error(err, "error while updating Policy during reconciliation")
						return ctrl.Result{}, err
					}
				}
			}
		}
		log.Error(err, "error while creating Policy during reconciliation")
		return ctrl.Result{}, err
	}

	policy.Status.ObservedGeneration = policy.ObjectMeta.Generation
	if err := r.Status().Update(ctx, &policy); err != nil {
		return ctrl.Result{}, err
	}

	log.Info(fmt.Sprintf("Created Policy '%s'", policy.Status.ARN))

	return ctrl.Result{}, nil
}

func (r *PolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&iamv1beta1.Policy{}).
		Complete(r)
}

// Returns a function, that does everything necessary before we can delete our actual Policy (cleanup)
func policyCleanup(r *PolicyReconciler, ctx context.Context, policy *iamv1beta1.Policy) func() error {
	return func() error {
		attachments := iamv1beta1.PolicyAttachmentList{}
		if err := r.List(ctx, &attachments); err != nil {
			return err
		}
		for _, att := range attachments.Items {
			if att.Spec.PolicyReference.Name == policy.Name && att.Spec.PolicyReference.Namespace == policy.Namespace {
				err := fmt.Errorf("cannot delete policy due to existing PolicyAttachment '%s/%s'", att.Name, att.Namespace)
				return err
			}
		}
		return nil
	}
}
