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
	"errors"
	"fmt"
	"time"

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

const (
	errRequeueInterval = 10 * time.Second
	policiesFinalizer  = "policy.aws-iam.redradrat.xyz" // the finalizer for deleting the actual aws resources

)

// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=policies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=policies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=policies/finalizers,verbs=get;update

func (r *PolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("policy", req.NamespacedName)

	var policy iamv1beta1.Policy
	// Retrieves policy from the Kubernetes Cluster
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

	// now let's instantiate our PolicyInstance
	var ins *iam.PolicyInstance
	policyName := r.ResourcePrefix + policy.PolicyName()
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

	// We try to create the resource
	statusWriter, err := CreateAWSObject(iamsvc, ins, DoNothingPreFunc)
	statusWriter(ctx, ins, &policy, r.Status(), log)
	if err != nil {
		var awserr awserr.Error
		ok := errors.As(err, &awserr)
		if ok && awserr.Code() == awsiam.ErrCodeEntityAlreadyExistsException {
			err = r.updatePolicy(ctx, iamsvc, ins, policy)
			if err != nil {
				return ctrl.Result{RequeueAfter: errRequeueInterval}, err
			}
		} else {
			// we had an error during AWS Object create... so we return here to retry
			log.Error(err, "error while creating Policy during reconciliation")
			return ctrl.Result{RequeueAfter: errRequeueInterval}, err
		}
	} else {
		log.Info(fmt.Sprintf("Created Policy '%s'", policy.Status.ARN))
	}

	return ctrl.Result{}, nil
}

func (r *PolicyReconciler) updatePolicy(ctx context.Context, iamsvc *awsiam.IAM, ins *iam.PolicyInstance, policy iamv1beta1.Policy) error {
	// If EntityAlreadyExists, we just clean up the policy versions and update the resource
	statusWriter, err := CleanUpPolicyVersions(iamsvc, ins.ARN().String())
	statusWriter(ctx, ins, &policy, r.Status(), r.Log)
	if err != nil {
		r.Log.Error(err, "error while cleaning up Policy versions during reconciliation")
		return err
	}

	statusWriter, err = UpdateAWSObject(iamsvc, ins, DoNothingPreFunc)
	statusWriter(ctx, ins, &policy, r.Status(), r.Log)
	if err != nil {
		r.Log.Error(err, "error while updating Policy during reconciliation")
		return err
	}
	r.Log.Info(fmt.Sprintf("Updated Policy '%s'", policy.Status.ARN))
	return nil
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
