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

package config

import "github.com/spf13/viper"

const (
	// Default values for the proxy configuration
	DefaultBaudRate   = 115200
	DefaultBufferSize = 1024

	// Flag names for command-line arguments
	FlagBaudRate    = "baud-rate"
	FlagBufferSize  = "buffer-size"
	FlagVirtualPort = "virtual-port"
	FlagRealPort    = "real-port"
	FlagOverwrite   = "overwrite"

	// Viper prefix and keys for configuration
	ViperPrefix      = "proxy"
	ViperBaudRate    = ViperPrefix + "." + FlagBaudRate
	ViperBufferSize  = ViperPrefix + "." + FlagBufferSize
	ViperVirtualPort = ViperPrefix + "." + FlagVirtualPort
	ViperRealPort    = ViperPrefix + "." + FlagRealPort
	ViperOverwrite   = ViperPrefix + "." + FlagOverwrite
)

// NewDefaultConfig returns a ProxyConfig with default values
func NewDefaultConfig() *ProxyConfig {
	return &ProxyConfig{
		BaudRate:    DefaultBaudRate,
		BufferSize:  DefaultBufferSize,
		VirtualPort: "",
		RealPort:    "",
		Overwrite:   false,
	}
}

// NewFromViper creates a ProxyConfig from a viper instance
func NewFromViper(v *viper.Viper) *ProxyConfig {
	cfg := NewDefaultConfig()

	if v.IsSet(ViperBaudRate) {
		cfg.BaudRate = v.GetInt(ViperBaudRate)
	}
	if v.IsSet(ViperBufferSize) {
		cfg.BufferSize = v.GetInt(ViperBufferSize)
	}
	if v.IsSet(ViperVirtualPort) {
		cfg.VirtualPort = v.GetString(ViperVirtualPort)
	}
	if v.IsSet(ViperRealPort) {
		cfg.RealPort = v.GetString(ViperRealPort)
	}

	if v.IsSet(ViperOverwrite) {
		cfg.Overwrite = v.GetBool(ViperOverwrite)
	}

	return cfg
}

// ProxyConfig represents the proxy configuration
type ProxyConfig struct {
	BaudRate    int    `json:"baudRate"    mapstructure:"baudRate"    yaml:"baudRate"`
	BufferSize  int    `json:"bufferSize"  mapstructure:"bufferSize"  yaml:"bufferSize"`
	VirtualPort string `json:"virtualPort" mapstructure:"virtualPort" yaml:"virtualPort"`
	RealPort    string `json:"realPort"    mapstructure:"realPort"    yaml:"realPort"`
	Overwrite   bool   `json:"overwrite"   mapstructure:"overwrite"   yaml:"overwrite"`
}
