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

	awsarn "github.com/aws/aws-sdk-go/aws/arn"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iamv1beta1 "github.com/redradrat/aws-iam-operator/api/v1beta1"
	"github.com/redradrat/cloud-objects/aws/iam"
)

// PolicyAssignmentReconciler reconciles a PolicyAssignment object
type PolicyAttachmentReconciler struct {
	client.Client
	Region string
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=policyattachments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aws-iam.redradrat.xyz,resources=policyattachments/status,verbs=get;update;patch

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

	// first let's get the ARNs from the referenced resources in the spec
	policyArn, targetArn, err := getPolicyAttachmentARNs(&policyattachment, ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, errWithStatus(&policyattachment, err, r.Status(), ctx)
	}

	// now we need to translate the specified target resource in the CR to an IAM AttachmentType
	attachType, err := policyattachment.GetAttachmentType()
	if err != nil {
		return ctrl.Result{}, errWithStatus(&policyattachment, err, r.Status(), ctx)
	}

	// Get our actual IAM Service to communicate with AWS; we don't need to continue without it
	iamsvc, err := IAMService(r.Region)
	if err != nil {
		return ctrl.Result{}, errWithStatus(&policyattachment, err, r.Status(), ctx)
	}

	// now let's instantiate our PolicyAttachmentInstance
	ins := iam.NewPolicyAttachmentInstance(policyArn, attachType, targetArn)

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

			// delete the actual AWS Object and pass the cleanup function
			statusUpdater, err := DeleteAWSObject(iamsvc, ins, DoNothingPreFunc)
			// we got a StatusUpdater function returned... let's execute it
			statusUpdater(ins, &policyattachment, ctx, r.Status(), log)
			if err != nil {
				// we had an error during AWS Object deletion... so we return here to retry
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

	// if there is already an ARN in our status, then we remove the PolicyAttachment from that ARN:
	// 	1) 	A user could have changed the TargetReference,
	//		so we need to remove it from the old status ARN
	//
	// 	2) 	AWS doesn't support updates of attachments,
	//		so even if the target is the same as the ARN, we need to recreate
	if policyattachment.Status.ARN != "" {
		// delete the actual AWS Object and pass the cleanup function
		statusUpdater, err := DeleteAWSObject(iamsvc, ins, DoNothingPreFunc)
		// we got a StatusUpdater function returned... let's execute it
		statusUpdater(ins, &policyattachment, ctx, r.Status(), log)
		if err != nil {
			// we had an error during AWS Object deletion... so we return here to retry
			log.Error(err, "error while deleting PolicyAttachment during reconciliation")
			return ctrl.Result{}, err
		}
	}
	statusUpdater, err := CreateAWSObject(iamsvc, ins, DoNothingPreFunc)
	statusUpdater(ins, &policyattachment, ctx, r.Status(), log)
	if err != nil {
		log.Error(err, "error while creating PolicyAttachment during reconciliation")
		return ctrl.Result{}, errWithStatus(&policyattachment, err, r.Status(), ctx)
	}

	log.Info(fmt.Sprintf("Created PolicyAttachment on target '%s'", policyattachment.Status.ARN))

	return ctrl.Result{}, nil
}

func checkPolicyAttachmentRefs(policyAttachment *iamv1beta1.PolicyAttachment, c client.Client, ctx context.Context) error {
	policies := iamv1beta1.PolicyList{}
	if err := c.List(ctx, &policies); err != nil {
		return err
	}

	foundtarget := false
	foundpolicy := false
	targetReferenceType := policyAttachment.Spec.TargetReference.Type
	switch targetReferenceType {
	case iamv1beta1.RoleTargetType:
		roles := iamv1beta1.RoleList{}
		if err := c.List(ctx, &roles); err != nil {
			return err
		}
		for _, role := range roles.Items {
			tarref := policyAttachment.Spec.TargetReference
			if tarref.Name == role.Name && tarref.Namespace == role.Namespace {
				foundtarget = true
			}
		}
	case iamv1beta1.UserTargetType:
		users := iamv1beta1.UserList{}
		if err := c.List(ctx, &users); err != nil {
			return err
		}
		for _, user := range users.Items {
			tarref := policyAttachment.Spec.TargetReference
			if tarref.Name == user.Name && tarref.Namespace == user.Namespace {
				foundtarget = true
			}
		}
	case iamv1beta1.GroupTargetType:
		groups := iamv1beta1.GroupList{}
		if err := c.List(ctx, &groups); err != nil {
			return err
		}
		for _, group := range groups.Items {
			tarref := policyAttachment.Spec.TargetReference
			if tarref.Name == group.Name && tarref.Namespace == group.Namespace {
				foundtarget = true
			}
		}
	default:
		return fmt.Errorf("defined target reference type '%s' is unknown", targetReferenceType)
	}
	for _, policy := range policies.Items {
		polref := policyAttachment.Spec.PolicyReference
		if polref.Name == policy.Name && polref.Namespace == policy.Namespace {
			foundpolicy = true
		}
	}
	if !(foundtarget == true && foundpolicy == true) {
		err := fmt.Errorf(fmt.Sprintf("defined references do not exist for PolicyAttachment '%s/%s", policyAttachment.Name, policyAttachment.Namespace))
		return err
	}

	return nil
}

func (r *PolicyAttachmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&iamv1beta1.PolicyAttachment{}).
		Complete(r)
}

