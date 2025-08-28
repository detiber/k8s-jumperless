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

package generator

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/detiber/k8s-jumperless/jumperless"
	"github.com/detiber/k8s-jumperless/utils/internal/generator/config"
	"go.bug.st/serial"
)

var ErrNoJumperlessDevice = errors.New("no Jumperless device found")

// generator represents a serial port generator that records communication
type generator struct {
	config *config.GeneratorConfig
	logger *log.Logger
}

// New creates a new generator instance
func New(c *config.GeneratorConfig, logger *log.Logger) (*generator, error) {
	if logger == nil {
		logger = log.New(os.Stdout, "[generator] ", log.LstdFlags)
	}

	return &generator{
		config: c,
		logger: logger,
	}, nil
}

// Run starts the generator
func (p *generator) Run(ctx context.Context) error {
	// Open real serial port
	mode := &serial.Mode{
		BaudRate: p.config.BaudRate,
	}

	if p.config.Port == "" {
		p.logger.Printf("No real port configured, attempting to detect...")

		j, err := jumperless.NewJumperless(ctx, p.config.Port, p.config.BaudRate)
		if err != nil {
			return fmt.Errorf("failed to create Jumperless instance for port detection: %w", err)
		}

		if j == nil {
			return ErrNoJumperlessDevice
		}

		p.config.Port = j.GetPort()
		version := j.GetVersion()

		p.logger.Printf("Detected Jumperless port: %s (version: %s)", p.config.Port, version)
	}

	port, err := serial.Open(p.config.Port, mode)
	if err != nil {
		return fmt.Errorf("failed to open serial port %s: %w", p.config.Port, err)
	}

	defer func() {
		if err := port.Close(); err != nil {
			p.logger.Printf("Warning: failed to close serial port: %v", err)
		} else {
			p.logger.Printf("Closed serial port: %s", p.config.Port)
		}
	}()

	go func() {
		<-ctx.Done()

		p.logger.Printf("Context done, forcing shutdown by closing port %s", p.config.Port)
		if err := port.Close(); err != nil {
			p.logger.Printf("Warning: failed to close serial port: %v", err)
		} else {
			p.logger.Printf("Closed serial port: %s", p.config.Port)
		}
	}()

	p.logger.Printf("Connected to serial port: %s", p.config.Port)

	p.logger.Printf("Starting generator with %d requests", len(p.config.Requests))

	readBuffer := make([]byte, p.config.BufferSize)

	for _, req := range p.config.Requests {
		// Reset buffers
		if err := port.ResetInputBuffer(); err != nil {
			return fmt.Errorf("failed to reset input buffer on port %s: %w", p.config.Port, err)
		}

		if err := port.ResetOutputBuffer(); err != nil {
			return fmt.Errorf("failed to reset output buffer on port %s: %w", p.config.Port, err)
		}

		// Send request
		p.logger.Printf("Sending request: %q", req.Data)
		if _, err := port.Write([]byte(req.Data)); err != nil {
			return fmt.Errorf("error writing to port %s: %w", p.config.Port, err)
		}

		// Drain to ensure all data is sent
		if err := port.Drain(); err != nil {
			p.logger.Printf("Error draining real port: %v", err)
		}

		// Read response
		if req.Timeout > 0 {
			if err := port.SetReadTimeout(req.Timeout); err != nil {
				return fmt.Errorf("failed to set read timeout on port %s: %w", p.config.Port, err)
			}
		}

		if _, err := port.Read(readBuffer); err != nil {
			return fmt.Errorf("error reading from port %s: %w", p.config.Port, err)
		}

		p.logger.Printf("Received response: %q", readBuffer)
	}

	return nil
}
