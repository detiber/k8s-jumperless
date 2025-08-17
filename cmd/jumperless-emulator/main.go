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

	"github.com/detiber/k8s-jumperless/pkg/emulator"
)

func main() {
	var (
		configFile = flag.String("config", "", "Configuration file path (YAML or JSON)")
		port       = flag.String("port", "", "Serial port path (overrides config)")
		baudRate   = flag.Int("baud-rate", 0, "Baud rate (overrides config)")
		verbose    = flag.Bool("verbose", false, "Enable verbose logging")
		genConfig  = flag.String("generate-config", "", "Generate default config file and exit")
	)
	flag.Parse()

	logger := log.New(os.Stdout, "[jumperless-emulator] ", log.LstdFlags)
	if !*verbose {
		logger.SetOutput(os.Stderr)
	}

	// Generate config file if requested
	if *genConfig != "" {
		config := emulator.DefaultConfig()
		if err := emulator.SaveConfig(config, *genConfig); err != nil {
			logger.Fatalf("Failed to generate config file: %v", err)
		}
		fmt.Printf("Generated default config file: %s\n", *genConfig)
		return
	}

	// Load configuration
	config, err := emulator.LoadConfig(*configFile)
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Override config with command line flags
	if *port != "" {
		config.Serial.Port = *port
	}
	if *baudRate != 0 {
		config.Serial.BaudRate = *baudRate
	}

	logger.Printf("Starting Jumperless emulator with config: port=%s, baud=%d",
		config.Serial.Port, config.Serial.BaudRate)

	// Create emulator
	emu, err := emulator.New(config, logger)
	if err != nil {
		logger.Fatalf("Failed to create emulator: %v", err)
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Printf("Received signal %s, shutting down...", sig)
		cancel()
	}()

	// Start emulator
	if err := emu.Start(ctx); err != nil {
		logger.Fatalf("Failed to start emulator: %v", err)
	}

	logger.Printf("Emulator started. Virtual serial port: %s", emu.GetPortName())
	logger.Printf("Press Ctrl+C to stop")

	// Wait for shutdown signal
	<-ctx.Done()

	logger.Printf("Stopping emulator...")
	if err := emu.Stop(); err != nil {
		logger.Printf("Error stopping emulator: %v", err)
	}

	logger.Printf("Emulator stopped")
}
