/*
Copyright 2025.

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
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	jumperlessv5alpha1 "github.com/detiber/k8s-jumperless/api/v5alpha1"
	"github.com/detiber/k8s-jumperless/internal/controller/local"
)

var ErrNotImplemented = errors.New("not yet implemented")

// JumperlessReconciler reconciles a Jumperless object
type JumperlessReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=jumperless.detiber.us,resources=jumperlesses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=jumperless.detiber.us,resources=jumperlesses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=jumperless.detiber.us,resources=jumperlesses/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *JumperlessReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, retErr error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Reconciling Jumperless", "request", req.NamespacedName)

	// Fetch the Jumperless instance
	instance := &jumperlessv5alpha1.Jumperless{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		log.Error(err, "unable to fetch Jumperless")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err) //nolint:wrapcheck
	}

	// Create a copy of the existing status to use for updates, this enables preserving
	// the order of lists to improve the patching behavior to avoid unnecessary updates
	// to the resource version when the status is actually semantically equivalent, but
	// differences in ordering is causing issues with comparison.
	status := instance.Status.DeepCopy()

	// Always update the status
	defer func() {
		if err := r.patchStatus(ctx, instance, status); err != nil {
			log.Error(err, "unable to patch Jumperless status")
			retErr = kerrors.NewAggregate([]error{retErr, err})
			return
		}

		log.Info("Successfully patched Jumperless status", "name", instance.Name, "namespace", instance.Namespace)
	}()

	// Initialize conditions if not already present
	if len(instance.Status.Conditions) == 0 ||
		meta.FindStatusCondition(instance.Status.Conditions, jumperlessv5alpha1.ConditionReady) == nil {
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               jumperlessv5alpha1.ConditionReady,
			Status:             metav1.ConditionUnknown,
			Reason:             "Reconciling",
			Message:            "Jumperless is being reconciled",
			ObservedGeneration: instance.Generation,
		})

		// Return to avoid further processing until next reconciliation
		// status will be updated in the deferred patch
		return ctrl.Result{}, nil
	}

	// Determine if we are running on localhost or a remote host
	// and perform the appropriate reconciliation.
	// If no hostname is specified, default to localhost.
	switch hostname := instance.Spec.Host.Hostname; hostname {
	case "":
		log.Info("No hostname specified, defaulting to localhost")
		fallthrough
	case "localhost", "127.0.0.1", "::1":
		if err := r.reconcileLocal(ctx, instance, status); err != nil {
			log.Error(err, "unable to reconcile Jumperless locally")
			return ctrl.Result{}, fmt.Errorf("unable to reconcile Jumperless locally: %w", err)
		}
	default:
		if err := r.reconcileRemote(ctx, instance, status); err != nil {
			log.Error(err, "unable to reconcile Jumperless remotely", "hostname", hostname)
			return ctrl.Result{}, fmt.Errorf("unable to reconcile Jumperless remotely: %w", err)
		}
	}

	log.Info("Successfully reconciled Jumperless", "name", instance.Name, "namespace", instance.Namespace)
	return ctrl.Result{}, nil
}

func (r *JumperlessReconciler) patchStatus(ctx context.Context, instance *jumperlessv5alpha1.Jumperless, status *jumperlessv5alpha1.JumperlessStatus) error {
	log := ctrl.LoggerFrom(ctx)

	// Create a new instance to hold the status update to avoid issues with potential SSA diffs
	statusInstance := &jumperlessv5alpha1.Jumperless{}
	statusInstance.SetGroupVersionKind(jumperlessv5alpha1.GroupVersion.WithKind("Jumperless"))
	statusInstance.SetName(instance.Name)
	statusInstance.SetNamespace(instance.Namespace)

	// Deep copy the existing status to the new instance to ensure similar ordering
	// to appease SSA diffing
	status.DeepCopyInto(&statusInstance.Status)

	// Convert to unstructured for SSA patch
	uResource, err := runtime.DefaultUnstructuredConverter.ToUnstructured(statusInstance)
	if err != nil {
		log.Error(err, "unable to convert Jumperless status to unstructured")
		return fmt.Errorf("unable to convert Jumperless status to unstructured: %w", err)
	}

	u := &unstructured.Unstructured{}
	u.SetUnstructuredContent(uResource)

	if err := r.Status().Patch(ctx, u, client.Apply, client.ForceOwnership, client.FieldOwner("k8s-jumperless")); err != nil {
		log.Error(err, "unable to patch Jumperless status")
		return fmt.Errorf("unable to patch Jumperless status: %w", err)
	}

	return nil
}

func (r *JumperlessReconciler) reconcileRemote(_ context.Context, instance *jumperlessv5alpha1.Jumperless, status *jumperlessv5alpha1.JumperlessStatus) error {
	// do remote reconciliation

	// set ready condition to false with not implemented reason
	// status will be updated in the deferred patch in Reconcile
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               jumperlessv5alpha1.ConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             "NotImplemented",
		Message:            "Remote reconciliation is not yet implemented",
		ObservedGeneration: instance.Generation,
	})

	return fmt.Errorf("remote reconciliation not implemented: %w", ErrNotImplemented)
}

func (r *JumperlessReconciler) reconcileLocal(ctx context.Context, instance *jumperlessv5alpha1.Jumperless, status *jumperlessv5alpha1.JumperlessStatus) error {
	log := ctrl.LoggerFrom(ctx)

	// do local reconciliation
	log.Info("Reconciling Jumperless locally")

	// Unless there is an existing ready condition that is true for the current generation,
	// ensure the ready condition is set to false with reconciling reason
	currentStatusCondition := meta.FindStatusCondition(status.Conditions, jumperlessv5alpha1.ConditionReady)
	if currentStatusCondition == nil ||
		currentStatusCondition.Status == metav1.ConditionTrue && currentStatusCondition.ObservedGeneration != instance.Generation {
		changed := meta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               jumperlessv5alpha1.ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             "Reconciling",
			Message:            "Jumperless is being reconciled locally",
			ObservedGeneration: instance.Generation,
		})

		// Return to avoid further processing until next reconciliation
		// status will be updated in the deferred patch
		if changed {
			log.Info("Jumperless is not ready, reconciling")
			return nil
		}
	}

	ports, err := local.EnumerateSerialPorts()
	if err != nil {
		// set ready condition to false with no serial port found reason
		// status will be updated in the deferred patch in Reconcile
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               jumperlessv5alpha1.ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             "NoSerialPortFound",
			Message:            fmt.Sprintf("Unable to find Jumperless serial port: %v", err),
			ObservedGeneration: instance.Generation,
		})

		return fmt.Errorf("unable to enumerate serial ports: %w", err)
	}

	port, version, err := local.FindJumperlessPort(ctx, ports)
	if err != nil {
		// set ready condition to false with no jumperless found reason
		// status will be updated in the deferred patch in Reconcile
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               jumperlessv5alpha1.ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             "NoJumperlessFound",
			Message:            fmt.Sprintf("Unable to find Jumperless serial port: %v", err),
			ObservedGeneration: instance.Generation,
		})

		return fmt.Errorf("unable to find Jumperless port: %w", err)
	}

	if port == nil {
		// set ready condition to false with no jumperless port found reason
		// status will be updated in the deferred patch in Reconcile
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               jumperlessv5alpha1.ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             "NoJumperlessPortFound",
			Message:            "No Jumperless serial port found",
			ObservedGeneration: instance.Generation,
		})

		return fmt.Errorf("no Jumperless port found: %w", local.ErrNoSerialPortFound)
	}

	log.Info("Found Jumperless", "port", port, "firmwareVersion", version)

	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               jumperlessv5alpha1.ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "Jumperless successfully reconciled locally",
		ObservedGeneration: instance.Generation,
	})

	status.LocalPort = ptr.To(port.Name)
	status.FirmwareVersion = ptr.To(version)

	dacStatus := []jumperlessv5alpha1.DACStatus{}
	for _, channel := range jumperlessv5alpha1.DACChannels {
		dacVoltage, err := local.GetDAC(ctx, port.Name, channel)
		if err != nil {
			log.Error(err, "unable to get DAC voltage", "channel", channel)
			return fmt.Errorf("unable to get DAC voltage for channel %s: %w", channel, err)
		}

		log.Info("Retrieved DAC voltage", "channel", channel, "voltage", dacVoltage)

		// Initialize DAC status for each channel
		s := jumperlessv5alpha1.DACStatus{
			Channel: channel.String(),
			Voltage: dacVoltage,
		}
		dacStatus = append(dacStatus, s)
	}

	status.DACS = dacStatus

	nets, err := local.GetNets(ctx, port.Name)
	if err != nil {
		log.Error(err, "unable to get nets")
		return fmt.Errorf("unable to get nets: %w", err)
	}

	status.Nets = nets

	config, err := local.GetConfig(ctx, port.Name)
	if err != nil {
		log.Error(err, "unable to get Jumperless config")
		return fmt.Errorf("unable to get Jumperless config: %w", err)
	}

	status.UpsertConfig(config)

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *JumperlessReconciler) SetupWithManager(mgr ctrl.Manager) error {
	//nolint:wrapcheck
	return ctrl.NewControllerManagedBy(mgr).
		For(&jumperlessv5alpha1.Jumperless{}).
		Named("jumperless").
		Complete(r)
}
