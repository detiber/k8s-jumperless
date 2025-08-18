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
	config          *Config
	ptmx            *os.File // Primary side of pty
	pts             *os.File // Secondary side of pty (optional, for testing)
	logger          *log.Logger
	shutdown        chan struct{}
	requestCounters map[string]int // Track request counts for sequential responses
}

// New creates a new emulator instance
func New(config *Config, logger *log.Logger) (*Emulator, error) {
	if logger == nil {
		logger = log.New(os.Stdout, "[emulator] ", log.LstdFlags)
	}

	return &Emulator{
		config:          config,
		logger:          logger,
		shutdown:        make(chan struct{}),
		requestCounters: make(map[string]int),
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
			if matched, _ := regexp.MatchString(mapping.Request, request); matched {
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
	// Update request counter for this mapping
	requestKey := mapping.Request
	e.requestCounters[requestKey]++

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

	// Get the appropriate response (handling multiple responses)
	responseText := mapping.GetResponse(e.requestCounters[requestKey])

	// Handle regex substitutions
	if mapping.IsRegex {
		if regex, err := regexp.Compile(mapping.Request); err == nil {
			responseText = regex.ReplaceAllString(originalRequest, responseText)
		}
	}

	// Process hardware state updates and placeholders
	responseText = e.processResponse(responseText, originalRequest)

	e.logger.Printf("Sending response: %q", responseText)

	// Send response (chunked or all at once)
	if mapping.ResponseConfig.Chunked && mapping.ResponseConfig.ChunkSize > 0 {
		e.sendChunkedResponse(responseText, mapping.ResponseConfig)
	} else {
		e.sendFullResponse(responseText)
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

// processResponse processes the response text, handling hardware state updates and placeholders
func (e *Emulator) processResponse(response, request string) string {
	// Handle hardware commands and update state
	e.processHardwareCommand(request)

	// Replace placeholders with current hardware state
	return e.replacePlaceholders(response, request)
}

// processHardwareCommand processes hardware-related commands and updates internal state
func (e *Emulator) processHardwareCommand(request string) {
	// DAC set commands: set_dac(channel, voltage)
	if matched, _ := regexp.MatchString(`set_dac\((\d+),\s*([+-]?\d*\.?\d+)\)`, request); matched {
		regex := regexp.MustCompile(`set_dac\((\d+),\s*([+-]?\d*\.?\d+)\)`)
		matches := regex.FindStringSubmatch(request)
		if len(matches) >= 3 {
			channel := matches[1]
			voltage := parseFloat(matches[2])
			if voltage >= -8.0 && voltage <= 8.0 {
				if e.config.Jumperless.DACChannels == nil {
					e.config.Jumperless.DACChannels = make(map[string]DACChannel)
				}
				e.config.Jumperless.DACChannels[channel] = DACChannel{Voltage: voltage}
				e.logger.Printf("Updated DAC channel %s to %.2fV", channel, voltage)
			}
		}
	}

	// GPIO set commands: gpio_set(pin, value)
	if matched, _ := regexp.MatchString(`gpio_set\((\d+),\s*([01])\)`, request); matched {
		regex := regexp.MustCompile(`gpio_set\((\d+),\s*([01])\)`)
		matches := regex.FindStringSubmatch(request)
		if len(matches) >= 3 {
			pin := matches[1]
			value := parseInt(matches[2])
			if e.config.Jumperless.GPIOPins == nil {
				e.config.Jumperless.GPIOPins = make(map[string]GPIOPin)
			}
			currentPin := e.config.Jumperless.GPIOPins[pin]
			currentPin.Value = value
			e.config.Jumperless.GPIOPins[pin] = currentPin
			e.logger.Printf("Updated GPIO pin %s to %d", pin, value)
		}
	}

	// Connection commands: connect(nodeA, nodeB)
	if matched, _ := regexp.MatchString(`connect\(([^,]+),\s*([^)]+)\)`, request); matched {
		regex := regexp.MustCompile(`connect\(([^,]+),\s*([^)]+)\)`)
		matches := regex.FindStringSubmatch(request)
		if len(matches) >= 3 {
			nodeA := strings.TrimSpace(matches[1])
			nodeB := strings.TrimSpace(matches[2])
			e.addConnection(nodeA, nodeB)
			e.logger.Printf("Connected nodes %s and %s", nodeA, nodeB)
		}
	}

	// Disconnect commands: disconnect(nodeA, nodeB)
	if matched, _ := regexp.MatchString(`disconnect\(([^,]+),\s*([^)]+)\)`, request); matched {
		regex := regexp.MustCompile(`disconnect\(([^,]+),\s*([^)]+)\)`)
		matches := regex.FindStringSubmatch(request)
		if len(matches) >= 3 {
			nodeA := strings.TrimSpace(matches[1])
			nodeB := strings.TrimSpace(matches[2])
			e.removeConnection(nodeA, nodeB)
			e.logger.Printf("Disconnected nodes %s and %s", nodeA, nodeB)
		}
	}

	// Clear all connections: clear()
	if matched, _ := regexp.MatchString(`clear\(\)`, request); matched {
		e.config.Jumperless.Connections = []Connection{}
		e.logger.Printf("Cleared all connections")
	}
}

// replacePlaceholders replaces placeholders in response with current hardware state
func (e *Emulator) replacePlaceholders(response, request string) string {
	result := response

	// Replace DAC voltage placeholders: {{dac_voltage:channel}}
	dacRegex := regexp.MustCompile(`\{\{dac_voltage:(\w+)\}\}`)
	result = dacRegex.ReplaceAllStringFunc(result, func(match string) string {
		channel := dacRegex.FindStringSubmatch(match)[1]
		if dac, exists := e.config.Jumperless.DACChannels[channel]; exists {
			return fmt.Sprintf("%.2fV", dac.Voltage)
		}
		return "0.00V"
	})

	// Replace ADC voltage placeholders: {{adc_voltage:channel}}
	adcRegex := regexp.MustCompile(`\{\{adc_voltage:(\w+)\}\}`)
	result = adcRegex.ReplaceAllStringFunc(result, func(match string) string {
		channel := adcRegex.FindStringSubmatch(match)[1]
		if adc, exists := e.config.Jumperless.ADCChannels[channel]; exists {
			return fmt.Sprintf("%.2fV", adc.Voltage)
		}
		return "0.00V"
	})

	// Replace GPIO value placeholders: {{gpio_value:pin}}
	gpioRegex := regexp.MustCompile(`\{\{gpio_value:(\w+)\}\}`)
	result = gpioRegex.ReplaceAllStringFunc(result, func(match string) string {
		pin := gpioRegex.FindStringSubmatch(match)[1]
		if gpio, exists := e.config.Jumperless.GPIOPins[pin]; exists {
			return fmt.Sprintf("%d", gpio.Value)
		}
		return "0"
	})

	// Replace connection status placeholders: {{is_connected:nodeA:nodeB}}
	connRegex := regexp.MustCompile(`\{\{is_connected:([^:]+):([^}]+)\}\}`)
	result = connRegex.ReplaceAllStringFunc(result, func(match string) string {
		matches := connRegex.FindStringSubmatch(match)
		if len(matches) >= 3 {
			nodeA := matches[1]
			nodeB := matches[2]
			if e.isConnected(nodeA, nodeB) {
				return "true"
			}
		}
		return "false"
	})

	return result
}

// Helper functions for connection management
func (e *Emulator) addConnection(nodeA, nodeB string) {
	// Ensure we don't add duplicate connections
	for _, conn := range e.config.Jumperless.Connections {
		if (conn.NodeA == nodeA && conn.NodeB == nodeB) || (conn.NodeA == nodeB && conn.NodeB == nodeA) {
			return // Already connected
		}
	}
	e.config.Jumperless.Connections = append(e.config.Jumperless.Connections, Connection{
		NodeA: nodeA,
		NodeB: nodeB,
	})
}

func (e *Emulator) removeConnection(nodeA, nodeB string) {
	for i, conn := range e.config.Jumperless.Connections {
		if (conn.NodeA == nodeA && conn.NodeB == nodeB) || (conn.NodeA == nodeB && conn.NodeB == nodeA) {
			e.config.Jumperless.Connections = append(e.config.Jumperless.Connections[:i], e.config.Jumperless.Connections[i+1:]...)
			return
		}
	}
}

func (e *Emulator) isConnected(nodeA, nodeB string) bool {
	for _, conn := range e.config.Jumperless.Connections {
		if (conn.NodeA == nodeA && conn.NodeB == nodeB) || (conn.NodeA == nodeB && conn.NodeB == nodeA) {
			return true
		}
	}
	return false
}

// Helper functions for parsing
func parseFloat(s string) float64 {
	if val, err := regexp.MatchString(`^[+-]?\d*\.?\d+$`, s); err == nil && val {
		// Simple float parsing for demo
		var result float64
		fmt.Sscanf(s, "%f", &result)
		return result
	}
	return 0.0
}

func parseInt(s string) int {
	if val, err := regexp.MatchString(`^\d+$`, s); err == nil && val {
		var result int
		fmt.Sscanf(s, "%d", &result)
		return result
	}
	return 0
}
