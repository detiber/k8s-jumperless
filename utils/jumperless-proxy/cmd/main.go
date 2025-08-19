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

	"github.com/detiber/k8s-jumperless/utils/jumperless-proxy/proxy"
)

const (
	defaultConfigFile         = "jumperless-proxy.yml"
	cfgConfig                 = "config"
	cfgGenerateConfig         = "generate-config"
	cfgVerbose                = "verbose"
	cfgSerialPort             = "serial.port"
	cfgSerialVirtualPort      = "serial.virtual-port"
	cfgSerialBaudRate         = "serial.baud-rate"
	cfgSerialStopBits         = "serial.stop-bits"
	cfgSerialParity           = "serial.parity"
	cfgRecordingFile          = "recording.file"
	cfgRecordingFormat        = "recording.format"
	cfgDisableRecording       = "recording.disable"
	cfgGenerateEmulatorConfig = "generate-emulator-config"
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

	configStringVar(
		cmd.PersistentFlags(),
		v,
		cfgSerialVirtualPort,
		"",
		"virtual port path (overrides config)",
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

	// Recording flags
	configStringVar(
		cmd.PersistentFlags(),
		v,
		cfgRecordingFile,
		"",
		"recording output file (overrides config)",
	)

	configStringVar(
		cmd.PersistentFlags(),
		v,
		cfgRecordingFormat,
		"",
		"recording format: yaml, json, log (overrides config)",
	)

	configBoolVar(
		cmd.PersistentFlags(),
		v,
		cfgDisableRecording,
		false,
		"disable recording",
	)

	// Utility flags
	cmd.Flags().Bool(cfgGenerateConfig, false, "generate default config file and exit")
	cmd.Flags().Bool(cfgGenerateEmulatorConfig, false, "generate emulator config from recording and exit")
}

func main() {
	v := viper.New()

	// Environment variable support
	v.SetEnvPrefix("JUMPERLESS_PROXY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	rootCmd := &cobra.Command{
		Use:   "jumperless-proxy",
		Short: "Jumperless recording proxy",
		Long: `A recording proxy that sits between applications and real Jumperless hardware 
to capture communication patterns for emulator configuration generation.`,
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

			return runProxy(v, cmd)
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

func runProxy(v *viper.Viper, cmd *cobra.Command) error {
	// Setup logger
	logger := log.New(os.Stdout, "[jumperless-proxy] ", log.LstdFlags)
	if !v.GetBool("verbose") {
		logger.SetOutput(os.Stderr)
	}

	// Load configuration
	config, err := loadConfigWithOverrides(v)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger.Printf("Starting Jumperless proxy with config:")
	logger.Printf("  Virtual port: %s (baud: %d, stopBits: %d, parity: %s)",
		config.VirtualPort.Port, config.VirtualPort.BaudRate, config.VirtualPort.StopBits, config.VirtualPort.Parity)
	logger.Printf("  Real port: %s (baud: %d, stopBits: %d, parity: %s)",
		config.RealPort.Port, config.RealPort.BaudRate, config.RealPort.StopBits, config.RealPort.Parity)
	logger.Printf("  Recording: %v (file: %s, format: %s)",
		config.Recording.Enabled, config.Recording.OutputFile, config.Recording.OutputFormat)

	// Create proxy
	p, err := proxy.New(config, logger)
	if err != nil {
		return fmt.Errorf("failed to create proxy: %w", err)
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Printf("Received signal %s, shutting down...", sig)

		// Generate emulator config if requested
		genEmulatorConfig, _ := cmd.Flags().GetString("generate-emulator-config")
		if genEmulatorConfig != "" {
			logger.Printf("Generating emulator config: %s", genEmulatorConfig)
			if err := p.SaveEmulatorConfig(genEmulatorConfig); err != nil {
				logger.Printf("Error generating emulator config: %v", err)
			} else {
				logger.Printf("Emulator config generated: %s", genEmulatorConfig)
			}
		}

		cancel()
	}()

	// Start proxy
	if err := p.Start(ctx); err != nil {
		return fmt.Errorf("failed to start proxy: %w", err)
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
	return nil
}

func loadConfigWithOverrides(v *viper.Viper) (*proxy.Config, error) {
	// Start with default config
	config := proxy.DefaultConfig()

	// Load from file if available
	if v.ConfigFileUsed() != "" {
		var err error
		config, err = proxy.LoadConfig(v.ConfigFileUsed())
		if err != nil {
			return nil, fmt.Errorf("failed to load proxy config from file %s: %w", v.ConfigFileUsed(), err)
		}
	}

	// Apply command line overrides
	if v.IsSet(cfgSerialVirtualPort) {
		config.VirtualPort.Port = v.GetString(cfgSerialVirtualPort)
	}
	if v.IsSet(cfgSerialPort) {
		config.RealPort.Port = v.GetString(cfgSerialPort)
	}
	if v.IsSet(cfgSerialBaudRate) && v.GetInt(cfgSerialBaudRate) > 0 {
		config.VirtualPort.BaudRate = v.GetInt(cfgSerialBaudRate)
		config.RealPort.BaudRate = v.GetInt(cfgSerialBaudRate)
	}
	if v.IsSet(cfgSerialStopBits) && v.GetInt(cfgSerialStopBits) > 0 {
		config.VirtualPort.StopBits = v.GetInt(cfgSerialStopBits)
		config.RealPort.StopBits = v.GetInt(cfgSerialStopBits)
	}
	if v.IsSet(cfgSerialParity) {
		config.VirtualPort.Parity = v.GetString(cfgSerialParity)
		config.RealPort.Parity = v.GetString(cfgSerialParity)
	}
	if v.IsSet(cfgRecordingFile) {
		config.Recording.OutputFile = v.GetString(cfgRecordingFile)
	}
	if v.IsSet(cfgRecordingFormat) {
		config.Recording.OutputFormat = v.GetString(cfgRecordingFormat)
	}
	if v.IsSet(cfgDisableRecording) {
		// Note: disable-recording flag inverts the logic
		config.Recording.Enabled = !v.GetBool(cfgDisableRecording)
	}

	return config, nil
}
