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

import (
	"iter"
	"slices"
	"time"

	"github.com/spf13/viper"
)

const (
	// Default values for the emulator configuration
	DefaultBufferSize = 1024

	// Flag names for command-line arguments
	FlagBufferSize  = "buffer-size"
	FlagVirtualPort = "virtual-port"

	// Viper prefix and keys for configuration
	ViperPrefix      = "emulator"
	ViperBufferSize  = ViperPrefix + "." + FlagBufferSize
	ViperVirtualPort = ViperPrefix + "." + FlagVirtualPort
)

// NewFromViper creates an EmulatorConfig from a viper instance
func NewFromViper(v *viper.Viper) *EmulatorConfig {
	cfg := NewDefaultConfig()

	if v.IsSet(ViperBufferSize) {
		cfg.BufferSize = v.GetInt(ViperBufferSize)
	}
	if v.IsSet(ViperVirtualPort) {
		cfg.VirtualPort = v.GetString(ViperVirtualPort)
	}
	if v.IsSet(ViperPrefix + ".mappings") {
		if err := v.UnmarshalKey(ViperPrefix+".mappings", &cfg.Mappings); err != nil {
			// If unmarshaling fails, return an empty list of mappings
			cfg.Mappings = []RequestResponse{}
		}
	}

	return cfg
}

// NewDefaultConfig returns an EmulatorConfig with default values
func NewDefaultConfig() *EmulatorConfig {
	return &EmulatorConfig{
		BufferSize:  DefaultBufferSize,
		VirtualPort: "",
		Mappings:    []RequestResponse{},
	}
}

// EmulatorConfig represents the emulator configuration
type EmulatorConfig struct {
	BufferSize  int    `json:"bufferSize"  mapstructure:"buffer-size"  yaml:"bufferSize"`
	VirtualPort string `json:"virtualPort" mapstructure:"virtual-port" yaml:"virtualPort"`

	// Request/response mappings
	Mappings Mappings `json:"mappings" mapstructure:"mappings" yaml:"mappings"`
}

type Mappings []RequestResponse

func (m *Mappings) Get(request string) (*RequestResponse, bool) {
	i := slices.IndexFunc(*m, func(r RequestResponse) bool {
		return r.Request == request
	})
	if i >= 0 {
		return &(*m)[i], true
	}

	return nil, false
}

func (m *Mappings) AddResponse(request string, response ...ResponseOption) {
	i := slices.IndexFunc(*m, func(r RequestResponse) bool {
		return r.Request == request
	})

	if i >= 0 {
		(*m)[i].Responses = append((*m)[i].Responses, response...)
	} else {
		*m = append(*m, RequestResponse{
			Request:   request,
			Responses: response,
		})
	}
}

func (m *Mappings) All() iter.Seq2[string, RequestResponse] {
	return func(yield func(string, RequestResponse) bool) {
		for _, mapping := range *m {
			if !yield(mapping.Request, mapping) {
				return
			}
		}
	}
}

// RequestResponse defines a request pattern and its response(s)
type RequestResponse struct {
	// Request
	Request string `json:"request" mapstructure:"request" yaml:"request"`

	// Multiple responses with ordering
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

// ResponseOption represents a single response option
type ResponseOption struct {
	Chunks []ResponseChunk `json:"chunks" mapstructure:"chunks" yaml:"chunks"`
}
