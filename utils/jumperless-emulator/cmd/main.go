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
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/detiber/k8s-jumperless/utils/jumperless-emulator/emulator"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "jumperless-emulator",
		Short: "Jumperless device emulator",
		Long: `A virtual Jumperless device that creates a pseudo-terminal (pty) based serial port 
and responds to commands based on configurable request/response mappings.`,
		RunE: runEmulator,
	}
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./emulator.yaml)")
	rootCmd.PersistentFlags().Bool("verbose", false, "enable verbose logging")

	// Serial port flags
	rootCmd.Flags().String("port", "", "serial port path (overrides config)")
	rootCmd.Flags().Int("baud-rate", 0, "baud rate (overrides config)")
	rootCmd.Flags().Int("stop-bits", 0, "stop bits: 1 or 2 (overrides config)")
	rootCmd.Flags().String("parity", "", "parity: none, odd, even, mark, space (overrides config)")

	// Utility flags
	rootCmd.Flags().String("generate-config", "", "generate default config file and exit")

	// Bind flags to viper
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("serial.port", rootCmd.Flags().Lookup("port"))
	_ = viper.BindPFlag("serial.baudRate", rootCmd.Flags().Lookup("baud-rate"))
	_ = viper.BindPFlag("serial.stopBits", rootCmd.Flags().Lookup("stop-bits"))
	_ = viper.BindPFlag("serial.parity", rootCmd.Flags().Lookup("parity"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("emulator")
		viper.SetConfigType("yaml")
	}

	// Environment variable support
	viper.SetEnvPrefix("JUMPERLESS_EMULATOR")
	viper.AutomaticEnv()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		if viper.GetBool("verbose") {
			fmt.Fprintf(os.Stderr, "Using config file: %s\n", viper.ConfigFileUsed())
		}
	}
}

func runEmulator(cmd *cobra.Command, args []string) error {
	// Handle generate-config flag
	genConfig, _ := cmd.Flags().GetString("generate-config")
	if genConfig != "" {
		config := emulator.DefaultConfig()
		if err := emulator.SaveConfig(config, genConfig); err != nil {
			return fmt.Errorf("failed to generate config file: %w", err)
		}
		fmt.Printf("Generated default config file: %s\n", genConfig)
		return nil
	}

	// Setup logger
	logger := log.New(os.Stdout, "[jumperless-emulator] ", log.LstdFlags)
	if !viper.GetBool("verbose") {
		logger.SetOutput(os.Stderr)
	}

	// Load configuration
	config, err := loadConfigWithOverrides()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger.Printf("Starting Jumperless emulator with config: port=%s, baud=%d, stopBits=%d, parity=%s",
		config.Serial.Port, config.Serial.BaudRate, config.Serial.StopBits, config.Serial.Parity)

	// Create emulator
	emu, err := emulator.New(config, logger)
	if err != nil {
		return fmt.Errorf("failed to create emulator: %w", err)
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
		return fmt.Errorf("failed to start emulator: %w", err)
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
	return nil
}

func loadConfigWithOverrides() (*emulator.Config, error) {
	// Start with default config
	config := emulator.DefaultConfig()

	// Load from file if available
	if viper.ConfigFileUsed() != "" {
		var err error
		config, err = emulator.LoadConfig(viper.ConfigFileUsed())
		if err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Apply command line overrides
	if viper.IsSet("serial.port") {
		config.Serial.Port = viper.GetString("serial.port")
	}
	if viper.IsSet("serial.baudRate") && viper.GetInt("serial.baudRate") > 0 {
		config.Serial.BaudRate = viper.GetInt("serial.baudRate")
	}
	if viper.IsSet("serial.stopBits") && viper.GetInt("serial.stopBits") > 0 {
		config.Serial.StopBits = viper.GetInt("serial.stopBits")
	}
	if viper.IsSet("serial.parity") {
		config.Serial.Parity = viper.GetString("serial.parity")
	}

	return config, nil
}
