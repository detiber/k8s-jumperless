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

package config

const DefaultBaudRate = 115200
const DefaultBufferSize = 1024

// ProxyConfig represents the proxy configuration
type ProxyConfig struct {
	// Baud rate
	BaudRate int `json:"baudRate" mapstructure:"baud-rate" yaml:"baudRate"`

	// Buffer size for reading/writing
	BufferSize int `json:"bufferSize" mapstructure:"buffer-size" yaml:"bufferSize"`

	// Virtual port name (e.g., "/dev/ttyUSB0")
	VirtualPort string `json:"virtualPort" mapstructure:"virtual-port" yaml:"virtualPort"`

	// Real port name (e.g., "/tmp/jumperless-proxy")
	RealPort string `json:"realPort" mapstructure:"real-port" yaml:"realPort"`

	// Recording configuration
	Recording RecordingConfig `json:"recording" mapstructure:"recording" yaml:"recording"`
}

// RecordingConfig defines recording parameters
type RecordingConfig struct {
	// File for recorded data
	File string `json:"file" mapstructure:"file" yaml:"file"`

	// Emulator config file to append to
	EmulatorConfig string `json:"emulatorConfig" mapstructure:"emulator-config" yaml:"emulatorConfig"`

	// Whether to include timestamps in recording
	IncludeTimestamps bool `json:"includeTimestamps" mapstructure:"include-timestamps" yaml:"includeTimestamps"`

	// Buffer size for recording (0 = unbuffered)
	BufferSize int `json:"bufferSize" mapstructure:"buffer-size" yaml:"bufferSize"`
}

// DefaultConfig returns a default proxy configuration
func DefaultConfig() *ProxyConfig {
	return &ProxyConfig{
		BaudRate:    DefaultBaudRate,
		BufferSize:  DefaultBufferSize,
		VirtualPort: "",
		RealPort:    "",
		Recording: RecordingConfig{
			EmulatorConfig:    "",
			File:              "",
			IncludeTimestamps: true,
			BufferSize:        0, // Unbuffered for real-time recording
		},
	}
}
