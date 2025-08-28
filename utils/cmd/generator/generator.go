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

package generator

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/detiber/k8s-jumperless/utils/internal/generator"
	"github.com/detiber/k8s-jumperless/utils/internal/generator/config"
)

const (
	cfgPrefix     = "generator"
	cfgBaudRate   = "baud-rate"
	cfgBufferSize = "buffer-size"
	cfgPort       = "port"
)

func NewGeneratorCommand(v *viper.Viper, parentLogger *log.Logger) *cobra.Command {
	logger := log.New(parentLogger.Writer(), parentLogger.Prefix()+" [generator]", parentLogger.Flags())
	cmd := &cobra.Command{
		Use:   "generator",
		Short: "Jumperless generator",
		Long:  `A generator sends configured commands to a Jumperless device over a serial port`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			return runGenerator(ctx, v, logger)
		},
	}

	// Set default config
	defaultConfig := config.NewDefaultConfig()
	v.SetDefault(cfgPrefix, defaultConfig)

	// Command-line flags
	cmd.Flags().Int(cfgBaudRate, defaultConfig.BaudRate, "baud rate for the real serial port")
	_ = v.BindPFlag(cfgPrefix+"."+cfgBaudRate, cmd.Flags().Lookup(cfgBaudRate))

	cmd.Flags().Int(cfgBufferSize, defaultConfig.BufferSize, "buffer size for reading from the real serial port")
	_ = v.BindPFlag(cfgPrefix+"."+cfgBufferSize, cmd.Flags().Lookup(cfgBufferSize))

	cmd.Flags().String(cfgPort, defaultConfig.Port, "serial port to use (if not specified, will attempt to auto-detect)")
	_ = v.BindPFlag(cfgPrefix+"."+cfgPort, cmd.Flags().Lookup(cfgPort))

	return cmd
}

func runGenerator(ctx context.Context, v *viper.Viper, logger *log.Logger) error {
	generatorConfig := new(config.GeneratorConfig)

	if err := v.Unmarshal(generatorConfig); err != nil {
		return fmt.Errorf("failed to unmarshal current config: %w", err)
	}

	logger.Printf("Starting Jumperless generator with config: %+v", generatorConfig)

	// Create generator
	g, err := generator.New(generatorConfig, logger)
	if err != nil {
		return fmt.Errorf("failed to create generator: %w", err)
	}

	// Start generator
	if err := g.Run(ctx); err != nil {
		return fmt.Errorf("failed to start generator: %w", err)
	}

	logger.Printf("generator stopped")
	return nil
}
