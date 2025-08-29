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
	"bytes"
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
	emulatorConfig "github.com/detiber/k8s-jumperless/utils/internal/emulator/config"
	"github.com/detiber/k8s-jumperless/utils/internal/proxy/config"
	"go.bug.st/serial"
)

var ErrNoJumperlessDevice = errors.New("no Jumperless device found")

// Proxy represents a serial port proxy that records communication
type Proxy struct {
	config     *config.ProxyConfig
	logger     *log.Logger
	recorder   *Recorder
	pseudoTTY  *os.File // This is what we listen on for user input
	virtualTTY *os.File // This is what we return to the user as the virtual port
	realPort   serial.Port
}

// New creates a new proxy instance
func New(c *config.ProxyConfig, logger *log.Logger) (*Proxy, error) {
	if logger == nil {
		logger = log.New(os.Stdout, "[proxy] ", log.LstdFlags)
	}

	return &Proxy{
		config:   c,
		logger:   logger,
		recorder: NewRecorder(logger),
	}, nil
}

// Run the proxy
// The Run method will block until the context is cancelled or an error occurs
func (p *Proxy) Run(ctx context.Context) (emulatorConfig.Mappings, error) {
	// Create virtual serial port (pty)
	pseudoTTY, virtualTTY, err := pty.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to create pty: %w", err)
	}

	defer func() {
		if err := pseudoTTY.Close(); err != nil {
			p.logger.Printf("Warning: failed to close pseudo TTY: %v", err)
		} else {
			p.logger.Printf("Closed pseudo TTY: %s", pseudoTTY.Name())
		}

		if err := virtualTTY.Close(); err != nil {
			p.logger.Printf("Warning: failed to close virtual TTY: %v", err)
		} else {
			p.logger.Printf("Closed virtual TTY: %s", virtualTTY.Name())
		}
	}()

	// Ensure non-blocking reads on pseudo TTY, this allows us to implement read timeouts
	fd := pseudoTTY.Fd()
	if err := syscall.SetNonblock(int(fd), true); err != nil {
		return nil, fmt.Errorf("failed to set pseudo TTY to non-blocking: %w", err)
	}

	p.pseudoTTY = pseudoTTY
	p.virtualTTY = virtualTTY

	// Create symlink to the configured virtual port name if specified
	if p.config.VirtualPort != "" && p.config.VirtualPort != virtualTTY.Name() {
		// Remove existing symlink if it exists
		if err := os.Remove(p.config.VirtualPort); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to remove existing virtual port %s: %w", p.config.VirtualPort, err)
		}

		// Create symlink
		if err := os.Symlink(virtualTTY.Name(), p.config.VirtualPort); err != nil {
			return nil, fmt.Errorf("failed to create symlink %s -> %s: %w", p.config.VirtualPort, virtualTTY.Name(), err)
		}

		defer func() {
			// Remove symlink if it was created
			if p.config.VirtualPort != "" {
				if err := os.Remove(p.config.VirtualPort); err != nil && !os.IsNotExist(err) {
					p.logger.Printf("Warning: failed to remove virtual port %s: %v", p.config.VirtualPort, err)
				} else {
					p.logger.Printf("Removed virtual port symlink: %s", p.config.VirtualPort)
				}
			}
		}()

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
			return nil, fmt.Errorf("failed to create Jumperless instance for port detection: %w", err)
		}

		if j == nil {
			return nil, ErrNoJumperlessDevice
		}

		p.config.RealPort = j.GetPort()
		version := j.GetVersion()

		p.logger.Printf("Detected Jumperless port: %s (version: %s)", p.config.RealPort, version)
	}

	realPort, err := serial.Open(p.config.RealPort, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to open real serial port %s: %w", p.config.RealPort, err)
	}

	defer func() {
		if err := realPort.Close(); err != nil {
			p.logger.Printf("Warning: failed to close real serial port: %v", err)
		} else {
			p.logger.Printf("Closed real serial port: %s", p.config.RealPort)
		}
	}()

	if err := realPort.ResetInputBuffer(); err != nil {
		return nil, fmt.Errorf("failed to reset input buffer on real port %s: %w", p.config.RealPort, err)
	}
	if err := realPort.ResetOutputBuffer(); err != nil {
		return nil, fmt.Errorf("failed to reset output buffer on real port %s: %w", p.config.RealPort, err)
	}

	p.realPort = realPort
	p.logger.Printf("Connected to real serial port: %s", p.config.RealPort)

	wg := sync.WaitGroup{}

	// Start recorder and proxy goroutines
	recorderctx, cancelRecorder := context.WithCancelCause(ctx)
	wg.Go(func() { p.recorder.Run(recorderctx) })

	v2rctx, cancelV2R := context.WithCancelCause(ctx)
	wg.Go(func() { p.proxyVirtualToReal(v2rctx) })

	r2vctx, cancelR2V := context.WithCancelCause(ctx)
	wg.Go(func() { p.proxyRealToVirtual(r2vctx) })

	p.logger.Printf("Proxy started. Virtual serial port: %s", p.GetVirtualPortName())
	p.logger.Printf("Press Ctrl+C to stop")

	// Wait for context cancellation
	<-ctx.Done()
	p.logger.Printf("Context done, shutting down proxy")

	// Cancel all goroutines
	cancelV2R(nil)

	// Give some time for an active read/write to finish
	time.Sleep(100 * time.Millisecond)

	// Force close the pseudo TTY to unblock any active reads
	if err := p.pseudoTTY.Close(); err != nil {
		p.logger.Printf("Warning: failed to close pseudo TTY: %v", err)
	} else {
		p.logger.Printf("Closed pseudo TTY: %s", p.pseudoTTY.Name())
	}

	cancelR2V(nil)

	// Give some time for an active read/write to finish
	time.Sleep(100 * time.Millisecond)

	// Force close the real port to unblock any active reads
	if err := p.realPort.Close(); err != nil {
		p.logger.Printf("Warning: failed to close real serial port: %v", err)
	} else {
		p.logger.Printf("Closed real serial port: %s", p.config.RealPort)
	}

	cancelRecorder(nil)

	// Wait for all goroutines to finish
	wg.Wait()

	recording := p.recorder.GetRecording()
	if len(recording) > 0 {
		p.logger.Printf("Recorded %d request/response pairs", len(recording))
	} else {
		p.logger.Printf("No requests/responses recorded")
	}

	return recording, nil
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
				p.recorder.RecordRequest(bytes.Clone(data))

				// Forward to real port
				if _, err := p.realPort.Write(bytes.Clone(data)); err != nil {
					p.logger.Printf("Error writing to real port: %v", err)
				}

				p.logger.Printf("Request: %q", data)

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

				p.recorder.RecordResponse(bytes.Clone(data))

				// Forward to virtual port
				if _, err := p.pseudoTTY.Write(bytes.Clone(data)); err != nil {
					p.logger.Printf("Error writing to virtual port: %v", err)
				}

				p.logger.Printf("Response: %q", data)
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
