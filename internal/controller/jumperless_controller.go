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

	"go.bug.st/serial/enumerator"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	jumperlessv5alpha1 "github.com/detiber/k8s-jumperless/api/v5alpha1"
)

var ErrNotImplemented = errors.New("not yet implemented")
var ErrNoSerialPortFound = errors.New("no serial port found")

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
func (r *JumperlessReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Reconciling Jumperless", "request", req.NamespacedName)

	// Fetch the Jumperless instance
	instance := &jumperlessv5alpha1.Jumperless{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		log.Error(err, "unable to fetch Jumperless")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Fetched Jumperless instance", "instance", instance)

	// Determine if we are running on localhost or a remote host
	// and perform the appropriate reconciliation.
	// If no hostname is specified, default to localhost.
	switch hostname := instance.Spec.Host.Hostname; hostname {
	case "":
		log.Info("No hostname specified, defaulting to localhost")
		fallthrough
	case "localhost", "127.0.0.1", "::1":
		return r.reconcileLocal(ctx, instance)
	default:
		// do remote reconciliation
		log.Info("Reconciling Jumperless remotely", "hostname", hostname)
		return ctrl.Result{}, fmt.Errorf("remote reconciliation not implemented: %w", ErrNotImplemented)
	}
}

func (r *JumperlessReconciler) reconcileLocal(ctx context.Context, _ *jumperlessv5alpha1.Jumperless) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// do local reconciliation
	log.Info("Reconciling Jumperless locally")

	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		log.Error(err, "unable to list serial ports")
		return ctrl.Result{}, fmt.Errorf("unable to list serial ports: %w", err)
	}
	if len(ports) == 0 {
		log.Error(ErrNoSerialPortFound, "no serial ports found")
		return ctrl.Result{}, fmt.Errorf("no serial ports found: %w", ErrNoSerialPortFound)
	}
	for _, port := range ports {
		log.Info("Found serial port", "port", port)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *JumperlessReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&jumperlessv5alpha1.Jumperless{}).
		Named("jumperless").
		Complete(r)
}
