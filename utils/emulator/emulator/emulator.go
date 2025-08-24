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
	"context"
	"log"
	"os"

	"github.com/detiber/k8s-jumperless/utils/emulator/emulator/config"
)

// Emulator represents a Jumperless device emulator
type Emulator struct {
	config *config.EmulatorConfig
	logger *log.Logger
}

// New creates a new emulator instance
func New(c *config.EmulatorConfig, logger *log.Logger) (*Emulator, error) {
	if logger == nil {
		logger = log.New(os.Stdout, "[emulator] ", log.LstdFlags)
	}

	return &Emulator{
		config: c,
		logger: logger,
	}, nil
}

// Start starts the emulator
func (e *Emulator) Start(ctx context.Context) error {

	return nil
}

// Stop stops the emulator
func (e *Emulator) Stop() error {

	return nil
}

// GetPortName returns the actual port name
func (e *Emulator) GetPortName() string {
	return ""
}
