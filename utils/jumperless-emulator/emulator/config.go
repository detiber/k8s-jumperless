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
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config represents the emulator configuration
type Config struct {
	// Serial port configuration
	Serial SerialConfig `json:"serial" yaml:"serial"`

	// Jumperless device configuration
	Jumperless JumperlessConfig `json:"jumperless" yaml:"jumperless"`

	// Request/response mappings
	Mappings []RequestResponse `json:"mappings" yaml:"mappings"`
}

// SerialConfig defines serial port parameters
type SerialConfig struct {
	// Port name (e.g., "/dev/ttyS0", "/tmp/jumperless")
	Port string `json:"port" yaml:"port"`

	// Baud rate
	BaudRate int `json:"baudRate" yaml:"baudRate"`

	// Buffer size for reading/writing
	BufferSize int `json:"bufferSize" yaml:"bufferSize"`

	// Stop bits (1 or 2)
	StopBits int `json:"stopBits" yaml:"stopBits"`

	// Parity (none, odd, even, mark, space)
	Parity string `json:"parity" yaml:"parity"`
}

// JumperlessConfig represents the internal Jumperless device state
type JumperlessConfig struct {
	// Firmware version
	FirmwareVersion string `json:"firmwareVersion" yaml:"firmwareVersion"`

	// Hardware configuration
	Hardware HardwareConfig `json:"hardware" yaml:"hardware"`

	// DAC channels (4 channels: 0, 1, TOP_RAIL, BOTTOM_RAIL)
	DACChannels map[string]DACChannel `json:"dacChannels" yaml:"dacChannels"`

	// ADC channels (5 channels: 0-4)
	ADCChannels map[string]ADCChannel `json:"adcChannels" yaml:"adcChannels"`

	// INA sensors (2 sensors)
	INASensors map[string]INASensor `json:"inaSensors" yaml:"inaSensors"`

	// GPIO pins (10 pins)
	GPIOPins map[string]GPIOPin `json:"gpioPins" yaml:"gpioPins"`

	// Node connections (pairs of connected nodes)
	Connections []Connection `json:"connections" yaml:"connections"`

	// Node definitions
	Nodes map[string]Node `json:"nodes" yaml:"nodes"`
}

// HardwareConfig represents Jumperless hardware information
type HardwareConfig struct {
	Generation    int `json:"generation"    yaml:"generation"`
	Revision      int `json:"revision"      yaml:"revision"`
	ProbeRevision int `json:"probeRevision" yaml:"probeRevision"`
}

// DACChannel represents a DAC channel state
type DACChannel struct {
	Voltage float64 `json:"voltage" yaml:"voltage"` // -8.0 to +8.0V
}

// ADCChannel represents an ADC channel state
type ADCChannel struct {
	Voltage  float64 `json:"voltage"  yaml:"voltage"`  // 0.0-8.0V for channels 0-3, 0.0-5.0V for channel 4
	MaxValue float64 `json:"maxValue" yaml:"maxValue"` // Maximum allowed voltage
}

// INASensor represents an INA219 current/voltage sensor
type INASensor struct {
	Current    float64 `json:"current"    yaml:"current"`    // Amperes
	Voltage    float64 `json:"voltage"    yaml:"voltage"`    // Volts
	BusVoltage float64 `json:"busVoltage" yaml:"busVoltage"` // Volts
	Power      float64 `json:"power"      yaml:"power"`      // Watts
}

// GPIOPin represents a GPIO pin state
type GPIOPin struct {
	Value     int    `json:"value"     yaml:"value"`     // 0 or 1
	Direction string `json:"direction" yaml:"direction"` // "input" or "output"
	Pull      string `json:"pull"      yaml:"pull"`      // "none", "up", "down"
}

// Connection represents a connection between two nodes
type Connection struct {
	NodeA string `json:"nodeA" yaml:"nodeA"`
	NodeB string `json:"nodeB" yaml:"nodeB"`
}

// Node represents a Jumperless node
type Node struct {
	Number   int      `json:"number"   yaml:"number"`
	Constant string   `json:"constant" yaml:"constant"`
	Aliases  []string `json:"aliases"  yaml:"aliases"`
	Type     string   `json:"type"     yaml:"type"` // "gpio", "dac", "adc", "power", etc.
}

// RequestResponse defines a request pattern and its response(s)
type RequestResponse struct {
	// Request pattern (can be literal string or regex)
	Request string `json:"request" yaml:"request"`

	// Whether request is a regex pattern
	IsRegex bool `json:"isRegex" yaml:"isRegex"`

	// Single response (for backward compatibility)
	Response string `json:"response,omitempty" yaml:"response,omitempty"`

	// Multiple responses with ordering/randomization
	Responses []ResponseOption `json:"responses,omitempty" yaml:"responses,omitempty"`

	// Response configuration
	ResponseConfig ResponseConfig `json:"responseConfig" yaml:"responseConfig"`
}

