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
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/creack/pty"
	"github.com/detiber/k8s-jumperless/utils/jumperless"
	"go.bug.st/serial"

	"github.com/detiber/k8s-jumperless/utils/jumperless-proxy/proxy/config"
)

// Proxy represents a serial port proxy that records communication
type Proxy struct {
	config   *config.ProxyConfig
	ptmx     *os.File // Master side of pty (virtual port)
	pts      *os.File // Slave side of pty (virtual port)
	realPort serial.Port
	logger   *log.Logger
	recorder *Recorder
	shutdown chan struct{}
}

// New creates a new proxy instance
func New(c *config.ProxyConfig, logger *log.Logger) (*Proxy, error) {
	if logger == nil {
		logger = log.New(os.Stdout, "[proxy] ", log.LstdFlags)
	}

	return &Proxy{
		config:   c,
		logger:   logger,
		shutdown: make(chan struct{}),
		recorder: NewRecorder(logger, c.Recording.File, c.Recording.EmulatorConfig),
	}, nil
}

// Start starts the proxy
func (p *Proxy) Start(ctx context.Context) error {
	// Create virtual serial port (pty)
	ptmx, pts, err := pty.Open()
	if err != nil {
		return fmt.Errorf("failed to create pty: %w", err)
	}

	p.ptmx = ptmx
	p.pts = pts

	// Create symlink to the configured virtual port name if specified
	if p.config.VirtualPort != "" && p.config.VirtualPort != pts.Name() {
		// Remove existing symlink if it exists
		if err := os.Remove(p.config.VirtualPort); err != nil && !os.IsNotExist(err) {
			p.logger.Printf("Warning: failed to remove existing virtual port %s: %v", p.config.VirtualPort, err)
		}

		// Create symlink
		if err := os.Symlink(pts.Name(), p.config.VirtualPort); err != nil {
			p.tryCleanup() // Clean up if symlink creation fails
			return fmt.Errorf("failed to create symlink %s -> %s: %w", p.config.VirtualPort, pts.Name(), err)
		}
		p.logger.Printf("Created virtual serial port: %s -> %s", p.config.VirtualPort, pts.Name())
	} else {
		p.logger.Printf("Created virtual serial port: %s", pts.Name())
	}

	// Open real serial port
	mode := &serial.Mode{
		BaudRate: p.config.BaudRate,
	}

	if p.config.RealPort == "" {
		p.logger.Printf("No real port configured, attempting to detect...")

		ports, err := jumperless.EnumerateSerialPorts()
		if err != nil {
			return fmt.Errorf("failed to enumerate serial ports: %w", err)
		}

		port, version, err := jumperless.FindJumperlessPort(ctx, ports)
		if err != nil {
			return fmt.Errorf("failed to find Jumperless port: %w", err)
		}

		p.config.RealPort = port.Name
		p.logger.Printf("Detected Jumperless port: %s (version: %s)", p.config.RealPort, version)
	}

	// Start recorder
	p.recorder.Start(ctx)

	realPort, err := serial.Open(p.config.RealPort, mode)
	if err != nil {
		return fmt.Errorf("failed to open real serial port %s: %w", p.config.RealPort, err)
	}

	if err := realPort.ResetInputBuffer(); err != nil {
		p.tryCleanup()
		return fmt.Errorf("failed to reset input buffer on real port %s: %w", p.config.RealPort, err)
	}
	if err := realPort.ResetOutputBuffer(); err != nil {
		p.tryCleanup()
		return fmt.Errorf("failed to reset output buffer on real port %s: %w", p.config.RealPort, err)
	}

	p.realPort = realPort
	p.logger.Printf("Connected to real serial port: %s", p.config.RealPort)

	// Start proxy goroutines
	go p.proxyVirtualToReal(ctx)
	go p.proxyRealToVirtual(ctx)

	return nil
}

