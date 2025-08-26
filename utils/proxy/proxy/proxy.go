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
	"io"
	"log"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/detiber/k8s-jumperless/jumperless"
	"github.com/detiber/k8s-jumperless/utils/proxy/proxy/config"
	"go.bug.st/serial"
)

var ErrNoJumperlessDevice = errors.New("no Jumperless device found")

// Proxy represents a serial port proxy that records communication
type Proxy struct {
	config         *config.ProxyConfig
	logger         *log.Logger
	recorder       *Recorder
	pseudoTTY      *os.File // This is what we listen on for user input
	virtualTTY     *os.File // This is what we return to the user as the virtual port
	realPort       serial.Port
	cancelV2R      context.CancelCauseFunc
	cancelR2V      context.CancelCauseFunc
	cancelRecorder context.CancelCauseFunc
	wg             sync.WaitGroup
}

// New creates a new proxy instance
func New(c *config.ProxyConfig, logger *log.Logger) (*Proxy, error) {
	if logger == nil {
		logger = log.New(os.Stdout, "[proxy] ", log.LstdFlags)
	}

	return &Proxy{
		config:   c,
		logger:   logger,
		recorder: NewRecorder(logger, c.EmulatorConfig),
	}, nil
}

// Start starts the proxy
func (p *Proxy) Start(ctx context.Context) error {
	// Create virtual serial port (pty)
	pseudoTTY, virtualTTY, err := pty.Open()
	if err != nil {
		return fmt.Errorf("failed to create pty: %w", err)
	}

	// Ensure non-blocking reads on pseudo TTY, this allows us to implement read timeouts
	fd := pseudoTTY.Fd()
	if err := syscall.SetNonblock(int(fd), true); err != nil {
		p.tryCleanup()
		return fmt.Errorf("failed to set pseudo TTY to non-blocking: %w", err)
	}

	p.pseudoTTY = pseudoTTY
	p.virtualTTY = virtualTTY

	// Create symlink to the configured virtual port name if specified
	if p.config.VirtualPort != "" && p.config.VirtualPort != virtualTTY.Name() {
		// Remove existing symlink if it exists
		if err := os.Remove(p.config.VirtualPort); err != nil && !os.IsNotExist(err) {
			p.tryCleanup() // Clean up if symlink creation fails
			return fmt.Errorf("failed to remove existing virtual port %s: %w", p.config.VirtualPort, err)
		}

		// Create symlink
		if err := os.Symlink(virtualTTY.Name(), p.config.VirtualPort); err != nil {
			p.tryCleanup() // Clean up if symlink creation fails
			return fmt.Errorf("failed to create symlink %s -> %s: %w", p.config.VirtualPort, virtualTTY.Name(), err)
		}
		p.logger.Printf("Created virtual serial port: %s -> %s", p.config.VirtualPort, virtualTTY.Name())
	} else {
		p.logger.Printf("Created virtual serial port: %s", virtualTTY.Name())
	}

	// Open real serial port
	mode := &serial.Mode{
		BaudRate: p.config.BaudRate,
	}

	if p.config.RealPort == "" {
		p.logger.Printf("No real port configured, attempting to detect...")

		j, err := jumperless.NewJumperless(ctx, p.config.RealPort, p.config.BaudRate)
		if err != nil {
			p.tryCleanup()
			return fmt.Errorf("failed to create Jumperless instance for port detection: %w", err)
		}

		if j == nil {
			p.tryCleanup()
			return ErrNoJumperlessDevice
		}

		p.config.RealPort = j.GetPort()
		version := j.GetVersion()

		p.logger.Printf("Detected Jumperless port: %s (version: %s)", p.config.RealPort, version)
	}

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

	// Start recorder
	recorderctx, cancelRecorder := context.WithCancelCause(ctx)
	p.cancelRecorder = cancelRecorder
	p.recorder.Start(recorderctx)

	// Start proxy goroutines
	v2rctx, cancelV2R := context.WithCancelCause(ctx)
	p.cancelV2R = cancelV2R
	p.wg.Go(func() { p.proxyVirtualToReal(v2rctx) })

	r2vctx, cancelR2V := context.WithCancelCause(ctx)
	p.cancelR2V = cancelR2V
	p.wg.Go(func() { p.proxyRealToVirtual(r2vctx) })

	return nil
}

