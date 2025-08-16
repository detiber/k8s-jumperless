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

package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/detiber/k8s-jumperless/utils/jumperless-emulator/emulator"
	"gopkg.in/yaml.v3"
)

// saveRecording saves the recorded data to a file
func (p *Proxy) saveRecording() error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(p.config.Recording.OutputFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	var data []byte
	var err error

	switch strings.ToLower(p.config.Recording.OutputFormat) {
	case "yaml", "yml":
		data, err = yaml.Marshal(p.recording)
		if err != nil {
			return fmt.Errorf("failed to marshal recording to YAML: %w", err)
		}
	case "json":
		data, err = json.MarshalIndent(p.recording, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal recording to JSON: %w", err)
		}
	case "log":
		data = []byte(p.formatAsLog())
	default:
		return fmt.Errorf("unsupported output format: %s (use yaml, json, or log)", p.config.Recording.OutputFormat)
	}

	if err := os.WriteFile(p.config.Recording.OutputFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write recording file %s: %w", p.config.Recording.OutputFile, err)
	}

	return nil
}

// formatAsLog formats the recording as a human-readable log
func (p *Proxy) formatAsLog() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Jumperless Proxy Recording\n"))
	sb.WriteString(fmt.Sprintf("# Start Time: %s\n", p.recording.StartTime.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("# End Time: %s\n", p.recording.EndTime.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("# Total Duration: %s\n", p.recording.EndTime.Sub(p.recording.StartTime)))
	sb.WriteString(fmt.Sprintf("# Total Entries: %d\n\n", len(p.recording.Entries)))

	for i, entry := range p.recording.Entries {
		timestamp := entry.Timestamp.Format("15:04:05.000")
		direction := strings.ToUpper(entry.Direction)

		duration := ""
		if entry.Duration > 0 {
			duration = fmt.Sprintf(" (took %v)", entry.Duration)
		}

		sb.WriteString(fmt.Sprintf("[%d] %s %s%s: %q\n", i+1, timestamp, direction, duration, entry.Data))
	}

	return sb.String()
}

// ConvertToEmulatorConfig converts a recording to an emulator configuration
func (p *Proxy) ConvertToEmulatorConfig() (*emulator.Config, error) {
	config := &emulator.Config{
		Serial: emulator.SerialConfig{
			Port:       "/tmp/jumperless",
			BaudRate:   115200,
			BufferSize: 1024,
		},
		Mappings: []emulator.RequestResponse{},
	}

	// Process entries to create request/response pairs
	pendingRequests := make(map[string]*emulator.RequestResponse)

	for _, entry := range p.recording.Entries {
		switch entry.Direction {
		case "request":
			// Clean up the request (trim whitespace, normalize)
			request := strings.TrimSpace(entry.Data)
			if request == "" {
				continue
			}

			// Create a new mapping
			mapping := &emulator.RequestResponse{
				Request:  request,
				IsRegex:  false,
				Response: "",
				ResponseConfig: emulator.ResponseConfig{
					Delay:     10 * time.Millisecond,
					JitterMax: 5 * time.Millisecond,
				},
			}

			// Check if this looks like a regex pattern
			if p.looksLikeRegex(request) {
				mapping.IsRegex = true
			}

			pendingRequests[request] = mapping

		case "response":
			// Find the most recent pending request
			var lastRequest string
			var lastMapping *emulator.RequestResponse

			for req, mapping := range pendingRequests {
				if mapping.Response == "" { // Still waiting for response
					lastRequest = req
					lastMapping = mapping
					break
				}
			}

			if lastMapping != nil {
				lastMapping.Response = entry.Data

				// Set response delay based on recorded duration
				if entry.Duration > 0 {
					lastMapping.ResponseConfig.Delay = entry.Duration
					lastMapping.ResponseConfig.JitterMax = entry.Duration / 10 // 10% jitter
				}

				// Add to config
				config.Mappings = append(config.Mappings, *lastMapping)
				delete(pendingRequests, lastRequest)
			}
		}
	}

	return config, nil
}

// looksLikeRegex checks if a string looks like it might be a regex pattern
func (p *Proxy) looksLikeRegex(s string) bool {
	// Simple heuristics to detect regex patterns
	regexIndicators := []string{
		`\(`, `\)`, `\d`, `\w`, `\s`, `\+`, `\*`, `\?`, `[`, `]`, `{`, `}`, `|`, `^`, `$`,
	}

	for _, indicator := range regexIndicators {
		if strings.Contains(s, indicator) {
			return true
		}
	}

	return false
}

// SaveEmulatorConfig saves the recording as an emulator configuration
func (p *Proxy) SaveEmulatorConfig(filename string) error {
	config, err := p.ConvertToEmulatorConfig()
	if err != nil {
		return fmt.Errorf("failed to convert recording to emulator config: %w", err)
	}

	return emulator.SaveConfig(config, filename)
}

// LoadConfig loads proxy configuration from a file
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

// SaveConfig saves proxy configuration to a file
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

// validateConfig validates and sets defaults for proxy configuration
func validateConfig(config *Config) error {
	// Set default virtual port config
	if config.VirtualPort.BaudRate == 0 {
		config.VirtualPort.BaudRate = 115200
	}
	if config.VirtualPort.BufferSize == 0 {
		config.VirtualPort.BufferSize = 1024
	}
	if config.VirtualPort.Port == "" {
		config.VirtualPort.Port = "/tmp/jumperless-proxy"
	}

	// Set default real port config
	if config.RealPort.BaudRate == 0 {
		config.RealPort.BaudRate = 115200
	}
	if config.RealPort.BufferSize == 0 {
		config.RealPort.BufferSize = 1024
	}
	if config.RealPort.Port == "" {
		config.RealPort.Port = "/dev/ttyUSB0"
	}

	// Set default recording config
	if config.Recording.OutputFormat == "" {
		config.Recording.OutputFormat = "yaml"
	}
	if config.Recording.OutputFile == "" {
		config.Recording.OutputFile = "jumperless-recording.yaml"
	}

	return nil
}
