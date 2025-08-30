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
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/detiber/k8s-jumperless/utils/cmd/emulator"
	"github.com/detiber/k8s-jumperless/utils/cmd/generator"
	"github.com/detiber/k8s-jumperless/utils/cmd/proxy"
)

const (
	defaultConfigFile = "jumperless-utils.yml"
	cfgConfig         = "config"
	cfgGenerateConfig = "generate-config"
	cfgVerbose        = "verbose"
	cfgShowConfig     = "show-config"
)

var ErrShowConfig = errors.New("show config requested")
var ErrGenerateConfig = errors.New("generate config requested")

type rootCommand struct {
	cmd    *cobra.Command
	v      *viper.Viper
	logger *log.Logger
}

func newRootCommand(logger *log.Logger) *rootCommand {
	v := viper.New()

	// Environment variable support
	v.SetEnvPrefix("JUMPERLESS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	rootLogger := log.New(logger.Writer(), "[jumperless-utils] ", logger.Flags())

	c := &rootCommand{
		v:      v,
		logger: rootLogger,
		cmd: &cobra.Command{
			Use:   "jumperless-utils",
			Short: "Jumperless utilities",
			Long:  `Various utilities for working with Jumperless devices`,
			PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
				configFile, err := cmd.Flags().GetString(cfgConfig)
				if err != nil {
					return fmt.Errorf("failed to get config flag: %w", err)
				}

				if err := loadConfig(v, configFile, rootLogger); err != nil {
					return fmt.Errorf("failed to load config: %w", err)
				}

				// Handle utility flags
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
					return showConfig(cmd, v)
				case shouldGenerateConfig:
					return generateConfig(cmd, v, configFile, rootLogger)
				default:
					return nil
				}
			},
			RunE: func(cmd *cobra.Command, _ []string) error {
				return cmd.Help()
			},
		},
	}

	// Command-line flags

	// General flags not mapped to config
	c.cmd.PersistentFlags().String(cfgConfig, "", "config file (default is "+defaultConfigFile+")")

	// General flags mapped to config
	c.cmd.PersistentFlags().Bool(cfgVerbose, false, "enable verbose logging")
	_ = v.BindPFlag(cfgVerbose, c.cmd.PersistentFlags().Lookup(cfgVerbose))

	// Utility flags not mapped to config
	c.cmd.PersistentFlags().Bool(cfgGenerateConfig, false, "generate default config file and exit")
	c.cmd.PersistentFlags().Bool(cfgShowConfig, false, "show current configuration and exit")

	// Add subcommands
	c.cmd.AddCommand(generator.NewGeneratorCommand(v, rootLogger))
	c.cmd.AddCommand(emulator.NewEmulatorCommand(v, rootLogger))
	c.cmd.AddCommand(proxy.NewProxyCommand(v, rootLogger, defaultConfigFile, cfgConfig))

	return c
}

func (c *rootCommand) execute(ctx context.Context) error {
	if err := c.cmd.ExecuteContext(ctx); err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}

	return nil
}

func loadConfig(v *viper.Viper, configFile string, logger *log.Logger) error {
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
			logger.Printf("No config file specified, using the default config values: %+v\n", defaultConfigValues)
		} else {
			logger.Printf("Using config file: %s\n", v.ConfigFileUsed())
			logger.Printf("Config values: %+v\n", v.AllSettings())
		}
	}

	return nil
}

func showConfig(cmd *cobra.Command, v *viper.Viper) error {
	// Write current config to stdout
	if err := v.WriteConfigTo(os.Stdout); err != nil {
		return fmt.Errorf("failed to write current config: %w", err)
	}

	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	return ErrShowConfig
}

func generateConfig(cmd *cobra.Command, v *viper.Viper, configFile string, logger *log.Logger) error {
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

	logger.Printf("Generated default config file: %s", configFile)

	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	return ErrShowConfig
}