// ResponseOption represents a single response option with optional weight
type ResponseOption struct {
	// Response content
	Response string `json:"response" yaml:"response"`

	// Weight for random selection (higher = more likely)
	Weight int `json:"weight,omitempty" yaml:"weight,omitempty"`

	// Order index for sequential responses (0-based)
	Order int `json:"order,omitempty" yaml:"order,omitempty"`
}

// ResponseConfig defines how responses should be delivered
type ResponseConfig struct {
	// Delay before sending response
	Delay time.Duration `json:"delay" yaml:"delay"`

	// Random jitter to add to delay (0 to JitterMax)
	JitterMax time.Duration `json:"jitterMax" yaml:"jitterMax"`

	// Whether to chunk the response
	Chunked bool `json:"chunked" yaml:"chunked"`

	// Size of each chunk (if chunked)
	ChunkSize int `json:"chunkSize" yaml:"chunkSize"`

	// Delay between chunks
	ChunkDelay time.Duration `json:"chunkDelay" yaml:"chunkDelay"`

	// Response selection mode: "sequential", "random", "weighted"
	SelectionMode string `json:"selectionMode,omitempty" yaml:"selectionMode,omitempty"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Serial: SerialConfig{
			Port:       "/tmp/jumperless",
			BaudRate:   115200,
			BufferSize: 1024,
			StopBits:   1,
			Parity:     "none",
		},
		Jumperless: JumperlessConfig{
			FirmwareVersion: "5.2.2.0",
			Hardware: HardwareConfig{
				Generation:    5,
				Revision:      5,
				ProbeRevision: 5,
			},
			DACChannels: map[string]DACChannel{
				"0":           {Voltage: 3.3},
				"1":           {Voltage: 0.0},
				"TOP_RAIL":    {Voltage: 3.5},
				"BOTTOM_RAIL": {Voltage: 3.5},
			},
			ADCChannels: map[string]ADCChannel{
				"0": {Voltage: 0.0, MaxValue: 8.0},
				"1": {Voltage: 0.0, MaxValue: 8.0},
				"2": {Voltage: 0.0, MaxValue: 8.0},
				"3": {Voltage: 0.0, MaxValue: 8.0},
				"4": {Voltage: 0.0, MaxValue: 5.0},
			},
			INASensors: map[string]INASensor{
				"0": {Current: 0.1, Voltage: 3.3, BusVoltage: 3.3, Power: 0.33},
				"1": {Current: 0.05, Voltage: 5.0, BusVoltage: 5.0, Power: 0.25},
			},
			GPIOPins: map[string]GPIOPin{
				"0": {Value: 0, Direction: "input", Pull: "none"},
				"1": {Value: 0, Direction: "input", Pull: "none"},
				"2": {Value: 0, Direction: "input", Pull: "none"},
				"3": {Value: 0, Direction: "input", Pull: "none"},
				"4": {Value: 0, Direction: "input", Pull: "none"},
				"5": {Value: 0, Direction: "input", Pull: "none"},
				"6": {Value: 0, Direction: "input", Pull: "none"},
				"7": {Value: 0, Direction: "input", Pull: "none"},
				"8": {Value: 0, Direction: "input", Pull: "none"},
				"9": {Value: 0, Direction: "input", Pull: "none"},
			},
			Connections: []Connection{},
			Nodes:       createDefaultNodes(),
		},
		Mappings: []RequestResponse{
			{
				Request:  "?",
				IsRegex:  false,
				Response: "Jumperless firmware version: 5.2.2.0\r\n",
				ResponseConfig: ResponseConfig{
					Delay:     10 * time.Millisecond,
					JitterMax: 5 * time.Millisecond,
				},
			},
			{
				Request: "~",
				IsRegex: false,
				Response: "\r\n\r\ncopy / edit / paste any of these lines\r\n" +
					"into the main menu to change a setting\r\n\r\n" +
					"Jumperless Config:\r\n\r\n\r\n" +
					"`[config] firmware_version = 5.2.2.0;\r\n\r\n" +
					"`[hardware] generation = 5;\r\n" +
					"`[hardware] revision = 5;\r\n" +
					"`[hardware] probe_revision = 5;\r\n\r\n" +
					"`[dacs] dac0_voltage = {{dac_voltage:0}};\r\n" +
					"`[dacs] dac1_voltage = {{dac_voltage:1}};\r\n" +
					"`[dacs] top_rail_voltage = {{dac_voltage:2}};\r\n" +
					"`[dacs] bottom_rail_voltage = {{dac_voltage:3}};\r\n\r\n",
				ResponseConfig: ResponseConfig{
					Delay:     15 * time.Millisecond,
					JitterMax: 5 * time.Millisecond,
				},
			},
			{
				Request:  `>dac_get\((\d+)\)`,
				IsRegex:  true,
				Response: "Python> >dac_get($1)\r\n{{dac_voltage:$1}}\r\n",
				ResponseConfig: ResponseConfig{
					Delay:     5 * time.Millisecond,
					JitterMax: 2 * time.Millisecond,
				},
			},
			{
				Request: ">print_nets()",
				IsRegex: false,
				Response: "Python> >print_nets()\r\n" +
					"Index\tName\t\tVoltage\t\tConstant\r\n" +
					"1\tGND\t\t 0 V         GND\r\n" +
					"2\tTop Rail\t 0.00 V      TOP_R\r\n" +
					"3\tBottom Rail\t 0.00 V      BOT_R\r\n" +
					"4\tDAC 0\t\t {{dac_value:0}} V      DAC_0\r\n" +
					"5\tDAC 1\t\t {{dac_value:1}} V      DAC_1\r\n" +
					"\r\n",
				ResponseConfig: ResponseConfig{
					Delay:     20 * time.Millisecond,
					JitterMax: 10 * time.Millisecond,
				},
			},
		},
	}
}

