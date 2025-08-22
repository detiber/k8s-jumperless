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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/detiber/k8s-jumperless/utils/jumperless-emulator/emulator"
	"gopkg.in/yaml.v3"
)

var (
	ErrResponseWithoutRequest      = errors.New("received response without preceding request")
	ErrUnsupportedOutputFormat     = errors.New("unsupported output format (use yaml, json, or log)")
	ErrUnsupportedConfigFileFormat = errors.New("unsupported config file format (use .yaml, .yml, or .json)")
)

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

// saveRecording saves the recorded data to a file
func (p *Proxy) saveRecording() error {
	if p.config.Recording.File == "" {
		p.logger.Println("No recording file configured, using default: jumperless-recording.yaml")
		p.config.Recording.File = "jumperless-recording.yaml"
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(p.config.Recording.File)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	var data []byte
	var err error

	data, err = yaml.Marshal(p.recording)
	if err != nil {
		return fmt.Errorf("failed to marshal recording to YAML: %w", err)
	}

	if err := os.WriteFile(p.config.Recording.File, data, 0600); err != nil {
		return fmt.Errorf("failed to write recording file %s: %w", p.config.Recording.File, err)
	}

	if p.config.Recording.EmulatorConfig != "" {
		// Append to emulator config if specified
		emulatorConfig, err := p.ConvertToEmulatorConfig()
		if err != nil {
			return fmt.Errorf("failed to convert recording to emulator config: %w", err)
		}
		emulatorData, err := yaml.Marshal(emulatorConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal emulator config: %w", err)
		}
		if err := os.WriteFile(p.config.Recording.EmulatorConfig, emulatorData, 0600); err != nil {
			return fmt.Errorf("failed to write emulator config file %s: %w", p.config.Recording.EmulatorConfig, err)
		}
	}

	return nil
}

// ConvertToEmulatorConfig converts a recording to an emulator configuration
func (p *Proxy) ConvertToEmulatorConfig() (*emulator.Config, error) {
	c := emulator.DefaultConfig()
	c.Mappings = []emulator.RequestResponse{}

	// gather pending requests
	pendingRequests := make(map[string]emulator.RequestResponse)

	// TODO: need to verify this updated logic works correctly

	// Process entries to create request/response pairs
	var currentRequest emulator.RequestResponse
	var currentResponse emulator.ResponseOption

	defer func() {
		for key := range pendingRequests {
			c.Mappings = append(c.Mappings, pendingRequests[key])
		}
	}()

	for _, entry := range p.recording.Entries {
		switch entry.Direction {
		case "request":
			// Clean up the request (trim whitespace, normalize)
			request := strings.TrimSpace(entry.Data)
			if request == "" {
				continue
			}

			switch currentRequest.Request {
			case "":
				// If we don't have a current request, create a new one
				// and initialize an empty response
				currentRequest = emulator.RequestResponse{
					Request:   request,
					Responses: []emulator.ResponseOption{},
				}
				currentResponse = emulator.ResponseOption{}
			case request:
				// If we already have this request, it's being repeated, so
				// save the existing response and start a new one
				currentRequest.Responses = append(currentRequest.Responses, currentResponse)
				currentResponse = emulator.ResponseOption{}
			default:
				// If we have a different request, save the currerent response
				// and either retrieve the existing request or start a new request
				currentRequest.Responses = append(currentRequest.Responses, currentResponse)
				currentResponse = emulator.ResponseOption{}
				pendingRequests[currentRequest.Request] = currentRequest

				existingRequest, ok := pendingRequests[request]
				if ok {
					// If we already have this request, reuse it
					currentRequest = existingRequest
				} else {
					// Create a new request
					currentRequest = emulator.RequestResponse{
						Request:   request,
						Responses: []emulator.ResponseOption{},
					}
				}
			}
		case "response":
			if currentRequest.Request == "" {
				return nil, fmt.Errorf("received response without preceding request: %w (data: %s)",
					ErrResponseWithoutRequest, entry.Data)
			}

			currentResponse.Chunks = append(currentResponse.Chunks, emulator.ResponseChunk{
				Data: entry.Data,
				// Set delay based on recorded duration
				Delay:     entry.Duration,
				JitterMax: entry.Duration / 10, // 10% jitter
			})
		}
	}

	// If we have a current request, ensure we save it
	if currentRequest.Request != "" {
		// If we have a current response, ensure we save it
		currentRequest.Responses = append(currentRequest.Responses, currentResponse)

		pendingRequests[currentRequest.Request] = currentRequest
	}

	return c, nil
}

// SaveEmulatorConfig saves the recording as an emulator configuration
func (p *Proxy) SaveEmulatorConfig(filename string) error {
	c, err := p.ConvertToEmulatorConfig()
	if err != nil {
		return fmt.Errorf("failed to convert recording to emulator config: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
