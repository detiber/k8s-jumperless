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
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	jumperlessv5alpha1 "github.com/detiber/k8s-jumperless/api/v5alpha1"
)

var ErrNotImplemented = errors.New("not yet implemented")
var ErrNoSerialPortFound = errors.New("no serial port found")
var ErrUnexpectedCommandOutput = errors.New("unexpected command output format")

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

func (r *JumperlessReconciler) reconcileLocal(ctx context.Context, instance *jumperlessv5alpha1.Jumperless) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// do local reconciliation
	log.Info("Reconciling Jumperless locally")

	ports, err := enumerateSerialPorts()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to enumerate serial ports: %w", err)
	}

	port, version, err := findJumperlessPort(ports)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to find Jumperless port: %w", err)
	}

	if port == nil {
		return ctrl.Result{}, fmt.Errorf("no Jumperless port found: %w", ErrNoSerialPortFound)
	}

	log.Info("Found Jumperless", "port", port, "firmwareVersion", version)

	instance.Status.LocalPort = ptr.To(port.Name)
	instance.Status.FirmwareVersion = ptr.To(version)

	dacStatus := []jumperlessv5alpha1.DACStatus{}
	for _, channel := range jumperlessv5alpha1.DACChannels {
		dacVoltage, err := execCommand(ctx, port.Name, fmt.Sprintf(">dac_get(%d)", channel))
		if err != nil {
			log.Error(err, "unable to get DAC voltage", "channel", channel)
			return ctrl.Result{}, fmt.Errorf("unable to get DAC voltage for channel %s: %w", channel, err)
		}

		log.Info("Retrieved DAC voltage", "channel", channel, "voltage", dacVoltage)

		// Initialize DAC status for each channel
		s := jumperlessv5alpha1.DACStatus{
			Channel: channel.String(),
			Voltage: strings.TrimSpace(dacVoltage) + "V", // Ensure voltage is suffixed with "V"
		}
		dacStatus = append(dacStatus, s)
	}

	instance.Status.DACS = dacStatus

	if err := r.Status().Update(ctx, instance); err != nil {
		log.Error(err, "unable to update Jumperless status")
		return ctrl.Result{}, fmt.Errorf("unable to update Jumperless status: %w", err)
	}

	log.Info("Successfully updated Jumperless status", "localPort", instance.Status.LocalPort, "firmwareVersion", instance.Status.FirmwareVersion)

	return ctrl.Result{}, nil
}

func enumerateSerialPorts() ([]*enumerator.PortDetails, error) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, fmt.Errorf("unable to list serial ports: %w", err)
	}

	if len(ports) == 0 {
		return nil, ErrNoSerialPortFound
	}

	return ports, nil
}

func findJumperlessPort(ports []*enumerator.PortDetails) (*enumerator.PortDetails, string, error) {
	errs := []error{}

	for _, port := range ports {
		// Check if this is a Jumperless port
		isJumperless, version, err := isJumperlessPort(port.Name)
		if err != nil {
			errs = append(errs, fmt.Errorf("unable to determine if port is Jumperless %w", err))
			continue
		}

		if isJumperless {
			return port, version, nil
		}
	}

	if len(errs) > 0 {
		return nil, "", kerrors.NewAggregate(errs)
	}

	return nil, "", nil
}

func execCommand(ctx context.Context, portName string, command string) (string, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Executing command on Jumperless", "port", portName, "command", command)

	mode := &serial.Mode{
		BaudRate: 115200,
	}

	s, err := serial.Open(portName, mode)
	if err != nil {
		return "", fmt.Errorf("unable to open serial port %s: %w", portName, err)
	}
	defer s.Close() //nolint:errcheck

	// Reset input and output buffers to ensure clean state
	if err := s.ResetInputBuffer(); err != nil {
		return "", fmt.Errorf("unable to reset input buffer: %w", err)
	}

	if err := s.ResetOutputBuffer(); err != nil {
		return "", fmt.Errorf("unable to reset output buffer: %w", err)
	}

	if _, err := s.Write([]byte(command)); err != nil {
		return "", fmt.Errorf("unable to write to serial port %s: %w", portName, err)
	}

	if err := s.Drain(); err != nil {
		return "", fmt.Errorf("failed to drain serial port: %s: %w", portName, err)
	}

	if err := s.SetReadTimeout(time.Second); err != nil {
		return "", fmt.Errorf("unable to set read timeout on serial port %s: %w", portName, err)
	}

	result := ""

	buff := make([]byte, 128)
	for {
		n, err := s.Read(buff)
		if err != nil {
			return "", fmt.Errorf("unable to read from serial port %s: %w", portName, err)
		}

		if n == 0 {
			break // No more data to read
		}

		result += string(buff[:n])
	}

	result = ansi.Strip(result) // Remove ANSI escape codes

	// Split the output and strip the first and last lines
	// Example output:
	// Python> >dac_get(0)\r\n3.3V\r\n
	// The first line is the command prompt, the last line is empty.
	// The first line may also contain repeated substrings of the command and prompt
	// since Jumperless is streaming the prompt back using ANSI escape codes.
	split := strings.Split(result, "\r\n")
	if len(split) != 3 {
		log.Info("Unexpected command output format", "port", portName, "command", command, "result", result, "split", split)
		return "", fmt.Errorf("unexpected command output format: expected 3 lines, got %d %w", len(split), ErrUnexpectedCommandOutput)
	}

	result = split[1] // Strip the first and last lines

	log.Info("Command executed", "port", portName, "command", command, "result", result)

	result = strings.TrimPrefix(result, "Python> "+command+"\n") // Remove command prompt prefix
	return result, nil
}

func isJumperlessPort(portName string) (bool, string, error) {
	mode := &serial.Mode{
		BaudRate: 115200,
	}

	s, err := serial.Open(portName, mode)
	if err != nil {
		return false, "", fmt.Errorf("unable to open serial port %s: %w", portName, err)
	}

	defer s.Close() //nolint:errcheck

	// Reset input and output buffers to ensure clean state
	if err := s.ResetInputBuffer(); err != nil {
		return false, "", fmt.Errorf("unable to reset input buffer: %w", err)
	}

	if err := s.ResetOutputBuffer(); err != nil {
		return false, "", fmt.Errorf("unable to reset output buffer: %w", err)
	}

	if _, err := s.Write([]byte("?")); err != nil {
		return false, "", fmt.Errorf("unable to write to serial port %s: %w", portName, err)
	}

	if err := s.Drain(); err != nil {
		return false, "", fmt.Errorf("failed to drain serial port: %s: %w", portName, err)
	}

	if err := s.SetReadTimeout(time.Second); err != nil {
		return false, "", fmt.Errorf("unable to set read timeout on serial port %s: %w", portName, err)
	}

	buff := make([]byte, 128)
	n, err := s.Read(buff)
	if err != nil {
		return false, "", fmt.Errorf("unable to read from serial port %s: %w", portName, err)
	}

	// If we read no data, assume this is not a Jumperless port
	if n == 0 {
		return false, "", nil
	}

	result := string(buff[:n])

	// Jumperless responds to "?" with a string containing "Jumperless firmware version:"
	expectedPrefix := "Jumperless firmware version:"
	if strings.Contains(result, expectedPrefix) {
		version := strings.TrimSpace(strings.Replace(result, expectedPrefix, "", 1))
		return true, version, nil
	}

	return false, "", nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *JumperlessReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&jumperlessv5alpha1.Jumperless{}).
		Named("jumperless").
		Complete(r)
}
