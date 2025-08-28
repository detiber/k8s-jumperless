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

import "time"

const DefaultBaudRate = 115200
const DefaultBufferSize = 1024

func NewDefaultConfig() *GeneratorConfig {
	return &GeneratorConfig{
		BaudRate:   DefaultBaudRate,
		BufferSize: DefaultBufferSize,
		Port:       "",
		Requests:   []Request{},
	}
}

// generatorConfig represents the generator configuration
type GeneratorConfig struct {
	BaudRate   int       `json:"baudRate"   mapstructure:"baud-rate"   yaml:"baudRate"`
	BufferSize int       `json:"bufferSize" mapstructure:"buffer-size" yaml:"bufferSize"`
	Port       string    `json:"port"       mapstructure:"port"        yaml:"port"`
	Requests   []Request `json:"requests"   mapstructure:"requests"    yaml:"requests"`
}

type Request struct {
	Data    string        `json:"data"    mapstructure:"data"    yaml:"data"`
	Timeout time.Duration `json:"timeout" mapstructure:"timeout" yaml:"timeout"`
}
