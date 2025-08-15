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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
var ErrParseNetLine = errors.New("unable to parse net line")

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
		return ctrl.Result{}, client.IgnoreNotFound(err) //nolint:wrapcheck
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

	port, version, err := findJumperlessPort(ctx, ports)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to find Jumperless port: %w", err)
	}

	if port == nil {
		return ctrl.Result{}, fmt.Errorf("no Jumperless port found: %w", ErrNoSerialPortFound)
	}

	log.Info("Found Jumperless", "port", port, "firmwareVersion", version)

	// Create a new instance to hold the status update to avoid issues with potential SSA diffs
	statusInstance := &jumperlessv5alpha1.Jumperless{}
	statusInstance.SetGroupVersionKind(jumperlessv5alpha1.GroupVersion.WithKind("Jumperless"))
	statusInstance.SetName(instance.Name)
	statusInstance.SetNamespace(instance.Namespace)

	// Deep copy the existing status to the new instance to ensure similar ordering
	// to appease SSA diffing
	instance.Status.DeepCopyInto(&statusInstance.Status)

	statusInstance.Status.LocalPort = ptr.To(port.Name)
	statusInstance.Status.FirmwareVersion = ptr.To(version)

	dacStatus := []jumperlessv5alpha1.DACStatus{}
	for _, channel := range jumperlessv5alpha1.DACChannels {
		dacVoltage, err := getDAC(ctx, port.Name, channel)
		if err != nil {
			log.Error(err, "unable to get DAC voltage", "channel", channel)
			return ctrl.Result{}, fmt.Errorf("unable to get DAC voltage for channel %s: %w", channel, err)
		}

		log.Info("Retrieved DAC voltage", "channel", channel, "voltage", dacVoltage)

		// Initialize DAC status for each channel
		s := jumperlessv5alpha1.DACStatus{
			Channel: channel.String(),
			Voltage: dacVoltage,
		}
		dacStatus = append(dacStatus, s)
	}

	statusInstance.Status.DACS = dacStatus

	nets, err := getNets(ctx, port.Name)
	if err != nil {
		log.Error(err, "unable to get nets")
		return ctrl.Result{}, fmt.Errorf("unable to get nets: %w", err)
	}

	statusInstance.Status.Nets = nets

	config, err := getConfig(ctx, port.Name)
	if err != nil {
		log.Error(err, "unable to get Jumperless config")
		return ctrl.Result{}, fmt.Errorf("unable to get Jumperless config: %w", err)
	}

	statusInstance.Status.UpsertConfig(config)

	// Convert to unstructured for SSA patch
	uResource, err := runtime.DefaultUnstructuredConverter.ToUnstructured(statusInstance)
	if err != nil {
		log.Error(err, "unable to convert Jumperless status to unstructured")
		return ctrl.Result{}, fmt.Errorf("unable to convert Jumperless status to unstructured: %w", err)
	}

	u := &unstructured.Unstructured{}
	u.SetUnstructuredContent(uResource)

	if err := r.Status().Patch(ctx, u, client.Apply, client.ForceOwnership, client.FieldOwner("k8s-jumperless")); err != nil {
		log.Error(err, "unable to patch Jumperless status")
		return ctrl.Result{}, fmt.Errorf("unable to patch Jumperless status: %w", err)
	}

	log.Info("Successfully reconciled Jumperless", "name", instance.Name, "namespace", instance.Namespace)

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

