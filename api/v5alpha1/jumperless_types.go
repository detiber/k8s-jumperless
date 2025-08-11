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

package v5alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DACChannel represents the available DAC channels.
//
//go:generate stringer -type=DACChannel
type DACChannel int32

const (
	DAC0 DACChannel = iota
	DAC1
	TOP_RAIL
	BOTTOM_RAIL
)

// DAC represents a single DAC channel configuration.
type DAC struct {
	// Channel is the DAC channel to set.
	// Valid values are "DAC0", "DAC1", "TOP_RAIL", "BOTTOM_RAIL".
	// +kubebuilder:validation:Enum=DAC0;DAC1;TOP_RAIL;BOTTOM_RAIL
	// +required
	Channel string `json:"channel"`

	// Voltage is the desired voltage to set the DAC channel to.
	// The value is a string representing a quantity, e.g. "3.3V", "0.5V", "-1.2V".
	// Valid range is from -8V to +8V.
	// Examples of valid values: "0V", "3.3V", "-1.5V", "7.8V"
	// Examples of invalid values: "10V", "-9V", "3.33V", "abc"
	// +kubebuilder:validation:Pattern=`^(-?([0-7](\.[0-9]{1,2})?|8(\.0{1,2})?))V$`
	// +required
	Voltage string `json:"voltage"`

	// Save indicates whether the voltage setting should be saved to config.
	// If true, the setting will persist across power cycles.
	// If false, the setting will be lost when power is removed.
	// +default=true
	// +optional
	Save *bool `json:"save,omitempty"`
}

// OLEDDisplay represents the configuration for the optional OLED display.
type OLEDDisplay struct {
	// Enabled indicates whether the OLED display should be enabled.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// JumperlessHost represents a host that is connected to the Jumperless device.
type JumperlessHost struct {
	// Hostname is the hostname or IPAddress of the connected host.
	// +required
	Hostname string `json:"hostname"`

	// Username is the username to use when connecting to the host.
	// +optional
	Username *string `json:"username,omitempty"`

	// SSHKeyRef is a reference to a Kubernetes Secret that contains the SSH private key
	// to use when connecting to the host.
	// The Secret must contain a key named "ssh-privatekey" with the private key data.
	// +optional
	SSHKeyRef *corev1.SecretReference `json:"sshKeyRef,omitempty"`
}

// JumperlessSpec defines the desired state of Jumperless
type JumperlessSpec struct {
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// Host defines the host that is connected to the Jumperless device.
	// +required
	Host JumperlessHost `json:"host"`

	// OLED configures the optional OLED display.
	// If omitted, the display will be disabled.
	// +optional
	OLED *OLEDDisplay `json:"oled,omitempty"`

	// DACS is a list of DAC channel configurations to apply.
	// Each entry specifies a channel, the desired voltage, and whether to save the setting.
	// If multiple entries specify the same channel, the last one takes precedence.
	// +listType=map
	// +listMapKey=channel
	// +patchStrategy=merge
	// +patchMergeKey=channel
	// +optional
	DACS []DAC `json:"dacs,omitempty" patchStrategy:"merge" patchMergeKey:"channel"`
}

// JumperlessStatus defines the observed state of Jumperless.
type JumperlessStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// FirmwareVersion is the version of the Jumperless firmware currently running on the device.
	// This field is populated by the controller after successfully connecting to the device.
	// +optional
	FirmwareVersion *string `json:"firmwareVersion,omitempty"`

	// LocalPort is the name of the local serial port that is connected to the Jumperless device.
	// This field is populated by the controller after successfully discovering the device.
	// +optional
	LocalPort *string `json:"localPort,omitempty"`

	// conditions represent the current state of the Jumperless resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional.
	// - "Progressing": the resource is being created or updated.
	// - "Degraded": the resource failed to reach or maintain its desired state.
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Jumperless is the Schema for the jumperlesses API
type Jumperless struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Jumperless
	// +required
	Spec JumperlessSpec `json:"spec"`

	// status defines the observed state of Jumperless
	// +optional
	Status JumperlessStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// JumperlessList contains a list of Jumperless
type JumperlessList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Jumperless `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Jumperless{}, &JumperlessList{})
}
