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

	"github.com/detiber/k8s-jumperless/utils/proxy/proxy"
	"github.com/detiber/k8s-jumperless/utils/proxy/proxy/config"
)

const (
	defaultConfigFile = "jumperless-proxy.yml"
	cfgConfig         = "config"
	cfgGenerateConfig = "generate-config"
	cfgVerbose        = "verbose"
	cfgShowConfig     = "show-config"
	cfgBaudRate       = "baud-rate"
	cfgBufferSize     = "buffer-size"
	cfgVirtualPort    = "virtual-port"
	cfgRealPort       = "real-port"
	cfgEmulatorConfig = "emulator-config"
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

	configIntVar(
		cmd.Flags(), v, cfgBaudRate, config.DefaultBaudRate, "baud rate for serial communication",
	)

	configIntVar(
		cmd.Flags(), v, cfgBufferSize, config.DefaultBufferSize, "buffer size for serial communication",
	)

	configStringVar(
		cmd.Flags(), v, cfgVirtualPort, "",
		"name of the virtual serial port to create (e.g., /tmp/jumperless-virtual)",
	)

	configStringVar(
		cmd.Flags(), v, cfgRealPort, "",
		"name of the real serial port connected to the Jumperless device (e.g., /dev/ttyACM0)",
	)

	configStringVar(
		cmd.Flags(), v, cfgEmulatorConfig, "",
		"path to save the generated emulator configuration file",
	)

	// Utility flags
	cmd.Flags().Bool(cfgGenerateConfig, false, "generate default config file and exit")
	cmd.Flags().Bool(cfgShowConfig, false, "show current configuration and exit")
}

func main() {
	v := viper.New()

	// Environment variable support
	v.SetEnvPrefix("JUMPERLESS_PROXY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	rootCmd := &cobra.Command{
		Use:   "proxy",
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
			if err := v.ReadInConfig(); err != nil && !errors.As(err, &viperNotFoundErr) && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("error reading config file: %w", err)
			}

			if v.GetBool(cfgVerbose) {
				cfgFile := v.ConfigFileUsed()
				if cfgFile == "" {
					defaultConfigValues := v.AllSettings()
					fmt.Fprintf(os.Stderr, "No config file specified, using the default config values: %+v\n", defaultConfigValues)
				} else {
					fmt.Fprintf(os.Stderr, "Using config file: %s\n", v.ConfigFileUsed())
					fmt.Fprintf(os.Stderr, "Config values: %+v\n", v.AllSettings())
				}
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile, err := cmd.Flags().GetString(cfgConfig)
			if err != nil {
				return fmt.Errorf("failed to get config flag: %w", err)
			}

			shouldShowConfig, err := cmd.Flags().GetBool(cfgShowConfig)
			if err != nil {
				return fmt.Errorf("failed to get show-config flag: %w", err)
			}

			shouldGenerateConfig, err := cmd.Flags().GetBool(cfgGenerateConfig)
			if err != nil {
				return fmt.Errorf("failed to get generate-config flag: %w", err)
			}

			switch {
			case shouldShowConfig:
				// Write current config to stdout
				if err := v.WriteConfigTo(os.Stdout); err != nil {
					return fmt.Errorf("failed to write current config: %w", err)
				}

				return nil
			case shouldGenerateConfig:
				// Generate default config file
				if configFile == "" {
					configFile = defaultConfigFile
					if err := v.SafeWriteConfig(); err != nil {
						return fmt.Errorf("failed to generate config file: %w", err)
					}
				} else {
					if err := v.WriteConfigAs(configFile); err != nil {
						return fmt.Errorf("failed to generate config file: %w", err)
					}
				}

				fmt.Printf("Generated default config file: %s\n", configFile)

				return nil
			default:
				return runProxy(v)
			}
		},
	}

	configFlags(rootCmd, v)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runProxy(v *viper.Viper) error {
	// Setup logger
	logger := log.New(os.Stdout, "[proxy] ", log.LstdFlags)
	if !v.GetBool("verbose") {
		logger.SetOutput(os.Stderr)
	}

	proxyConfig := new(config.ProxyConfig)

	if err := v.Unmarshal(proxyConfig); err != nil {
		return fmt.Errorf("failed to unmarshal current config: %w", err)
	}

	logger.Printf("Starting Jumperless proxy with config: %+v", proxyConfig)

	// Create proxy
	p, err := proxy.New(proxyConfig, logger)
	if err != nil {
		return fmt.Errorf("failed to create proxy: %w", err)
	}

	// Setup signal handling
	ctx, cancel := context.WithCancelCause(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Printf("Received signal %s, shutting down...", sig)

		cancel(nil)
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
