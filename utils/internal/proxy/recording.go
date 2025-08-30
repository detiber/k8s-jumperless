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
	"log"
	"strconv"
	"time"

	emulatorConfig "github.com/detiber/k8s-jumperless/utils/internal/emulator/config"
)

var (
	ErrResponseWithoutRequest      = errors.New("received response without preceding request")
	ErrUnsupportedOutputFormat     = errors.New("unsupported output format (use yaml, json, or log)")
	ErrUnsupportedConfigFileFormat = errors.New("unsupported config file format (use .yaml, .yml, or .json)")
)

// Recorder handles recording of serial port interactions
type Recorder struct {
	logger   *log.Logger
	requests emulatorConfig.Mappings
	reqChan  chan []byte
	resChan  chan []byte
}

// NewRecorder creates a new Recorder instance
func NewRecorder(logger *log.Logger) *Recorder {
	return &Recorder{
		logger:   logger,
		requests: make(emulatorConfig.Mappings, 0),
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

func (r *Recorder) GetRecording() emulatorConfig.Mappings {
	return r.requests
}

// Run the Recorder
// The Recorder will run until the context is cancelled
func (r *Recorder) Run(ctx context.Context) {
	var currentRequest string
	var currentResponse *emulatorConfig.ResponseOption
	var currentRequestTime time.Time

	defer (func() {
		// Ensure that we finalize the last recording if needed
		if currentRequest != "" && currentResponse != nil {
			r.logger.Printf("Finalizing recording for request: %s", currentRequest)
			r.requests.AddResponse(currentRequest, *currentResponse)
		}
	})()

	for {
		select {
		case <-ctx.Done():
			r.logger.Println("Recorder stopping")
			return
		case req := <-r.reqChan:
			r.logger.Printf("Received request to record: %s", req)

			if currentRequest != "" && currentResponse != nil {
				r.logger.Printf("Saving recording for previous request: %s", currentRequest)
				r.requests.AddResponse(currentRequest, *currentResponse)
			}

			currentRequestTime = time.Now()
			currentRequest = string(req)
			currentResponse = new(emulatorConfig.ResponseOption)
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
