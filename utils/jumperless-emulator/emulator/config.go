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

package emulator

import (
	"regexp"
	"time"
)

// Config represents the emulator configuration
type Config struct {
	// Serial port configuration
	Serial SerialConfig `yaml:"serial" json:"serial"`

	// Request/response mappings
	Mappings []RequestResponse `yaml:"mappings" json:"mappings"`
}

// SerialConfig defines serial port parameters
type SerialConfig struct {
	// Port name (e.g., "/dev/ttyS0", "/tmp/jumperless")
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

// RequestResponse defines a request pattern and its response
type RequestResponse struct {
	// Request pattern (can be literal string or regex)
	Request string `yaml:"request" json:"request"`

	// Whether request is a regex pattern
	IsRegex bool `yaml:"isRegex" json:"isRegex"`

	// Compiled regex (populated during load)
	CompiledRegex *regexp.Regexp `yaml:"-" json:"-"`

	// Response to send back
	Response string `yaml:"response" json:"response"`

	// Response configuration
	ResponseConfig ResponseConfig `yaml:"responseConfig" json:"responseConfig"`
}

// ResponseConfig defines how responses should be delivered
type ResponseConfig struct {
	// Delay before sending response
	Delay time.Duration `yaml:"delay" json:"delay"`

	// Random jitter to add to delay (0 to JitterMax)
	JitterMax time.Duration `yaml:"jitterMax" json:"jitterMax"`

	// Whether to chunk the response
	Chunked bool `yaml:"chunked" json:"chunked"`

	// Size of each chunk (if chunked)
	ChunkSize int `yaml:"chunkSize" json:"chunkSize"`

	// Delay between chunks
	ChunkDelay time.Duration `yaml:"chunkDelay" json:"chunkDelay"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Serial: SerialConfig{
			Port:       "/tmp/jumperless",
			BaudRate:   115200,
			BufferSize: 1024,
			StopBits:   1,
			Parity:     "none",
		},
		Mappings: []RequestResponse{
			{
				Request:  "?",
				IsRegex:  false,
				Response: "Jumperless firmware version: 5.2.2.0\r\n",
				ResponseConfig: ResponseConfig{
					Delay:     10 * time.Millisecond,
					JitterMax: 5 * time.Millisecond,
				},
			},
			{
				Request:  "~",
				IsRegex:  false,
				Response: "~\r\n\r\ncopy / edit / paste any of these lines\r\ninto the main menu to change a setting\r\n\r\nJumperless Config:\r\n\r\n\r\n`[config] firmware_version = 5.2.2.0;\r\n\r\n`[hardware] generation = 5;\r\n`[hardware] revision = 5;\r\n`[hardware] probe_revision = 5;\r\n\r\n`[dacs] top_rail = 3.50;\r\n`[dacs] bottom_rail = 3.50;\r\n\r\nEND\r\n",
				ResponseConfig: ResponseConfig{
					Delay:     500 * time.Millisecond,
					JitterMax: 100 * time.Millisecond,
				},
			},
			{
				Request:  `>dac_get\((\d+)\)`,
				IsRegex:  true,
				Response: "Python> >dac_get($1)\r\n3.3V\r\n",
				ResponseConfig: ResponseConfig{
					Delay:     10 * time.Millisecond,
					JitterMax: 5 * time.Millisecond,
				},
			},
			{
				Request:  `>print_nets\(\)`,
				IsRegex:  true,
				Response: "Python> >print_nets()\r\nIndex\tName\t\tVoltage\tNode\r\n1\t GND\t\t 0 V         GND\t    \r\n2\t Top Rail\t 3.50 V      TOP_R\t    \r\n3\t Bottom Rail\t 3.50 V      BOT_R\t    \r\n4\t DAC 0\t\t 3.33 V      DAC_0,BUF_IN\t    \r\n5\t DAC 1\t\t 0.00 V      DAC_1\t    \r\n",
				ResponseConfig: ResponseConfig{
					Delay:     10 * time.Millisecond,
					JitterMax: 5 * time.Millisecond,
				},
			},
		},
	}
}
