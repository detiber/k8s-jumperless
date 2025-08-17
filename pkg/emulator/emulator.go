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
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/creack/pty"
)

// Emulator represents a Jumperless device emulator
type Emulator struct {
	config   *Config
	ptmx     *os.File // Master side of pty
	pts      *os.File // Slave side of pty (optional, for testing)
	logger   *log.Logger
	shutdown chan struct{}
}

// New creates a new emulator instance
func New(config *Config, logger *log.Logger) (*Emulator, error) {
	if logger == nil {
		logger = log.New(os.Stdout, "[emulator] ", log.LstdFlags)
	}

	// Compile regex patterns
	for i := range config.Mappings {
		mapping := &config.Mappings[i]
		if mapping.IsRegex {
			regex, err := regexp.Compile(mapping.Request)
			if err != nil {
				return nil, fmt.Errorf("failed to compile regex pattern %q: %w", mapping.Request, err)
			}
			mapping.CompiledRegex = regex
		}
	}

	return &Emulator{
		config:   config,
		logger:   logger,
		shutdown: make(chan struct{}),
	}, nil
}

// Start starts the emulator
func (e *Emulator) Start(ctx context.Context) error {
	// Create a pty
	ptmx, pts, err := pty.Open()
	if err != nil {
		return fmt.Errorf("failed to create pty: %w", err)
	}

	e.ptmx = ptmx
	e.pts = pts

	// Create symlink to the configured port name if specified
	if e.config.Serial.Port != "" && e.config.Serial.Port != pts.Name() {
		// Remove existing symlink if it exists
		if err := os.Remove(e.config.Serial.Port); err != nil && !os.IsNotExist(err) {
			e.logger.Printf("Warning: failed to remove existing port %s: %v", e.config.Serial.Port, err)
		}

		// Create symlink
		if err := os.Symlink(pts.Name(), e.config.Serial.Port); err != nil {
			return fmt.Errorf("failed to create symlink %s -> %s: %w", e.config.Serial.Port, pts.Name(), err)
		}
		e.logger.Printf("Created virtual serial port: %s -> %s", e.config.Serial.Port, pts.Name())
	} else {
		e.logger.Printf("Created virtual serial port: %s", pts.Name())
	}

	// Start handling requests
	go e.handleRequests(ctx)

	return nil
}

// Stop stops the emulator
func (e *Emulator) Stop() error {
	close(e.shutdown)

	if e.ptmx != nil {
		e.ptmx.Close()
	}
	if e.pts != nil {
		e.pts.Close()
	}

	// Clean up symlink if we created one
	if e.config.Serial.Port != "" {
		if err := os.Remove(e.config.Serial.Port); err != nil && !os.IsNotExist(err) {
			e.logger.Printf("Warning: failed to clean up port symlink %s: %v", e.config.Serial.Port, err)
		}
	}

	return nil
}

// GetPortName returns the actual port name
func (e *Emulator) GetPortName() string {
	if e.config.Serial.Port != "" {
		return e.config.Serial.Port
	}
	if e.pts != nil {
		return e.pts.Name()
	}
	return ""
}

// handleRequests handles incoming requests from the serial port
func (e *Emulator) handleRequests(ctx context.Context) {
	buffer := make([]byte, e.config.Serial.BufferSize)
	requestBuffer := strings.Builder{}

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.shutdown:
			return
		default:
			// Set read timeout
			e.ptmx.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

			n, err := e.ptmx.Read(buffer)
			if err != nil {
				if os.IsTimeout(err) {
					continue // Timeout is expected
				}
				if err == io.EOF {
					e.logger.Printf("Client disconnected")
					continue
				}
				e.logger.Printf("Error reading from pty: %v", err)
				continue
			}

			if n > 0 {
				data := string(buffer[:n])
				requestBuffer.WriteString(data)

				// Process complete requests (assuming they end with newline or are single commands)
				request := strings.TrimSpace(requestBuffer.String())
				if request != "" {
					e.logger.Printf("Received request: %q", request)

					// Find matching response
					response := e.findResponse(request)
					if response != nil {
						go e.sendResponse(response, request)
					} else {
						e.logger.Printf("No response configured for request: %q", request)
					}

					requestBuffer.Reset()
				}
			}
		}
	}
}

// findResponse finds the appropriate response for a request
func (e *Emulator) findResponse(request string) *RequestResponse {
	for _, mapping := range e.config.Mappings {
		if mapping.IsRegex {
			if mapping.CompiledRegex != nil && mapping.CompiledRegex.MatchString(request) {
				return &mapping
			}
		} else {
			if strings.Contains(request, mapping.Request) {
				return &mapping
			}
		}
	}
	return nil
}

// sendResponse sends a response with configured delays and chunking
func (e *Emulator) sendResponse(mapping *RequestResponse, originalRequest string) {
	// Calculate delay with jitter
	delay := mapping.ResponseConfig.Delay
	if mapping.ResponseConfig.JitterMax > 0 {
		jitter := time.Duration(rand.Int63n(int64(mapping.ResponseConfig.JitterMax)))
		delay += jitter
	}

	// Wait for the delay
	if delay > 0 {
		time.Sleep(delay)
	}

	// Prepare response (handle regex substitutions)
	response := mapping.Response
	if mapping.IsRegex && mapping.CompiledRegex != nil {
		response = mapping.CompiledRegex.ReplaceAllString(originalRequest, mapping.Response)
	}

	e.logger.Printf("Sending response: %q", response)

	// Send response (chunked or all at once)
	if mapping.ResponseConfig.Chunked && mapping.ResponseConfig.ChunkSize > 0 {
		e.sendChunkedResponse(response, mapping.ResponseConfig)
	} else {
		e.sendFullResponse(response)
	}
}

// sendFullResponse sends the complete response at once
func (e *Emulator) sendFullResponse(response string) {
	if _, err := e.ptmx.Write([]byte(response)); err != nil {
		e.logger.Printf("Error writing response: %v", err)
	}
}

// sendChunkedResponse sends the response in chunks with delays
func (e *Emulator) sendChunkedResponse(response string, config ResponseConfig) {
	data := []byte(response)
	chunkSize := config.ChunkSize

	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}

		chunk := data[i:end]
		if _, err := e.ptmx.Write(chunk); err != nil {
			e.logger.Printf("Error writing chunk: %v", err)
			return
		}

		// Delay between chunks (except after the last chunk)
		if end < len(data) && config.ChunkDelay > 0 {
			time.Sleep(config.ChunkDelay)
		}
	}
}
