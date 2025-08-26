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

const DefaultBaudRate = 115200
const DefaultBufferSize = 1024

// ProxyConfig represents the proxy configuration
type ProxyConfig struct {
	BaudRate       int    `json:"baudRate"       mapstructure:"baud-rate"       yaml:"baudRate"`
	BufferSize     int    `json:"bufferSize"     mapstructure:"buffer-size"     yaml:"bufferSize"`
	VirtualPort    string `json:"virtualPort"    mapstructure:"virtual-port"    yaml:"virtualPort"`
	RealPort       string `json:"realPort"       mapstructure:"real-port"       yaml:"realPort"`
	EmulatorConfig string `json:"emulatorConfig" mapstructure:"emulator-config" yaml:"emulatorConfig"`
}
