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
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"go.bug.st/serial"
)

func TestEmulator(t *testing.T) {
	// Create a temporary config for testing
	config := DefaultConfig()
	config.Serial.Port = "/tmp/test-jumperless"

	// Create logger for testing
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	// Create emulator
	emu, err := New(config, logger)
	if err != nil {
		t.Fatalf("Failed to create emulator: %v", err)
	}

	// Start emulator
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	if err := emu.Start(ctx); err != nil {
		t.Fatalf("Failed to start emulator: %v", err)
	}
	defer func() {
		if err := emu.Stop(); err != nil {
			t.Logf("Error stopping emulator: %v", err)
		}
	}()

	// Give emulator time to start
	time.Sleep(100 * time.Millisecond)

	// Test firmware version query
	t.Run("FirmwareVersionQuery", func(t *testing.T) {
		response := sendCommand(t, config.Serial.Port, "?")
		if !strings.Contains(response, "Jumperless firmware version:") {
			t.Errorf("Expected firmware version response, got: %q", response)
		}
	})

	// Test config query
	t.Run("ConfigQuery", func(t *testing.T) {
		response := sendCommand(t, config.Serial.Port, "~")
		if !strings.Contains(response, "[config]") {
			t.Errorf("Expected config response, got: %q", response)
		}
	})

	// Test DAC query
	t.Run("DACQuery", func(t *testing.T) {
		response := sendCommand(t, config.Serial.Port, ">dac_get(0)")
		if !strings.Contains(response, "3.30V") {
			t.Errorf("Expected DAC response with voltage, got: %q", response)
		}
	})

	// Test print_nets query
	t.Run("PrintNetsQuery", func(t *testing.T) {
		response := sendCommand(t, config.Serial.Port, ">print_nets()")
		if !strings.Contains(response, "Index") || !strings.Contains(response, "GND") {
			t.Errorf("Expected nets table response, got: %q", response)
		}
	})
}

func sendCommand(t *testing.T, portName, command string) string {
	t.Helper()
	mode := &serial.Mode{
		BaudRate: 115200,
	}

	port, err := serial.Open(portName, mode)
	if err != nil {
		t.Fatalf("Failed to open port %s: %v", portName, err)
	}
	defer func() {
		if err := port.Close(); err != nil {
			t.Logf("Error closing port: %v", err)
		}
	}()

	// Send command
	if _, err := port.Write([]byte(command)); err != nil {
		t.Fatalf("Failed to write command: %v", err)
	}

	if err := port.Drain(); err != nil {
		t.Fatalf("Failed to drain port: %v", err)
	}

	// Set read timeout
	if err := port.SetReadTimeout(2 * time.Second); err != nil {
		t.Fatalf("Failed to set read timeout: %v", err)
	}

	// Wait for response
	time.Sleep(100 * time.Millisecond)

	// Read response
	buffer := make([]byte, 1024)
	response := ""

	for {
		n, err := port.Read(buffer)
		if err != nil {
			if os.IsTimeout(err) {
				break // Expected timeout when no more data
			}
			t.Fatalf("Failed to read response: %v", err)
		}

		if n == 0 {
			break
		}

		response += string(buffer[:n])
	}

	return response
}

func TestConfigLoading(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		config := DefaultConfig()
		if config == nil {
			t.Fatal("DefaultConfig returned nil")
		}
		if config.Serial.BaudRate != 115200 {
			t.Errorf("Expected baud rate 115200, got %d", config.Serial.BaudRate)
		}
		if len(config.Mappings) == 0 {
			t.Error("Expected some default mappings")
		}
	})

	t.Run("ConfigValidation", func(t *testing.T) {
		config := &Config{
			Serial: SerialConfig{
				Port: "/tmp/test",
			},
			Mappings: []RequestResponse{
				{
					Request:  "test",
					Response: "response",
				},
			},
		}

		// Create emulator to test validation
		_, err := New(config, nil)
		if err != nil {
			t.Errorf("Failed to create emulator with valid config: %v", err)
		}
	})

	t.Run("RegexValidation", func(t *testing.T) {
		config := &Config{
			Serial: SerialConfig{
				Port:     "/tmp/test",
				BaudRate: 115200,
			},
			Mappings: []RequestResponse{
				{
					Request:  `>invalid[regex`,
					IsRegex:  true,
					Response: "response",
				},
			},
		}

		// This should fail due to invalid regex
		_, err := New(config, nil)
		if err == nil {
			t.Error("Expected error for invalid regex, got nil")
		}
	})
}
