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

	// Command-line flags
	cmd.Flags().Int(config.FlagBaudRate, config.DefaultBaudRate, "baud rate for the real serial port")
	_ = v.BindPFlag(config.ViperBaudRate, cmd.Flags().Lookup(config.FlagBaudRate))

	cmd.Flags().Int(config.FlagBufferSize, config.DefaultBufferSize, "buffer size for reading from the real serial port")
	_ = v.BindPFlag(config.ViperBufferSize, cmd.Flags().Lookup(config.FlagBufferSize))

	cmd.Flags().String(config.FlagPort, "",
		"real serial port to use (if not specified, will attempt to auto-detect)")
	_ = v.BindPFlag(config.ViperPort, cmd.Flags().Lookup(config.FlagPort))

	return cmd
}

func runGenerator(ctx context.Context, v *viper.Viper, logger *log.Logger) error {
	generatorConfig := config.NewFromViper(v)

	logger.Printf("Starting Jumperless generator with config: %+v", generatorConfig)

	// Create generator
	g, err := generator.New(generatorConfig, logger)
	if err != nil {
		return fmt.Errorf("failed to create generator: %w", err)
	}

	// Run generator
	if err := g.Run(ctx); err != nil {
		return fmt.Errorf("failed to run generator: %w", err)
	}

	logger.Printf("generator stopped")
	return nil
}
