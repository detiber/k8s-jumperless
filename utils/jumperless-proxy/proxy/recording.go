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
	"context"
	"errors"
	"fmt"
	"log"
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

// Recorder handles recording of serial port interactions
type Recorder struct {
	recChan            chan RecordEntry
	startTime          time.Time
	endTime            time.Time
	entries            []RecordEntry
	stop               chan struct{}
	logger             *log.Logger
	filename           string
	emulatorConfigFile string
}

// NewRecorder creates a new Recorder instance
func NewRecorder(logger *log.Logger, filename string, emulatorConfigFile string) *Recorder {

	return &Recorder{
		recChan:            make(chan RecordEntry),
		entries:            []RecordEntry{},
		stop:               make(chan struct{}),
		logger:             logger,
		filename:           filename,
		emulatorConfigFile: emulatorConfigFile,
	}
}

func (r *Recorder) startListener(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case entry := <-r.recChan:
				r.entries = append(r.entries, entry)
			}
		}
	}()
}

func (r *Recorder) shutdown() {
	r.endTime = time.Now()

	if err := r.saveRecording(); err != nil {
		r.logger.Printf("Error saving recording: %v", err)
	} else {
		r.logger.Printf("Recording saved to: %s", r.filename)
		if r.emulatorConfigFile != "" {
			r.logger.Printf("Emulator config saved to: %s", r.emulatorConfigFile)
		}
	}
}

// Start begins the recording process
func (r *Recorder) Start(ctx context.Context) {
	r.startTime = time.Now()

	listenContext, cancelListener := context.WithCancel(ctx)
	r.startListener(listenContext)

	go func() {
		for {
			select {
			case <-ctx.Done():
				// context cancelled, shutdown
				// listener context is a child of ctx, so it will be cancelled too
				r.shutdown()
				return
			case <-r.stop:
				// start with shutting down the listener
				cancelListener()
				// then shutdown the recorder
				r.shutdown()
			}
		}
	}()
}

// Record records a new entry
func (r *Recorder) Record(ctx context.Context, entry RecordEntry) {
	select {
	case <-ctx.Done():
		return
	case r.recChan <- entry:
		return
	}
}

// Stop stops the recording process
func (r *Recorder) Stop() {
	close(r.stop)
}

// saveRecording saves the recorded data to a file
func (r *Recorder) saveRecording() error {
	r.logger.Println("Saving recording...")

	if r.filename == "" {
		r.logger.Println("No recording file configured, using default: jumperless-recording.yaml")
		r.filename = "jumperless-recording.yaml"
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(r.filename)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	var data []byte
	var err error

	recording := Recording{
		StartTime: r.startTime,
		EndTime:   r.endTime,
		Entries:   r.entries,
	}

	data, err = yaml.Marshal(recording)
	if err != nil {
		return fmt.Errorf("failed to marshal recording to YAML: %w", err)
	}

	if err := os.WriteFile(r.filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write recording file %s: %w", r.filename, err)
	}

	r.logger.Printf("Recording saved to %s", r.filename)

	if r.emulatorConfigFile != "" {
		r.logger.Printf("Saving emulator config to %s", r.emulatorConfigFile)

		// Append to emulator config if specified
		emulatorConfig, err := recording.ConvertToEmulatorConfig()
		if err != nil {
			r.logger.Printf("Error converting to emulator config: %v", err)
			return fmt.Errorf("failed to convert recording to emulator config: %w", err)
		}
		emulatorData, err := yaml.Marshal(emulatorConfig)
		if err != nil {
			r.logger.Printf("Error marshaling emulator config: %v", err)
			return fmt.Errorf("failed to marshal emulator config: %w", err)
		}
		if err := os.WriteFile(r.emulatorConfigFile, emulatorData, 0600); err != nil {
			r.logger.Printf("Error writing emulator config file: %v", err)
			return fmt.Errorf("failed to write emulator config file %s: %w", r.emulatorConfigFile, err)
		}

		r.logger.Printf("Emulator config saved to %s", r.emulatorConfigFile)
	}

	return nil
}

// ConvertToEmulatorConfig converts a recording to an emulator configuration
func (r *Recording) ConvertToEmulatorConfig() (*emulator.Config, error) {
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

	for _, entry := range r.Entries {
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
func (r *Recording) SaveEmulatorConfig(filename string) error {
	c, err := r.ConvertToEmulatorConfig()
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
