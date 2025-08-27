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
	"strconv"
	"time"

	emulatorConfig "github.com/detiber/k8s-jumperless/utils/emulator/emulator/config"
	"gopkg.in/yaml.v3"
)

var (
	ErrResponseWithoutRequest      = errors.New("received response without preceding request")
	ErrUnsupportedOutputFormat     = errors.New("unsupported output format (use yaml, json, or log)")
	ErrUnsupportedConfigFileFormat = errors.New("unsupported config file format (use .yaml, .yml, or .json)")
)

// Recorder handles recording of serial port interactions
type Recorder struct {
	logger   *log.Logger
	filename string
	requests map[string]emulatorConfig.RequestResponse
	reqChan  chan []byte
	resChan  chan []byte
}

// NewRecorder creates a new Recorder instance
func NewRecorder(logger *log.Logger, filename string) *Recorder {
	return &Recorder{
		logger:   logger,
		filename: filename,
		requests: make(map[string]emulatorConfig.RequestResponse),
		reqChan:  make(chan []byte),
		resChan:  make(chan []byte),
	}
}

func (r *Recorder) RecordRequest(req []byte) {
	r.logger.Printf("Recording request: %q", req)
	r.reqChan <- req
}

func (r *Recorder) RecordResponse(res []byte) {
	r.logger.Printf("Recording response chunk: %q", res)
	r.resChan <- res
}

func (r *Recorder) writeRecording() error {
	if r.filename == "" {
		r.logger.Println("No recording filename specified, skipping write")
		return nil
	}

	r.logger.Printf("Writing recording to %s", r.filename)

	emuConfig := emulatorConfig.EmulatorConfig{
		Mappings: make([]emulatorConfig.RequestResponse, 0, len(r.requests)),
	}

	for key := range r.requests {
		emuConfig.Mappings = append(emuConfig.Mappings, r.requests[key])
	}

	data, err := yaml.Marshal(emuConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(r.filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	r.logger.Printf("Recording written to %s", r.filename)

	return nil
}

func (r *Recorder) recordLoop(ctx context.Context) {
	var currentResponse *emulatorConfig.ResponseOption
	var currentRequestTime time.Time

	for {
		select {
		case <-ctx.Done():
			r.logger.Println("Recorder stopping")

			if err := r.writeRecording(); err != nil {
				r.logger.Printf("Error writing recording: %v", err)
			}

			return
		case req := <-r.reqChan:
			r.logger.Printf("Received request to record: %s", req)
			currentRequestTime = time.Now()

			reqStr := string(req)

			if _, exists := r.requests[reqStr]; !exists {
				r.requests[reqStr] = emulatorConfig.RequestResponse{
					Request: reqStr,
					Responses: []emulatorConfig.ResponseOption{
						{Chunks: []emulatorConfig.ResponseChunk{}},
					},
				}
			} else {
				// Append a new response option for subsequent responses to the same request
				r.requests[reqStr] = emulatorConfig.RequestResponse{
					Request: reqStr,
					Responses: append(r.requests[reqStr].Responses, emulatorConfig.ResponseOption{
						Chunks: []emulatorConfig.ResponseChunk{},
					}),
				}
			}

			currentResponse = &r.requests[reqStr].Responses[len(r.requests[reqStr].Responses)-1]
		case res := <-r.resChan:
			if currentResponse == nil {
				r.logger.Printf("Warning: %v: %s", ErrResponseWithoutRequest, res)
				continue
			}

			chunk := emulatorConfig.ResponseChunk{
				Data: strconv.Quote(string(res)),
			}

			// Set the delay based on the time since the request was recorded
			chunk.Delay = time.Since(currentRequestTime)
			chunk.JitterMax = chunk.Delay / 10 // 10% of the delay
			currentResponse.Chunks = append(currentResponse.Chunks, chunk)

			// Update the request time for the next chunk
			currentRequestTime = time.Now()
		}
	}
}

// Start begins the recording process
func (r *Recorder) Start(ctx context.Context) {
	r.logger.Println("Recorder started")

	go func() {
		r.recordLoop(ctx)
	}()
}
