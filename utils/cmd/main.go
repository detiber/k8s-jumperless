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
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Setup logger
	logger := log.New(os.Stdout, "", log.LstdFlags)

	c := newRootCommand(logger)

	// Setup signal handling
	ctx, cancel := context.WithCancelCause(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Printf("Received signal %s, shutting down...", sig)

		cancel(nil)
	}()

	if err := c.execute(ctx); err != nil {
		if errors.Is(err, ErrShowConfig) || errors.Is(err, ErrGenerateConfig) {
			os.Exit(0)
		}

		logger.Printf("Error: %v", err)
		os.Exit(1)
	}

	cancel(nil)
}