func findJumperlessPort(ctx context.Context, ports []*enumerator.PortDetails) (*enumerator.PortDetails, string, error) {
	errs := []error{}

	for _, port := range ports {
		// Check if this is a Jumperless port
		isJumperless, version, err := isJumperlessPort(ctx, port.Name)
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

func parseNets(netsOutput string) ([]jumperlessv5alpha1.Net, error) {
	errs := []error{}

	nets := slices.Collect(func(yield func(jumperlessv5alpha1.Net) bool) {
		for line := range strings.SplitSeq(netsOutput, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "Index") {

				net, err := parseNetLine(trimmed)
				if err != nil {
					errs = append(errs, fmt.Errorf("unable to parse net line %q: %w", trimmed, err))
					continue
				}

				if !yield(net) {
					return
				}
			}
		}
	})

	return nets, kerrors.NewAggregate(errs)
}

func parseNetLine(netLine string) (jumperlessv5alpha1.Net, error) {
	net := jumperlessv5alpha1.Net{}

	// Example net lines:
	//   "\r1\t GND\t\t 0 V         GND\t    "
	//   "2\t Top Rail\t 0.00 V      TOP_R\t    "
	//   "3\t Bottom Rail\t 0.00 V      BOT_R\t    "
	//   "4\t DAC 0\t\t 3.33 V      DAC_0,BUF_IN\t    "
	//   "5\t DAC 1\t\t 0.00 V      DAC_1\t    "

	// start by splitting fields on tabs to get index, name, and rest
	fields := strings.SplitN(netLine, "\t", 3)

	if len(fields) < 3 {
		return jumperlessv5alpha1.Net{}, fmt.Errorf("expected at least 3 fields, got %d for line %s: %w", len(fields), netLine, ErrParseNetLine)
	}

	// index is the first field
	index, err := strconv.ParseInt(strings.TrimSpace(fields[0]), 10, 32)
	if err != nil {
		return jumperlessv5alpha1.Net{}, fmt.Errorf("unable to parse index (%s) from net line %s: %w", fields[0], netLine, err)
	}

	net.Index = int32(index)

	// name is the second field
	net.Name = strings.TrimSpace(fields[1])

	// rest is the remaining fields
	rest := strings.TrimSpace(fields[2])

	before, after, found := strings.Cut(rest, " V")
	if !found {
		return jumperlessv5alpha1.Net{}, fmt.Errorf("unable to find voltage in net line %s: %w", netLine, ErrParseNetLine)
	}

	net.Voltage = strings.TrimSpace(before) + "V" // ensure voltage is suffixed with "V"

	nodesPart := strings.TrimSpace(after)
	net.Nodes = []string{}

	for node := range strings.SplitSeq(nodesPart, ",") {
		trimmed := strings.TrimSpace(node)
		if trimmed != "" {
			net.Nodes = append(net.Nodes, trimmed)
		}
	}

	return net, nil
}

func parseConfig(configOutput string) ([]jumperlessv5alpha1.JumperLessConfigSection, error) {
	// Example config output:
	// ~
	//
	// copy / edit / paste any of these lines
	// into the main menu to change a setting
	//
	// Jumperless Config:
	//
	//
	// `[config] firmware_version = 5.2.2.0;
	//
	// `[hardware] generation = 5;
	// `[hardware] revision = 5;
	// `[hardware] probe_revision = 5;
	//
	// `[dacs] top_rail = 3.50;
	// `[dacs] bottom_rail = 3.50;
	// ...
	// `[top_oled] font = jokerman;
	//
	// END

	errs := []error{}

	config := map[string]map[string]string{}

	for line := range strings.SplitSeq(configOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue // skip empty lines
		}

		if !strings.HasPrefix(trimmed, "`[") {
			continue // skip non-config lines
		}

		// The section name is between "`[" and "]"
		trimmed = strings.TrimPrefix(trimmed, "`[")

		section, entry, found := strings.Cut(trimmed, "]")
		if !found {
			errs = append(errs, fmt.Errorf("unable to parse config line %q: %w", line, ErrParseNetLine))
			continue
		}

		if _, ok := config[section]; !ok {
			config[section] = map[string]string{}
		}

		// Parse entry line
		key, value, found := strings.Cut(entry, "=")
		if !found {
			errs = append(errs, fmt.Errorf("unable to parse config entry line %q: %w", trimmed, ErrParseNetLine))
			continue
		}

		config[section][strings.TrimSpace(key)] = strings.TrimSuffix(strings.TrimSpace(value), ";")
	}

	jumperlessConfig := []jumperlessv5alpha1.JumperLessConfigSection{}

	for sectionName, entries := range config {
		section := jumperlessv5alpha1.JumperLessConfigSection{
			Name:    sectionName,
			Entries: []jumperlessv5alpha1.JumperlessConfigEntry{},
		}
		for key, value := range entries {
			section.Entries = append(section.Entries, jumperlessv5alpha1.JumperlessConfigEntry{
				Key:   key,
				Value: value,
			})
		}

		jumperlessConfig = append(jumperlessConfig, section)
	}

	return jumperlessConfig, kerrors.NewAggregate(errs)
}