func (p *Proxy) tryCleanup() {
	if p.ptmx != nil {
		if err := p.ptmx.Close(); err != nil {
			p.logger.Printf("Warning: failed to close ptmx: %v", err)
		}
	}

	if p.pts != nil {
		if err := p.pts.Close(); err != nil {
			p.logger.Printf("Warning: failed to close pts: %v", err)
		}
	}

	if p.realPort != nil {
		if err := p.realPort.Close(); err != nil {
			p.logger.Printf("Warning: failed to close realPort: %v", err)
		}
	}

	// Clean up symlink if we created one
	if p.config.VirtualPort != "" {
		if err := os.Remove(p.config.VirtualPort); err != nil && !os.IsNotExist(err) {
			p.logger.Printf("Warning: failed to clean up virtual port symlink %s: %v", p.config.VirtualPort, err)
		}
	}

	if p.recorder != nil {
		p.recorder.Stop()
	}
}

// Stop stops the proxy
func (p *Proxy) Stop() error {
	close(p.shutdown)

	p.tryCleanup()

	return nil
}

// GetVirtualPortName returns the virtual port name
func (p *Proxy) GetVirtualPortName() string {
	if p.config.VirtualPort != "" {
		return p.config.VirtualPort
	}
	if p.pts != nil {
		return p.pts.Name()
	}
	return ""
}

// proxyVirtualToReal forwards data from virtual port to real port (requests)
func (p *Proxy) proxyVirtualToReal(ctx context.Context) {
	buffer := make([]byte, p.config.BufferSize)

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.shutdown:
			return
		default:
			// Set read timeout
			if err := p.ptmx.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
				p.logger.Printf("Error setting read deadline on virtual port: %v", err)
				continue
			}

			n, err := p.ptmx.Read(buffer)
			if err != nil {
				if os.IsTimeout(err) {
					continue // Timeout is expected
				}
				if err == io.EOF {
					p.logger.Printf("Virtual port client disconnected")
					continue
				}
				p.logger.Printf("Error reading from virtual port: %v", err)
				continue
			}

			if n > 0 {
				data := buffer[:n]

				// Record request
				p.recordEntry(ctx, "request", string(data), 0)

				// Forward to real port
				if _, err := p.realPort.Write(data); err != nil {
					p.logger.Printf("Error writing to real port: %v", err)
				}

				p.logger.Printf("Request: %q", string(data))

				if err := p.realPort.Drain(); err != nil {
					p.logger.Printf("Error draining real port: %v", err)
				}
			}
		}
	}
}

// proxyRealToVirtual forwards data from real port to virtual port (responses)
func (p *Proxy) proxyRealToVirtual(ctx context.Context) {
	buffer := make([]byte, p.config.BufferSize)

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.shutdown:
			return
		default:
			// Set read timeout
			if err := p.realPort.SetReadTimeout(100 * time.Millisecond); err != nil {
				p.logger.Printf("Error setting read timeout: %v", err)
				continue
			}

			startTime := time.Now()
			n, err := p.realPort.Read(buffer)
			duration := time.Since(startTime)

			if err != nil {
				if os.IsTimeout(err) {
					continue // Timeout is expected
				}
				p.logger.Printf("Error reading from real port: %v", err)
				continue
			}

			if n > 0 {
				data := buffer[:n]

				// Record response
				p.recordEntry(ctx, "response", string(data), duration)

				// Forward to virtual port
				if _, err := p.ptmx.Write(data); err != nil {
					p.logger.Printf("Error writing to virtual port: %v", err)
				}

				p.logger.Printf("Response: %q (duration: %v)", string(data), duration)
			}
		}
	}
}

// recordEntry records a communication entry
func (p *Proxy) recordEntry(ctx context.Context, direction, data string, duration time.Duration) {
	p.recorder.Record(ctx, RecordEntry{
		Timestamp: time.Now(),
		Direction: direction,
		Data:      data,
		Duration:  duration,
	})
}