// createDefaultNodes creates the default Jumperless node definitions
func createDefaultNodes() map[string]Node {
	nodes := make(map[string]Node)

	// Power nodes
	nodes["GND"] = Node{Number: 1, Constant: "GND", Type: "power"}
	nodes["TOP_R"] = Node{Number: 2, Constant: "TOP_R", Aliases: []string{"TOP_RAIL"}, Type: "power"}
	nodes["BOT_R"] = Node{Number: 3, Constant: "BOT_R", Aliases: []string{"BOTTOM_RAIL"}, Type: "power"}

	// DAC nodes
	nodes["DAC_0"] = Node{Number: 4, Constant: "DAC_0", Type: "dac"}
	nodes["DAC_1"] = Node{Number: 5, Constant: "DAC_1", Type: "dac"}

	// GPIO nodes (simplified subset for demo)
	for i := range 10 {
		nodeKey := fmt.Sprintf("GPIO_%d", i)
		nodes[nodeKey] = Node{
			Number:   10 + i,
			Constant: nodeKey,
			Type:     "gpio",
		}
	}

	return nodes
}

// GetResponse returns a response string based on the configuration
func (rr *RequestResponse) GetResponse(requestCounter int) string {
	// If single response is defined, use it
	if rr.Response != "" {
		return rr.Response
	}

	// If no responses defined, return empty
	if len(rr.Responses) == 0 {
		return ""
	}

	// Single response option
	if len(rr.Responses) == 1 {
		return rr.Responses[0].Response
	}

	// Multiple responses - use selection mode
	switch rr.ResponseConfig.SelectionMode {
	case "sequential":
		return rr.getSequentialResponse(requestCounter)
	case "random":
		return rr.getRandomResponse()
	case "weighted":
		return rr.getWeightedResponse()
	default:
		// Default to sequential
		return rr.getSequentialResponse(requestCounter)
	}
}

// getSequentialResponse returns responses in order
func (rr *RequestResponse) getSequentialResponse(counter int) string {
	if len(rr.Responses) == 0 {
		return ""
	}
	index := counter % len(rr.Responses)
	return rr.Responses[index].Response
}

// getRandomResponse returns a random response
func (rr *RequestResponse) getRandomResponse() string {
	if len(rr.Responses) == 0 {
		return ""
	}
	index := rand.Intn(len(rr.Responses))
	return rr.Responses[index].Response
}

// getWeightedResponse returns a weighted random response
func (rr *RequestResponse) getWeightedResponse() string {
	if len(rr.Responses) == 0 {
		return ""
	}

	// Calculate total weight
	totalWeight := 0
	for _, resp := range rr.Responses {
		weight := resp.Weight
		if weight <= 0 {
			weight = 1 // Default weight
		}
		totalWeight += weight
	}

	if totalWeight == 0 {
		return rr.getRandomResponse()
	}

	// Select random point
	target := rand.Intn(totalWeight)
	current := 0

	for _, resp := range rr.Responses {
		weight := resp.Weight
		if weight <= 0 {
			weight = 1
		}
		current += weight
		if current > target {
			return resp.Response
		}
	}

	// Fallback to last response
	return rr.Responses[len(rr.Responses)-1].Response
}

// LoadConfig loads configuration from file or returns default config
func LoadConfig(configFile string) (*Config, error) {
	config := DefaultConfig()

	if configFile == "" {
		return config, nil
	}

	// Check if file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return config, nil
	}

	v := viper.New()
	v.SetConfigFile(configFile)

	// Determine config type from file extension
	ext := strings.ToLower(filepath.Ext(configFile))
	switch ext {
	case ".yaml", ".yml":
		v.SetConfigType("yaml")
	case ".json":
		v.SetConfigType("json")
	case ".toml":
		v.SetConfigType("toml")
	default:
		v.SetConfigType("yaml") // Default to YAML
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return config, nil
}

// SaveConfig saves configuration to file
func SaveConfig(config *Config, filename string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
