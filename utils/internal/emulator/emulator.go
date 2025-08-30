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
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/detiber/k8s-jumperless/utils/internal/emulator/config"
)

var ErrNoResponsesConfigured = errors.New("no responses configured")
var ErrPartialWrite = errors.New("partial write")

// Emulator represents a Jumperless device emulator
type Emulator struct {
	config          *config.EmulatorConfig
	logger          *log.Logger
	pseudoTTY       *os.File // This is what we listen on for user input
	virtualTTY      *os.File // This is what we return to the user as the virtual port
	cancel          context.CancelCauseFunc
	wg              sync.WaitGroup
	requestCounters map[string]int // Track request counts for sequential responses
}

// New creates a new emulator instance
func New(c *config.EmulatorConfig, logger *log.Logger) (*Emulator, error) {
	if logger == nil {
		logger = log.New(os.Stdout, "[emulator] ", log.LstdFlags)
	}

	return &Emulator{
		config:          c,
		logger:          logger,
		requestCounters: make(map[string]int, len(c.Mappings)),
	}, nil
}

// Start starts the emulator
func (e *Emulator) Start(ctx context.Context) error {
	// Create virtual serial port (pty)
	pseudoTTY, virtualTTY, err := pty.Open()
	if err != nil {
		return fmt.Errorf("failed to create pty: %w", err)
	}

	// Ensure non-blocking reads on pseudo TTY, this allows us to implement read timeouts
	fd := pseudoTTY.Fd()
	if err := syscall.SetNonblock(int(fd), true); err != nil {
		e.tryCleanup()
		return fmt.Errorf("failed to set pseudo TTY to non-blocking: %w", err)
	}

	e.pseudoTTY = pseudoTTY
	e.virtualTTY = virtualTTY

	// Create symlink to the configured virtual port name if specified
	if e.config.VirtualPort != "" && e.config.VirtualPort != virtualTTY.Name() {
		// Remove existing symlink if it exists
		if err := os.Remove(e.config.VirtualPort); err != nil && !os.IsNotExist(err) {
			e.tryCleanup() // Clean up if symlink creation fails
			return fmt.Errorf("failed to remove existing virtual port %s: %w", e.config.VirtualPort, err)
		}

		// Create symlink
		if err := os.Symlink(virtualTTY.Name(), e.config.VirtualPort); err != nil {
			e.tryCleanup() // Clean up if symlink creation fails
			return fmt.Errorf("failed to create symlink %s -> %s: %w", e.config.VirtualPort, virtualTTY.Name(), err)
		}
		e.logger.Printf("Created virtual serial port: %s -> %s", e.config.VirtualPort, virtualTTY.Name())
	} else {
		e.logger.Printf("Created virtual serial port: %s", virtualTTY.Name())
	}

	// Start recorder
	handlerctx, cancel := context.WithCancelCause(ctx)
	e.cancel = cancel
	e.wg.Go(func() { e.handleRequests(handlerctx) })

	return nil
}

// handleRequests handles incoming requests from the serial port
func (e *Emulator) handleRequests(ctx context.Context) {
	buffer := make([]byte, e.config.BufferSize)
	requestBuffer := strings.Builder{}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := e.pseudoTTY.Read(buffer)
			if err != nil {
				if os.IsTimeout(err) {
					continue // Timeout is expected
				}
				if errors.Is(err, io.EOF) {
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
						if err := e.sendResponse(response); err != nil {
							e.logger.Printf("Error sending response: %v", err)
						}
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
func (e *Emulator) findResponse(request string) *config.RequestResponse {
	for _, mapping := range e.config.Mappings {
		if strings.TrimSpace(request) == strings.TrimSpace(mapping.Request) {
			return &mapping
		}
	}

	return nil
}

// sendResponse sends a response with configured delays and chunking
func (e *Emulator) sendResponse(mapping *config.RequestResponse) error {
	requestKey := mapping.Request
	requestIndex := e.requestCounters[requestKey]

	switch len(mapping.Responses) {
	case 0:
		return fmt.Errorf("%w: %q", ErrNoResponsesConfigured, mapping.Request)
	case 1:
		requestIndex = 0
	default:
		requestIndex %= len(mapping.Responses)
	}

	// Update request counter for this mapping
	e.requestCounters[requestKey]++

	response := mapping.Responses[requestIndex]

	for _, chunk := range response.Chunks {
		delay := chunk.Delay

		if chunk.JitterMax > 0 {
			jitter := time.Duration(rand.Int63n(int64(chunk.JitterMax))) //nolint:gosec
			delay += jitter
		}

		if delay > 0 {
			time.Sleep(delay)
		}

		responseText := chunk.Data

		// try to unquote the response chunk
		unquoted, err := strconv.Unquote(responseText)
		if err != nil {
			// if unquoting fails, just use the original string
			e.logger.Printf("Warning: failed to unquote response chunk %q: %v", responseText, err)
		} else {
			responseText = unquoted
		}

		n, err := e.pseudoTTY.Write([]byte(responseText))
		if err != nil {
			return fmt.Errorf("failed to write response to pty: %w", err)
		}
		if n != len(responseText) {
			return fmt.Errorf("%w: wrote %d of %d bytes", ErrPartialWrite, n, len(responseText))
		}

		e.logger.Printf("Sent response chunk: %q", responseText)
	}

	return nil
}

func (e *Emulator) tryCleanup() {
	// Close pseudo TTY
	if e.pseudoTTY != nil {
		if err := e.pseudoTTY.Close(); err != nil {
			e.logger.Printf("Warning: failed to close pseudo TTY: %v", err)
		} else {
			e.logger.Printf("Closed pseudo TTY: %s", e.pseudoTTY.Name())
		}
	}

	// Close virtual TTY
	if e.virtualTTY != nil {
		if err := e.virtualTTY.Close(); err != nil {
			e.logger.Printf("Warning: failed to close virtual TTY: %v", err)
		} else {
			e.logger.Printf("Closed virtual TTY: %s", e.virtualTTY.Name())
		}
	}

	// Remove symlink if it was created
	if e.config.VirtualPort != "" {
		if err := os.Remove(e.config.VirtualPort); err != nil && !os.IsNotExist(err) {
			e.logger.Printf("Warning: failed to remove virtual port %s: %v", e.config.VirtualPort, err)
		} else {
			e.logger.Printf("Removed virtual port symlink: %s", e.config.VirtualPort)
		}
	}
}

// Stop stops the emulator
func (e *Emulator) Stop() error {
	// Cancel emulator goroutines
	if e.cancel != nil {
		// attempt to cancel between reads/writes
		e.cancel(nil)

		// Give some time for an active read/write to finish
		time.Sleep(100 * time.Millisecond)

		// Force close the pseudo TTY to unblock any active reads
		if err := e.pseudoTTY.Close(); err != nil {
			e.logger.Printf("Warning: failed to close pseudo TTY: %v", err)
		} else {
			e.logger.Printf("Closed pseudo TTY: %s", e.pseudoTTY.Name())
		}

	}

	e.wg.Wait()

	e.tryCleanup()

	return nil
}

// GetPortName returns the actual port name
func (e *Emulator) GetPortName() string {
	if e.config.VirtualPort != "" {
		return e.config.VirtualPort
	}
	if e.virtualTTY != nil {
		return e.virtualTTY.Name()
	}
	return ""
}
