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
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/detiber/k8s-jumperless/utils/jumperless-emulator/emulator"
)

const (
	defaultConfigFile = "jumperless-emulator.yml"
	cfgConfig         = "config"
	cfgGenerateConfig = "generate-config"
	cfgVerbose        = "verbose"
	cfgSerialPort     = "serial.port"
	cfgSerialBaudRate = "serial.baud-rate"
	cfgSerialStopBits = "serial.stop-bits"
	cfgSerialParity   = "serial.parity"
)

func configBoolVar(flagSet *pflag.FlagSet, v *viper.Viper, key string, defaultValue bool, description string) {
	flagSet.Bool(key, defaultValue, description)
	_ = v.BindPFlag(key, flagSet.Lookup(key))
}

func configStringVar(flagSet *pflag.FlagSet, v *viper.Viper, key, defaultValue, description string) {
	flagSet.String(key, defaultValue, description)
	_ = v.BindPFlag(key, flagSet.Lookup(key))
}

func configIntVar(flagSet *pflag.FlagSet, v *viper.Viper, key string, defaultValue int, description string) {
	flagSet.Int(key, defaultValue, description)
	_ = v.BindPFlag(key, flagSet.Lookup(key))
}

func configFlags(cmd *cobra.Command, v *viper.Viper) {
	// General flags
	cmd.PersistentFlags().String(cfgConfig, "", "config file (default is "+defaultConfigFile+")")

	configBoolVar(
		cmd.PersistentFlags(), v, cfgVerbose, false, "enable verbose logging",
	)

	// Serial port flags
	configStringVar(
		cmd.PersistentFlags(),
		v,
		cfgSerialPort,
		"",
		"serial port path (e.g. /dev/ttyUSB0) (overrides config)",
	)

	configIntVar(
		cmd.PersistentFlags(),
		v,
		cfgSerialBaudRate,
		0,
		"baud rate (e.g. 9600) (overrides config)",
	)

	configIntVar(
		cmd.PersistentFlags(),
		v,
		cfgSerialStopBits,
		0,
		"stop bits: 1 or 2 (overrides config)",
	)

	configStringVar(
		cmd.PersistentFlags(),
		v,
		cfgSerialParity,
		"",
		"parity: none, odd, even, mark, space (overrides config)",
	)

	// Utility flags
	cmd.Flags().Bool(cfgGenerateConfig, false, "generate default config file and exit")
}

func main() {
	v := viper.New()

	// Environment variable support
	v.SetEnvPrefix("JUMPERLESS_EMULATOR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	rootCmd := &cobra.Command{
		Use:   "jumperless-emulator",
		Short: "Jumperless device emulator",
		Long: `A virtual Jumperless device that creates a pseudo-terminal (pty) based serial port 
and responds to commands based on configurable request/response mappings.`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			configFile, err := cmd.Flags().GetString(cfgConfig)
			if err != nil {
				return fmt.Errorf("failed to get config flag: %w", err)
			}

			if configFile != "" {
				v.SetConfigFile(configFile)
			} else {
				base := filepath.Base(defaultConfigFile)
				ext := filepath.Ext(base)

				v.AddConfigPath(filepath.Dir(defaultConfigFile))
				v.SetConfigName(strings.TrimSuffix(base, ext))               // Use file name without extension
				v.SetConfigType(strings.TrimPrefix(filepath.Ext(base), ".")) // Use file extension as config type
			}

			// If a config file is found, read it in, we can ignore errors if not found
			var viperNotFoundErr viper.ConfigFileNotFoundError
			if err := v.ReadInConfig(); err != nil && !errors.As(err, &viperNotFoundErr) {
				return fmt.Errorf("error reading config file: %w", err)
			}

			if v.GetBool(cfgVerbose) {
				cfgFile := v.ConfigFileUsed()
				if cfgFile == "" {
					defaultConfigValues := v.AllSettings()
					fmt.Fprintf(os.Stderr, "No config file specified, using the default config values: %+v\n", defaultConfigValues)
				} else {
					fmt.Fprintf(os.Stderr, "Using config file: %s\n", v.ConfigFileUsed())
				}
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile, err := cmd.Flags().GetString(cfgConfig)
			if err != nil {
				return fmt.Errorf("failed to get config flag: %w", err)
			}

			shouldGenerateConfig, err := cmd.Flags().GetBool(cfgGenerateConfig)
			if err != nil {
				return fmt.Errorf("failed to get generate-config flag: %w", err)
			}

			if shouldGenerateConfig {
				if err := generateConfig(v, configFile); err != nil {
					return fmt.Errorf("failed to generate config: %w", err)
				}
				return nil
			}

			return runEmulator(v)
		},
	}

	configFlags(rootCmd, v)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generateConfig(v *viper.Viper, configFile string) error {
	// Generate default config file
	if err := v.SafeWriteConfig(); err != nil {
		return fmt.Errorf("failed to generate config file: %w", err)
	}

	if configFile == "" {
		configFile = defaultConfigFile
	}

	fmt.Printf("Generated default config file: %s\n", configFile)
	return nil
}

func runEmulator(v *viper.Viper) error {
	// Setup logger
	logger := log.New(os.Stdout, "[jumperless-emulator] ", log.LstdFlags)

	if v.GetBool(cfgVerbose) {
		config := v.AllSettings()
		logger.Printf("Starting Jumperless emulator with config: %v", config)
	}

	config, err := configFromViper(v)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

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

// configFromViper
func configFromViper(v *viper.Viper) (*emulator.Config, error) {
	config := emulator.DefaultConfig()

	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return config, nil
}