func getPolicyAttachmentARNs(policyAttachment *iamv1beta1.PolicyAttachment, ctx context.Context, c client.Client) (awsarn.ARN, awsarn.ARN, error) {
	var policyArn awsarn.ARN
	var targetArn awsarn.ARN

	if err := checkPolicyAttachmentRefs(policyAttachment, c, ctx); err != nil {
		return policyArn, targetArn, err
	}

	polref := policyAttachment.Spec.PolicyReference
	tarref := policyAttachment.Spec.TargetReference
	policy := iamv1beta1.Policy{}
	if err := c.Get(ctx, client.ObjectKey{Name: polref.Name, Namespace: polref.Namespace}, &policy); err != nil {
		return policyArn, targetArn, err
	}

	if policy.Status.ARN == "" {
		return policyArn, targetArn, fmt.Errorf("ARN is empty in status for policy reference")
	}
	policyArn, err := awsarn.Parse(policy.Status.ARN)
	if err != nil {
		return policyArn, targetArn, err
	}

	targetReferenceType := policyAttachment.Spec.TargetReference.Type
	switch targetReferenceType {
	case iamv1beta1.RoleTargetType:
		role := iamv1beta1.Role{}
		if err := c.Get(ctx, client.ObjectKey{Name: tarref.Name, Namespace: tarref.Namespace}, &role); err != nil {
			return policyArn, targetArn, err
		}
		if role.Status.ARN == "" {
			return policyArn, targetArn, fmt.Errorf("ARN is empty in status for target reference")
		}
		targetArn, err = awsarn.Parse(role.Status.ARN)
		if err != nil {
			return policyArn, targetArn, err
		}
	case iamv1beta1.UserTargetType:
		user := iamv1beta1.User{}
		if err := c.Get(ctx, client.ObjectKey{Name: tarref.Name, Namespace: tarref.Namespace}, &user); err != nil {
			return policyArn, targetArn, err
		}
		if user.Status.ARN == "" {
			return policyArn, targetArn, fmt.Errorf("ARN is empty in status for target reference")
		}
		targetArn, err = awsarn.Parse(user.Status.ARN)
		if err != nil {
			return policyArn, targetArn, err
		}
	case iamv1beta1.GroupTargetType:
		group := iamv1beta1.Group{}
		if err := c.Get(ctx, client.ObjectKey{Name: tarref.Name, Namespace: tarref.Namespace}, &group); err != nil {
			return policyArn, targetArn, err
		}
		if group.Status.ARN == "" {
			return policyArn, targetArn, fmt.Errorf("ARN is empty in status for target reference")
		}
		targetArn, err = awsarn.Parse(group.Status.ARN)
		if err != nil {
			return policyArn, targetArn, err
		}
	default:
		return policyArn, targetArn, fmt.Errorf("defined target reference type '%s' is unknown", targetReferenceType)
	}

	return policyArn, targetArn, nil
}
