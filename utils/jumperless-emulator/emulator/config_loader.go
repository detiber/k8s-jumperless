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

package emulator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadConfig loads configuration from a file
func LoadConfig(filename string) (*Config, error) {
	if filename == "" {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filename, err)
	}

	config := &Config{}

	// Determine file format based on extension
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format: %s (use .yaml, .yml, or .json)", ext)
	}

	// Validate and set defaults
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return config, nil
}

// SaveConfig saves configuration to a file
func SaveConfig(config *Config, filename string) error {
	var data []byte
	var err error

	// Determine file format based on extension
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".yaml", ".yml":
		data, err = yaml.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal config to YAML: %w", err)
		}
	case ".json":
		data, err = json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal config to JSON: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config file format: %s (use .yaml, .yml, or .json)", ext)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", filename, err)
	}

	return nil
}

// validateConfig validates and sets defaults for configuration
func validateConfig(config *Config) error {
	// Set default serial config
	if config.Serial.BaudRate == 0 {
		config.Serial.BaudRate = 115200
	}
	if config.Serial.BufferSize == 0 {
		config.Serial.BufferSize = 1024
	}
	if config.Serial.Port == "" {
		config.Serial.Port = "/tmp/jumperless"
	}
	if config.Serial.StopBits == 0 {
		config.Serial.StopBits = 1
	}
	if config.Serial.Parity == "" {
		config.Serial.Parity = "none"
	}

	// Validate mappings
	for i := range config.Mappings {
		mapping := &config.Mappings[i]

		// Set defaults for response config
		if mapping.ResponseConfig.ChunkSize == 0 && mapping.ResponseConfig.Chunked {
			mapping.ResponseConfig.ChunkSize = 32
		}

		// Validate regex patterns
		if mapping.IsRegex {
			// This will be validated during emulator creation
		}
	}

	return nil
}
