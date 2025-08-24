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

import "time"

// const DefaultBaudRate = 115200
const DefaultBufferSize = 1024

// EmulatorConfig represents the emulator configuration
type EmulatorConfig struct {
	BufferSize  int    `json:"bufferSize"  mapstructure:"buffer-size"  yaml:"bufferSize"`
	VirtualPort string `json:"virtualPort" mapstructure:"virtual-port" yaml:"virtualPort"`

	// Request/response mappings
	Mappings []RequestResponse `json:"mappings" mapstructure:"mappings" yaml:"mappings"`
}

// RequestResponse defines a request pattern and its response(s)
type RequestResponse struct {
	// Request pattern (can be literal string or regex)
	Request string `json:"request" mapstructure:"request" yaml:"request"`

	// Multiple responses with ordering/randomization
	Responses []ResponseOption `json:"responses" mapstructure:"responses" yaml:"responses"`
}

type ResponseChunk struct {
	// Chunk data
	Data string `json:"data" mapstructure:"data" yaml:"data"`

	// Delay before sending response
	Delay time.Duration `json:"delay" mapstructure:"delay" yaml:"delay"`

	// Random jitter to add to delay (0 to JitterMax)
	JitterMax time.Duration `json:"jitterMax" mapstructure:"jitter-max" yaml:"jitterMax"`
}

// ResponseOption represents a single response option with optional weight
type ResponseOption struct {
	Chunks []ResponseChunk `json:"chunks" mapstructure:"chunks" yaml:"chunks"`
}
