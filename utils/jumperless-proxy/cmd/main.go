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

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/detiber/k8s-jumperless/utils/jumperless-proxy/proxy"
)

func main() {
	var (
		configFile        = flag.String("config", "", "Configuration file path (YAML or JSON)")
		virtualPort       = flag.String("virtual-port", "", "Virtual port path (overrides config)")
		realPort          = flag.String("real-port", "", "Real port path (overrides config)")
		baudRate          = flag.Int("baud-rate", 0, "Baud rate (overrides config)")
		recordingFile     = flag.String("recording-file", "", "Recording output file (overrides config)")
		recordingFormat   = flag.String("recording-format", "", "Recording format: yaml, json, log (overrides config)")
		disableRecording  = flag.Bool("disable-recording", false, "Disable recording")
		verbose           = flag.Bool("verbose", false, "Enable verbose logging")
		genConfig         = flag.String("generate-config", "", "Generate default config file and exit")
		genEmulatorConfig = flag.String("generate-emulator-config", "", "Generate emulator config from recording and exit")
	)
	flag.Parse()

	logger := log.New(os.Stdout, "[jumperless-proxy] ", log.LstdFlags)
	if !*verbose {
		logger.SetOutput(os.Stderr)
	}

	// Generate config file if requested
	if *genConfig != "" {
		config := proxy.DefaultConfig()
		if err := proxy.SaveConfig(config, *genConfig); err != nil {
			logger.Fatalf("Failed to generate config file: %v", err)
		}
		fmt.Printf("Generated default config file: %s\n", *genConfig)
		return
	}

	// Load configuration
	config, err := proxy.LoadConfig(*configFile)
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Override config with command line flags
	if *virtualPort != "" {
		config.VirtualPort.Port = *virtualPort
	}
	if *realPort != "" {
		config.RealPort.Port = *realPort
	}
	if *baudRate != 0 {
		config.VirtualPort.BaudRate = *baudRate
		config.RealPort.BaudRate = *baudRate
	}
	if *recordingFile != "" {
		config.Recording.OutputFile = *recordingFile
	}
	if *recordingFormat != "" {
		config.Recording.OutputFormat = *recordingFormat
	}
	if *disableRecording {
		config.Recording.Enabled = false
	}

	logger.Printf("Starting Jumperless proxy with config:")
	logger.Printf("  Virtual port: %s (baud: %d)", config.VirtualPort.Port, config.VirtualPort.BaudRate)
	logger.Printf("  Real port: %s (baud: %d)", config.RealPort.Port, config.RealPort.BaudRate)
	logger.Printf("  Recording: %v (file: %s, format: %s)",
		config.Recording.Enabled, config.Recording.OutputFile, config.Recording.OutputFormat)

	// Create proxy
	p, err := proxy.New(config, logger)
	if err != nil {
		logger.Fatalf("Failed to create proxy: %v", err)
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Printf("Received signal %s, shutting down...", sig)

		// Generate emulator config if requested
		if *genEmulatorConfig != "" {
			logger.Printf("Generating emulator config: %s", *genEmulatorConfig)
			if err := p.SaveEmulatorConfig(*genEmulatorConfig); err != nil {
				logger.Printf("Error generating emulator config: %v", err)
			} else {
				logger.Printf("Emulator config generated: %s", *genEmulatorConfig)
			}
		}

		cancel()
	}()

	// Start proxy
	if err := p.Start(ctx); err != nil {
		logger.Fatalf("Failed to start proxy: %v", err)
	}

	logger.Printf("Proxy started. Virtual port: %s", p.GetVirtualPortName())
	logger.Printf("Connect your application to the virtual port and interact with the device")
	logger.Printf("Press Ctrl+C to stop and save recording")

	// Wait for shutdown signal
	<-ctx.Done()

	logger.Printf("Stopping proxy...")
	if err := p.Stop(); err != nil {
		logger.Printf("Error stopping proxy: %v", err)
	}

	logger.Printf("Proxy stopped")
}
