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

	"github.com/detiber/k8s-jumperless/utils/jumperless-proxy/proxy"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "jumperless-proxy",
		Short: "Jumperless recording proxy",
		Long: `A recording proxy that sits between applications and real Jumperless hardware 
to capture communication patterns for emulator configuration generation.`,
		RunE: runProxy,
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./proxy.yaml)")
	rootCmd.PersistentFlags().Bool("verbose", false, "enable verbose logging")

	// Port configuration flags
	rootCmd.Flags().String("virtual-port", "", "virtual port path (overrides config)")
	rootCmd.Flags().String("real-port", "", "real port path (overrides config)")
	rootCmd.Flags().Int("baud-rate", 0, "baud rate for both ports (overrides config)")
	rootCmd.Flags().Int("stop-bits", 0, "stop bits: 1 or 2 (overrides config)")
	rootCmd.Flags().String("parity", "", "parity: none, odd, even, mark, space (overrides config)")

	// Recording flags
	rootCmd.Flags().String("recording-file", "", "recording output file (overrides config)")
	rootCmd.Flags().String("recording-format", "", "recording format: yaml, json, log (overrides config)")
	rootCmd.Flags().Bool("disable-recording", false, "disable recording")

	// Utility flags
	rootCmd.Flags().String("generate-config", "", "generate default config file and exit")
	rootCmd.Flags().String("generate-emulator-config", "", "generate emulator config from recording and exit")

	// Bind flags to viper
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("virtualPort.port", rootCmd.Flags().Lookup("virtual-port"))
	viper.BindPFlag("realPort.port", rootCmd.Flags().Lookup("real-port"))
	viper.BindPFlag("baudRate", rootCmd.Flags().Lookup("baud-rate"))
	viper.BindPFlag("stopBits", rootCmd.Flags().Lookup("stop-bits"))
	viper.BindPFlag("parity", rootCmd.Flags().Lookup("parity"))
	viper.BindPFlag("recording.outputFile", rootCmd.Flags().Lookup("recording-file"))
	viper.BindPFlag("recording.outputFormat", rootCmd.Flags().Lookup("recording-format"))
	viper.BindPFlag("recording.enabled", rootCmd.Flags().Lookup("disable-recording"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("proxy")
		viper.SetConfigType("yaml")
	}

	// Environment variable support
	viper.SetEnvPrefix("JUMPERLESS_PROXY")
	viper.AutomaticEnv()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		if viper.GetBool("verbose") {
			fmt.Fprintf(os.Stderr, "Using config file: %s\n", viper.ConfigFileUsed())
		}
	}
}

func runProxy(cmd *cobra.Command, args []string) error {
	// Handle generate-config flag
	genConfig, _ := cmd.Flags().GetString("generate-config")
	if genConfig != "" {
		config := proxy.DefaultConfig()
		if err := proxy.SaveConfig(config, genConfig); err != nil {
			return fmt.Errorf("failed to generate config file: %w", err)
		}
		fmt.Printf("Generated default config file: %s\n", genConfig)
		return nil
	}

	// Setup logger
	logger := log.New(os.Stdout, "[jumperless-proxy] ", log.LstdFlags)
	if !viper.GetBool("verbose") {
		logger.SetOutput(os.Stderr)
	}

	// Load configuration
	config, err := loadConfigWithOverrides()
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

func loadConfigWithOverrides() (*proxy.Config, error) {
	// Start with default config
	config := proxy.DefaultConfig()

	// Load from file if available
	if viper.ConfigFileUsed() != "" {
		var err error
		config, err = proxy.LoadConfig(viper.ConfigFileUsed())
		if err != nil {
			return nil, err
		}
	}

	// Apply command line overrides
	if viper.IsSet("virtualPort.port") {
		config.VirtualPort.Port = viper.GetString("virtualPort.port")
	}
	if viper.IsSet("realPort.port") {
		config.RealPort.Port = viper.GetString("realPort.port")
	}
	if viper.IsSet("baudRate") && viper.GetInt("baudRate") > 0 {
		config.VirtualPort.BaudRate = viper.GetInt("baudRate")
		config.RealPort.BaudRate = viper.GetInt("baudRate")
	}
	if viper.IsSet("stopBits") && viper.GetInt("stopBits") > 0 {
		config.VirtualPort.StopBits = viper.GetInt("stopBits")
		config.RealPort.StopBits = viper.GetInt("stopBits")
	}
	if viper.IsSet("parity") {
		config.VirtualPort.Parity = viper.GetString("parity")
		config.RealPort.Parity = viper.GetString("parity")
	}
	if viper.IsSet("recording.outputFile") {
		config.Recording.OutputFile = viper.GetString("recording.outputFile")
	}
	if viper.IsSet("recording.outputFormat") {
		config.Recording.OutputFormat = viper.GetString("recording.outputFormat")
	}
	if viper.IsSet("recording.enabled") {
		// Note: disable-recording flag inverts the logic
		config.Recording.Enabled = !viper.GetBool("recording.enabled")
	}

	return config, nil
}
