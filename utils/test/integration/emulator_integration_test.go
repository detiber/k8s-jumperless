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

package integration

import (
	"context"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"go.bug.st/serial/enumerator"

	"github.com/detiber/k8s-jumperless/internal/controller/local"
	"github.com/detiber/k8s-jumperless/utils/jumperless-emulator/emulator"
)

func TestEmulatorWithLocalController(t *testing.T) {
	// Create a temporary config for testing
	config := emulator.DefaultConfig()
	config.Serial.Port = "/tmp/integration-test-jumperless"

	// Create logger for testing
	logger := log.New(os.Stdout, "[integration-test] ", log.LstdFlags)

	// Create emulator
	emu, err := emulator.New(config, logger)
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
	time.Sleep(200 * time.Millisecond)

	// Test that the local controller can find and communicate with the emulator
	t.Run("FindJumperlessPort", func(t *testing.T) {
		// Try to get all ports, but handle the case where enumeration fails
		ports, err := local.EnumerateSerialPorts()
		var testPorts []*enumerator.PortDetails

		if err != nil {
			// If enumeration fails, just use our test port directly
			t.Logf("Port enumeration failed (expected for virtual ports): %v", err)
		} else {
			// Filter to include our test port
			for _, port := range ports {
				if strings.Contains(port.Name, config.Serial.Port) || port.Name == config.Serial.Port {
					testPorts = append(testPorts, port)
				}
			}
		}

		// Add the emulator port manually since it might not show up in enumeration
		testPorts = append(testPorts, &enumerator.PortDetails{
			Name: config.Serial.Port,
		})

		// Try to find Jumperless port
		foundPort, version, err := local.FindJumperlessPort(ctx, testPorts)
		if err != nil {
			t.Fatalf("Failed to find Jumperless port: %v", err)
		}

		if foundPort == nil {
			t.Fatal("No Jumperless port found")
		}

		if !strings.Contains(version, "5.2.2.0") {
			t.Errorf("Expected version containing '5.2.2.0', got: %s", version)
		}

		t.Logf("Found Jumperless at port %s with version %s", foundPort.Name, version)
	})

	t.Run("GetConfig", func(t *testing.T) {
		configSections, err := local.GetConfig(ctx, config.Serial.Port)
		if err != nil {
			t.Fatalf("Failed to get config: %v", err)
		}

		if len(configSections) == 0 {
			t.Error("Expected some config sections")
		}

		// Look for expected sections
		foundSections := make(map[string]bool)
		for _, section := range configSections {
			foundSections[section.Name] = true
			t.Logf("Found config section: %s with %d entries", section.Name, len(section.Entries))
		}

		if !foundSections["config"] {
			t.Error("Expected 'config' section")
		}
		if !foundSections["hardware"] {
			t.Error("Expected 'hardware' section")
		}
		if !foundSections["dacs"] {
			t.Error("Expected 'dacs' section")
		}
	})

	t.Run("GetNets", func(t *testing.T) {
		nets, err := local.GetNets(ctx, config.Serial.Port)
		if err != nil {
			t.Fatalf("Failed to get nets: %v", err)
		}

		if len(nets) == 0 {
			t.Error("Expected some nets")
		}

		// Look for expected nets
		foundGND := false
		foundTopRail := false
		foundBottomRail := false

		for _, net := range nets {
			t.Logf("Found net: Index=%d, Name=%s, Voltage=%s, Nodes=%v",
				net.Index, net.Name, net.Voltage, net.Nodes)

			if strings.Contains(net.Name, "GND") {
				foundGND = true
			}
			if strings.Contains(net.Name, "Top Rail") {
				foundTopRail = true
			}
			if strings.Contains(net.Name, "Bottom Rail") {
				foundBottomRail = true
			}
		}

		if !foundGND {
			t.Error("Expected to find GND net")
		}
		if !foundTopRail {
			t.Error("Expected to find Top Rail net")
		}
		if !foundBottomRail {
			t.Error("Expected to find Bottom Rail net")
		}
	})

	t.Run("GetDAC", func(t *testing.T) {
		// Test DAC0
		voltage, err := local.GetDAC(ctx, config.Serial.Port, 0) // DAC0
		if err != nil {
			t.Fatalf("Failed to get DAC voltage: %v", err)
		}

		if !strings.Contains(voltage, "V") {
			t.Errorf("Expected voltage with 'V' suffix, got: %s", voltage)
		}

		t.Logf("DAC0 voltage: %s", voltage)

		// Test DAC1
		voltage1, err := local.GetDAC(ctx, config.Serial.Port, 1) // DAC1
		if err != nil {
			t.Fatalf("Failed to get DAC1 voltage: %v", err)
		}

		if !strings.Contains(voltage1, "V") {
			t.Errorf("Expected voltage with 'V' suffix, got: %s", voltage1)
		}

		t.Logf("DAC1 voltage: %s", voltage1)
	})
}
