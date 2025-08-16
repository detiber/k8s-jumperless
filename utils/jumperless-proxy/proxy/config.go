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

package proxy

import (
	"time"
)

// Config represents the proxy configuration
type Config struct {
	// Virtual serial port configuration (client side)
	VirtualPort SerialPortConfig `yaml:"virtualPort" json:"virtualPort"`

	// Real serial port configuration (device side)
	RealPort SerialPortConfig `yaml:"realPort" json:"realPort"`

	// Recording configuration
	Recording RecordingConfig `yaml:"recording" json:"recording"`
}

// SerialPortConfig defines serial port parameters
type SerialPortConfig struct {
	// Port name (e.g., "/dev/ttyUSB0", "/tmp/jumperless-proxy")
	Port string `yaml:"port" json:"port"`

	// Baud rate
	BaudRate int `yaml:"baudRate" json:"baudRate"`

	// Buffer size for reading/writing
	BufferSize int `yaml:"bufferSize" json:"bufferSize"`

	// Stop bits (1 or 2)
	StopBits int `yaml:"stopBits" json:"stopBits"`

	// Parity (none, odd, even, mark, space)
	Parity string `yaml:"parity" json:"parity"`
}

// RecordingConfig defines recording parameters
type RecordingConfig struct {
	// Enable recording
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Output file for recorded data
	OutputFile string `yaml:"outputFile" json:"outputFile"`

	// Format for output file (yaml, json, log)
	OutputFormat string `yaml:"outputFormat" json:"outputFormat"`

	// Whether to include timestamps in recording
	IncludeTimestamps bool `yaml:"includeTimestamps" json:"includeTimestamps"`

	// Buffer size for recording (0 = unbuffered)
	BufferSize int `yaml:"bufferSize" json:"bufferSize"`
}

// RecordEntry represents a single recorded interaction
type RecordEntry struct {
	Timestamp time.Time     `yaml:"timestamp" json:"timestamp"`
	Direction string        `yaml:"direction" json:"direction"` // "request" or "response"
	Data      string        `yaml:"data" json:"data"`
	Duration  time.Duration `yaml:"duration,omitempty" json:"duration,omitempty"` // Response time
}

// Recording represents a collection of recorded interactions
type Recording struct {
	StartTime time.Time     `yaml:"startTime" json:"startTime"`
	EndTime   time.Time     `yaml:"endTime" json:"endTime"`
	Entries   []RecordEntry `yaml:"entries" json:"entries"`
}

// DefaultConfig returns a default proxy configuration
func DefaultConfig() *Config {
	return &Config{
		VirtualPort: SerialPortConfig{
			Port:       "/tmp/jumperless-proxy",
			BaudRate:   115200,
			BufferSize: 1024,
			StopBits:   1,
			Parity:     "none",
		},
		RealPort: SerialPortConfig{
			Port:       "/dev/ttyUSB0",
			BaudRate:   115200,
			BufferSize: 1024,
			StopBits:   1,
			Parity:     "none",
		},
		Recording: RecordingConfig{
			Enabled:           true,
			OutputFile:        "jumperless-recording.yaml",
			OutputFormat:      "yaml",
			IncludeTimestamps: true,
			BufferSize:        0, // Unbuffered for real-time recording
		},
	}
}