func (p *Proxy) tryCleanup() {
	// Close pseudo TTY
	if p.pseudoTTY != nil {
		if err := p.pseudoTTY.Close(); err != nil {
			p.logger.Printf("Warning: failed to close pseudo TTY: %v", err)
		} else {
			p.logger.Printf("Closed pseudo TTY: %s", p.pseudoTTY.Name())
		}
	}

	// Close virtual TTY
	if p.virtualTTY != nil {
		if err := p.virtualTTY.Close(); err != nil {
			p.logger.Printf("Warning: failed to close virtual TTY: %v", err)
		} else {
			p.logger.Printf("Closed virtual TTY: %s", p.virtualTTY.Name())
		}
	}

	// Close real serial port
	if p.realPort != nil {
		if err := p.realPort.Close(); err != nil {
			p.logger.Printf("Warning: failed to close real serial port: %v", err)
		} else {
			p.logger.Printf("Closed real serial port: %s", p.config.RealPort)
		}
	}

	// Remove symlink if it was created
	if p.config.VirtualPort != "" {
		if err := os.Remove(p.config.VirtualPort); err != nil && !os.IsNotExist(err) {
			p.logger.Printf("Warning: failed to remove virtual port %s: %v", p.config.VirtualPort, err)
		} else {
			p.logger.Printf("Removed virtual port symlink: %s", p.config.VirtualPort)
		}
	}
}

// Stop stops the proxy
func (p *Proxy) Stop() error {
	// Cancel proxy goroutines
	if p.cancelV2R != nil {
		p.cancelV2R(nil)
	}

	if p.cancelR2V != nil {
		p.cancelR2V(nil)
	}

	if p.cancelRecorder != nil {
		p.cancelRecorder(nil)
	}

	p.wg.Wait()

	p.tryCleanup()

	return nil
}

// proxyVirtualToReal forwards data from virtual port to real port (requests)
func (p *Proxy) proxyVirtualToReal(ctx context.Context) {
	p.logger.Printf("Starting to proxy data from virtual port %s to real port %s", p.virtualTTY.Name(), p.config.RealPort)
	buffer := make([]byte, p.config.BufferSize)

	defer func() {
		p.logger.Printf("Stopped proxying data from virtual port to real port")
	}()

	for {
		select {
		case <-ctx.Done():
			p.logger.Printf("Context done, stopping proxyVirtualToReal")
			return
		default:
			// Set read timeout
			if err := p.pseudoTTY.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
				p.logger.Printf("Warning: failed to set read deadline: %v", err)
			}

			n, err := p.pseudoTTY.Read(buffer)
			if err != nil {
				if os.IsTimeout(err) {
					continue // Timeout is expected
				}
				if errors.Is(err, io.EOF) {
					p.logger.Printf("Virtual port client disconnected")
					continue
				}
				p.logger.Printf("Error reading from virtual port: %v", err)
				continue
			}

			if n > 0 {
				data := buffer[:n]

				// // Record request
				p.recorder.RecordRequest(data)

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
	p.logger.Printf("Starting to proxy data from real port %s to virtual port %s", p.config.RealPort, p.virtualTTY.Name())

	buffer := make([]byte, p.config.BufferSize)

	// Set read timeout
	if err := p.realPort.SetReadTimeout(500 * time.Millisecond); err != nil {
		p.logger.Printf("Error setting read timeout: %v", err)
		return
	}

	defer func() {
		p.logger.Printf("Stopped proxying data from real port to virtual port")
	}()

	for {
		select {
		case <-ctx.Done():
			p.logger.Printf("Context done, stopping proxyRealToVirtual")
			return
		default:
			n, err := p.realPort.Read(buffer)
			if err != nil {
				if os.IsTimeout(err) {
					continue // Timeout is expected
				}
				p.logger.Printf("Error reading from real port: %v", err)
				continue
			}

			if n > 0 {
				data := buffer[:n]

				p.recorder.RecordResponse(data)

				// Forward to virtual port
				if _, err := p.pseudoTTY.Write(data); err != nil {
					p.logger.Printf("Error writing to virtual port: %v", err)
				}

				p.logger.Printf("Response: %q", string(data))
			}
		}
	}
}

// GetVirtualPortName returns the virtual port name
func (p *Proxy) GetVirtualPortName() string {
	if p.config.VirtualPort != "" {
		return p.config.VirtualPort
	}
	if p.virtualTTY != nil {
		return p.virtualTTY.Name()
	}
	return ""
}
