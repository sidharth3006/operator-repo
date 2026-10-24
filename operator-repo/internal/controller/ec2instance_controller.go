/*
Copyright 2026.

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

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	computev1 "github.com/sidharth3006/operator-repo/api/v1"
)

// EC2InstanceReconciler reconciles a EC2Instance object
type EC2InstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=compute.cloud.com,resources=ec2instances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=compute.cloud.com,resources=ec2instances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=compute.cloud.com,resources=ec2instances/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EC2Instance object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.25.0/pkg/reconcile
func (r *EC2InstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	log := logf.FromContext(ctx)
	log.Info("=== RECONCILE LOOP STARTED ===", "namespace", req.Namespace, "name", req.Name)

	// Create a new instance of the EC2Instance struct to hold the data retrieved from Kubernetes API.
	// This struct will be populated with the current state of the EC2Insstance resource specified by request
	ec2Instance := &computev1.EC2Instance{}

	//Retrieve the EC2Instance resource from the kubernetes api server using the provided request's namespace
	if err := r.Get(ctx, req.NamespacedName, ec2Instance); err != nil {
		if errors.IsNotFound(err) {
			log.Info("EC2Instance not found. Ignoring since object must be deleted")
			// Kubernetes will not retry - done, wait for next event
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get EC2Instance")
		// Kubernetes will retyr with backoff
		return ctrl.Result{}, err
	}

	//check if deletionTimestamp is not zero
	if !ec2Instance.DeletionTimestamp.IsZero() {
		log.Info("Has deletionTimestamp, Instance is being deleted")
		_, err := deleteEc2Instance(ctx, ec2Instance)
		if err != nil {
			log.Error(err, "Failed to delete EC2 instance")
			// Kubernetes will retry with backoff
			return ctrl.Result{Requeue: true}, err
		}

		// Remove the finalizer
		controllerutil.RemoveFinalizer(ec2Instance, "ec2instance.compute.cloud.com")
		if err := r.Update(ctx, ec2Instance); err != nil {
			log.Error(err, "Failed to remove finalizer")
			// Kubernetes will retry with backoff
			return ctrl.Result{Requeue: true}, err
		}
		// at this point, the instance state is terminated and the finalizer is removed
		return ctrl.Result{}, nil
	}

	// Check if we already have an instance ID in status (It means instance already exisits no need to create a new one)
	// But we can check what is the status of ec2 instance or update hte status(not implemented now)
	if ec2Instance.Status.InstanceId != "" {
		log.Info("Instance already exists in Kubernetes. Checking if it's still running", "instanceID", ec2Instance.Status.InstanceId)
		log.Info("Instance already exists, skipping creation")
		return ctrl.Result{}, nil
		/*
			//Instance already exists, verify it's still running
			instanceExist, instanceState, _ := checkEC2InstanceExists(ctx,ec2Instance.Status.InstanceId,ec2Instance.Spec.Region)

			if !instanceExist{
				log.Info("Instance does not exist or is not running","instanceID",ec2Instance.Status.InstanceId)
				ec2Instance.Status.State = "Unknown"
				ec2Instance.Status.PublicIP = ""
				r.Status().Update(ctx,ec2Instance)

				// TODO: Delete the instance from Kubernetes
			}
		*/
	}

	log.Info("Reconciling EC2Instance", "name", ec2Instance.Name, "namespace", ec2Instance.Namespace)
	log.Info("EC2Instance spec", "InstanceType", ec2Instance.Spec.InstanceType)
	log.Info("EC2Instance spec", "ImageID", ec2Instance.Spec.AMIID)
	log.Info("EC2Instance spec", "Subnet", ec2Instance.Spec.Subnet)
	log.Info("EC2Instance spec", "Storage", ec2Instance.Spec.Storage)

	log.Info("Creating new instance")
	log.Info("=== About to add finalizer ===")
	controllerutil.AddFinalizer(ec2Instance, "ec2instance.compute.cloud.com")
	if err := r.Update(ctx, ec2Instance); err != nil {
		log.Error(err, "Failed to add finalizer")
		// Kubernetes will retry with backoff
		return ctrl.Result{Requeue: true}, err
	}
	log.Info("=== Finalizer Added - This update will trigger a NEW reconcile loop, but current reconcile continues ===")

	createdInstanceInfo, err := createEc2Instance(ec2Instance)
	if err != nil {
		log.Error(err, "Failed to create EC2 instance")
		return ctrl.Result{}, err
	}

	log.Info("=== About to update Status - this will trigger reconcile loop again ====",
		"instanceID", createdInstanceInfo.InstanceId,
		"instanceState", createdInstanceInfo.State)

	ec2Instance.Status.InstanceId = createdInstanceInfo.InstanceId
	ec2Instance.Status.State = createdInstanceInfo.State
	ec2Instance.Status.PublicIp = createdInstanceInfo.PublicIp

	//The reconcile function must return a ctrl.Result and an error.
	// Returning ctrl.Result{} with nil error means the reconcialation was successful
	// and no requeue is requested. If an error is returned, the controller will automatically requeue the request for another attempt
	err = r.Status().Update(ctx, ec2Instance)
	if err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	log.Info("===Status updated successfully ===")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EC2InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&computev1.EC2Instance{}).
		Named("ec2instance").
		Complete(r)
}
