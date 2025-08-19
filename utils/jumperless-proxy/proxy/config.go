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
	VirtualPort SerialPortConfig `json:"virtualPort" yaml:"virtualPort"`

	// Real serial port configuration (device side)
	RealPort SerialPortConfig `json:"realPort" yaml:"realPort"`

	// Recording configuration
	Recording RecordingConfig `json:"recording" yaml:"recording"`
}

// SerialPortConfig defines serial port parameters
type SerialPortConfig struct {
	// Port name (e.g., "/dev/ttyUSB0", "/tmp/jumperless-proxy")
	Port string `json:"port" yaml:"port"`

	// Baud rate
	BaudRate int `json:"baudRate" yaml:"baudRate"`

	// Buffer size for reading/writing
	BufferSize int `json:"bufferSize" yaml:"bufferSize"`

	// Stop bits (1 or 2)
	StopBits int `json:"stopBits" yaml:"stopBits"`

	// Parity (none, odd, even, mark, space)
	Parity string `json:"parity" yaml:"parity"`
}

// RecordingConfig defines recording parameters
type RecordingConfig struct {
	// Enable recording
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Output file for recorded data
	OutputFile string `json:"outputFile" yaml:"outputFile"`

	// Format for output file (yaml, json, log)
	OutputFormat string `json:"outputFormat" yaml:"outputFormat"`

	// Whether to include timestamps in recording
	IncludeTimestamps bool `json:"includeTimestamps" yaml:"includeTimestamps"`

	// Buffer size for recording (0 = unbuffered)
	BufferSize int `json:"bufferSize" yaml:"bufferSize"`
}

// RecordEntry represents a single recorded interaction
type RecordEntry struct {
	Timestamp time.Time     `json:"timestamp"          yaml:"timestamp"`
	Direction string        `json:"direction"          yaml:"direction"` // "request" or "response"
	Data      string        `json:"data"               yaml:"data"`
	Duration  time.Duration `json:"duration,omitempty" yaml:"duration,omitempty"` // Response time
}

// Recording represents a collection of recorded interactions
type Recording struct {
	StartTime time.Time     `json:"startTime" yaml:"startTime"`
	EndTime   time.Time     `json:"endTime"   yaml:"endTime"`
	Entries   []RecordEntry `json:"entries"   yaml:"entries"`
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
