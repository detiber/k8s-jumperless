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
	"context"
	"log"
	"os"

	"github.com/detiber/k8s-jumperless/utils/proxy/proxy/config"
)

// Proxy represents a serial port proxy that records communication
type Proxy struct {
	config *config.ProxyConfig
	logger *log.Logger
}

// New creates a new proxy instance
func New(c *config.ProxyConfig, logger *log.Logger) (*Proxy, error) {
	if logger == nil {
		logger = log.New(os.Stdout, "[proxy] ", log.LstdFlags)
	}

	return &Proxy{
		config: c,
		logger: logger,
	}, nil
}

// Start starts the proxy
func (p *Proxy) Start(ctx context.Context) error {

	// Start proxy goroutines
	go p.proxyVirtualToReal(ctx)
	go p.proxyRealToVirtual(ctx)

	return nil
}

func (p *Proxy) tryCleanup() {
}

// Stop stops the proxy
func (p *Proxy) Stop() error {
	p.tryCleanup()

	return nil
}

// proxyVirtualToReal forwards data from virtual port to real port (requests)
func (p *Proxy) proxyVirtualToReal(ctx context.Context) {
}

// proxyRealToVirtual forwards data from real port to virtual port (responses)
func (p *Proxy) proxyRealToVirtual(ctx context.Context) {
}