func getConfig(ctx context.Context, portName string) ([]jumperlessv5alpha1.JumperLessConfigSection, error) {
	configOutput, err := execRawCommand(ctx, portName, "~", 500*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("unable to get current config: %w", err)
	}

	return parseConfig(configOutput)
}

func getNets(ctx context.Context, portName string) ([]jumperlessv5alpha1.Net, error) {
	netsOutput, err := execPythonCommand(ctx, portName, "print_nets()", 10*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("unable to print nets: %w", err)
	}

	return parseNets(netsOutput)
}

func getDAC(ctx context.Context, portName string, channel jumperlessv5alpha1.DACChannel) (string, error) {
	dacVoltage, err := execPythonCommand(ctx, portName, fmt.Sprintf("dac_get(%d)", channel), 10*time.Millisecond)
	if err != nil {
		return "", fmt.Errorf("unable to get DAC voltage for channel %s: %w", channel, err)
	}

	result := strings.TrimSpace(dacVoltage) + "V" // Ensure result is suffixed with "V"

	return result, nil
}

func execPythonCommand(ctx context.Context, portName string, command string, waitForRead time.Duration) (string, error) {
	log := ctrl.LoggerFrom(ctx)

	result, err := execRawCommand(ctx, portName, ">"+command, waitForRead)
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %w", err)
	}

	result = ansi.Strip(result) // Remove ANSI escape codes

	// Split the output and strip the first and last lines
	// Example output:
	// Python> >dac_get(0)\r\n3.3V\r\n
	// The first line is the command prompt, the last line is empty.
	// The first line may also contain repeated substrings of the command and prompt
	// since Jumperless is streaming the prompt back using ANSI escape codes.
	resultLines := strings.Split(result, "\r\n")
	if len(resultLines) < 3 {
		log.Info("Unexpected command output format", "port", portName, "command", command, "result", result, "split", resultLines)
		return "", fmt.Errorf("unexpected command output format: expected 3 lines, got %d %w", len(resultLines), ErrUnexpectedCommandOutput)
	}

	filtered := slices.Collect(func(yield func(string) bool) {
		for _, line := range resultLines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "Python>") {
				if !yield(trimmed) {
					return
				}
			}
		}
	})

	log.Info("Python command executed", "port", portName, "command", command, "result", result, "filteredResult", filtered)

	switch len(filtered) {
	case 0:
		return "", fmt.Errorf("unexpected command output format: no output lines after filtering %w", ErrUnexpectedCommandOutput)
	case 1:
		return filtered[0], nil
	default:
		return strings.Join(filtered, "\n"), nil
	}
}

func execRawCommand(ctx context.Context, portName string, command string, waitForRead time.Duration) (string, error) {
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

	time.Sleep(waitForRead)

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

	log.Info("Command executed", "port", portName, "command", command, "rawResult", result)

	return result, nil
}

func isJumperlessPort(ctx context.Context, portName string) (bool, string, error) {
	result, err := execRawCommand(ctx, portName, "?", 10*time.Millisecond)
	if err != nil {
		return false, "", fmt.Errorf("failed to execute command: %w", err)
	}

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
	//nolint:wrapcheck
	return ctrl.NewControllerManagedBy(mgr).
		For(&jumperlessv5alpha1.Jumperless{}).
		Named("jumperless").
		Complete(r)
}
