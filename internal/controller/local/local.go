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

package local

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/ptr"

	jumperlessv5alpha1 "github.com/detiber/k8s-jumperless/api/v5alpha1"
	"github.com/detiber/k8s-jumperless/jumperless"
)

var ErrUnexpectedCommandOutput = errors.New("unexpected command output format")
var ErrParseNetLine = errors.New("unable to parse net line")
var ErrParseLineDuplicateIndex = errors.New("net index is not greater than previous index")

var namedColors = []string{ //nolint:gochecknoglobals
	"red",
	"orange",
	"amber",
	"yellow",
	"chartreuse",
	"green",
	"seafoam",
	"cyan",
	"blue",
	"royal blue",
	"indigo",
	"violet",
	"purple",
	"pink",
	"magenta",
	"white",
	"black",
	"grey",
}

// Example net lines:
// "Index\tName\t\tVoltage\t    Nodes\t
// \r1\t GND\t\t 0 V         GND,9
// 2\t Top Rail\t 0.00 V      TOP_R,55
// 3\t Bottom Rail\t 0.00 V      BOT_R
// 4\t DAC 0\t\t 3.33 V      DAC_0,BUF_IN
// 5\t DAC 1\t\t 0.00 V      DAC_1
// Index\tName\t\tColor\t    Nodes          ADC / GPIO
// 6\t Net 6\t\t red         UART_Rx,D1
// 7\t Net 7\t\t red         UART_Tx,D0
// 8\t Net 8\t\t pink        6,5
// 9\t Net 9\t\t indigo      A3,13
// 10\t Net 10\t\t blue        51,D10
// 11\t Net 11\t\t cyan        ADC_3,20  \t    \b-2.78 V
// 12\t Net 12\t\t \b\b* red    - f  GP_1,25   \t    input - floating
// 13\t Net 13\t\t \b\b* red    - h  GP_4,36   \t    output - high
func parseNets(netsOutput string) ([]jumperlessv5alpha1.Net, error) {
	errs := []error{}

	nets := slices.Collect(func(yield func(jumperlessv5alpha1.Net) bool) {
		hasColor := false

		currentIndex := int32(0)

		for line := range strings.SplitSeq(netsOutput, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				if strings.HasPrefix(trimmed, "Index") {
					if strings.Contains(trimmed, "Color") {
						hasColor = true
					} else {
						hasColor = false
					}
				} else {
					net, err := parseNetLine(trimmed, hasColor, currentIndex)
					if err != nil {
						if !errors.Is(err, ErrParseLineDuplicateIndex) {
							// Only append parse errors that are not due to duplicate index
							errs = append(errs, fmt.Errorf("unable to parse net line %q: %w", trimmed, err))
						}
						continue
					}

					if !yield(net) {
						return
					}
				}
			}
		}
	})

	return nets, kerrors.NewAggregate(errs)
}

func parseNetLine(netLine string, hasColor bool, currentIndex int32) (jumperlessv5alpha1.Net, error) {
	net := jumperlessv5alpha1.Net{}

	// start by splitting fields on tabs to get index, name, and rest
	fields := strings.SplitN(netLine, "\t", 3)

	if len(fields) < 3 {
		return jumperlessv5alpha1.Net{}, fmt.Errorf("expected at least 3 fields, got %d for line %s: %w", len(fields), netLine, ErrParseNetLine)
	}

	// index is the first field
	index, err := strconv.ParseInt(strings.TrimSpace(fields[0]), 10, 32)
	if err != nil {
		return jumperlessv5alpha1.Net{}, fmt.Errorf("unable to parse index (%s) from net line %s: %w", fields[0], netLine, err)
	}

	net.Index = int32(index)

	if net.Index <= currentIndex {
		// If the current index is not less than the previous index, return an error
		// this is to short circuit the case where we are seeing duplicated output lines
		return jumperlessv5alpha1.Net{}, fmt.Errorf("net index %d is not greater than previous index %d: %w", net.Index, currentIndex, ErrParseLineDuplicateIndex)
	}

	// name is the second field
	net.Name = strings.TrimSpace(fields[1])

	// rest is the remaining fields
	rest := strings.TrimSpace(fields[2])
	var nodesPart string

	// now parse the rest based on whether we have color, voltage, and GPIO
	if !hasColor {
		// for example:
		// "0 V         GND,9"
		// "0.00 V      TOP_R,55"
		// "0.00 V      BOT_R"
		// "3.33 V      DAC_0,BUF_IN"
		// "0.00 V      DAC_1"
		before, after, found := strings.Cut(rest, " V")
		if !found {
			return jumperlessv5alpha1.Net{}, fmt.Errorf("unable to find voltage in net line %s: %w", netLine, ErrParseNetLine)
		}

		net.Voltage = ptr.To(strings.TrimSpace(before) + "V") // ensure voltage is suffixed with "V"

		nodesPart = strings.TrimSpace(after)
	} else {
		// for example:
		// "red         UART_Rx,D1"
		// "red         UART_Tx,D0"
		// "pink        6,5"
		// "indigo      A3,13"
		// "blue        51,D10"
		// "royal blue  ADC_3,20  \t    \b-6.69 V"
		// "\b\b* red    - f  GP_1,25   \t    input - floating"
		// "\b\b* red    - h  GP_4,36   \t    output - high"

		// since there may still be \b and * characters before the color,
		// we need to trim those first
		for strings.HasPrefix(rest, "\b") {
			rest = strings.TrimPrefix(rest, "\b")
		}
		rest = strings.TrimSpace(strings.TrimPrefix(rest, "*"))

		for _, color := range namedColors {
			if strings.HasPrefix(rest, color) {
				net.Color = ptr.To(color)
				rest = strings.TrimSpace(strings.TrimPrefix(rest, color))
				break
			}
		}

		if net.Color == nil {
			return jumperlessv5alpha1.Net{}, fmt.Errorf("unable to find color in net line %s: %w", netLine, ErrParseNetLine)
		}

		// At this point rest should look something like:
		// "UART_Rx,D1"
		// "UART_Tx,D0"
		// "6,5"
		// "A3,13"
		// "51,D10"
		// "ADC_3,20  \t    \b-6.69 V"
		// "- f  GP_1,25   \t    input - floating"
		// "- h  GP_4,36   \t    output - high"

		// Check for content preceding the node list that we need to remove
		// such as "- f", or "- h"
		re := regexp.MustCompile(`^- [[:alpha:]]`)
		if match := re.FindString(rest); match != "" {
			// Remove the matched prefix
			rest = strings.TrimSpace(strings.TrimPrefix(rest, match))
		}

		// at this point rest should look something like:
		// "UART_Rx,D1"
		// "UART_Tx,D0"
		// "6,5"
		// "A3,13"
		// "51,D10"
		// "ADC_3,20  \t    \b-6.69 V"
		// "GP_1,25   \t    input - floating"
		// "GP_4,36   \t    output - high"
		before, after, found := strings.Cut(rest, "\t")
		nodesPart = strings.TrimSpace(before)

		if found {
			net.Data = ptr.To(strings.TrimSpace(after))
		}
	}

	net.Nodes = []string{}

	for node := range strings.SplitSeq(nodesPart, ",") {
		trimmed := strings.TrimSpace(node)
		if trimmed != "" {
			net.Nodes = append(net.Nodes, trimmed)
		}
	}

	return net, nil
}

func parseConfig(configOutput string) ([]jumperlessv5alpha1.JumperLessConfigSection, error) {
	// Example config output:
	// ~
	//
	// copy / edit / paste any of these lines
	// into the main menu to change a setting
	//
	// Jumperless Config:
	//
	//
	// `[config] firmware_version = 5.2.2.0;
	//
	// `[hardware] generation = 5;
	// `[hardware] revision = 5;
	// `[hardware] probe_revision = 5;
	//
	// `[dacs] top_rail = 3.50;
	// `[dacs] bottom_rail = 3.50;
	// ...
	// `[top_oled] font = jokerman;
	//
	// END

	errs := []error{}

	config := map[string]map[string]string{}

	for line := range strings.SplitSeq(configOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue // skip empty lines
		}

		if !strings.HasPrefix(trimmed, "`[") {
			continue // skip non-config lines
		}

		// The section name is between "`[" and "]"
		trimmed = strings.TrimPrefix(trimmed, "`[")

		section, entry, found := strings.Cut(trimmed, "]")
		if !found {
			errs = append(errs, fmt.Errorf("unable to parse config line %q: %w", line, ErrParseNetLine))
			continue
		}

		if _, ok := config[section]; !ok {
			config[section] = map[string]string{}
		}

		// Parse entry line
		key, value, found := strings.Cut(entry, "=")
		if !found {
			errs = append(errs, fmt.Errorf("unable to parse config entry line %q: %w", trimmed, ErrParseNetLine))
			continue
		}

		config[section][strings.TrimSpace(key)] = strings.TrimSuffix(strings.TrimSpace(value), ";")
	}

	jumperlessConfig := []jumperlessv5alpha1.JumperLessConfigSection{}

	for sectionName, entries := range config {
		section := jumperlessv5alpha1.JumperLessConfigSection{
			Name:    sectionName,
			Entries: []jumperlessv5alpha1.JumperlessConfigEntry{},
		}
		for key, value := range entries {
			section.Entries = append(section.Entries, jumperlessv5alpha1.JumperlessConfigEntry{
				Key:   key,
				Value: value,
			})
		}

		jumperlessConfig = append(jumperlessConfig, section)
	}

	return jumperlessConfig, kerrors.NewAggregate(errs)
}

func GetConfig(j *jumperless.Jumperless) ([]jumperlessv5alpha1.JumperLessConfigSection, error) {
	configOutput, err := j.ExecRawCommand("~", 500*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("unable to get current config: %w", err)
	}

	return parseConfig(configOutput)
}

func GetNets(j *jumperless.Jumperless) ([]jumperlessv5alpha1.Net, error) {
	netsOutput, err := j.ExecPythonCommand("print_nets()", 10*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("unable to print nets: %w", err)
	}

	return parseNets(netsOutput)
}

func GetDAC(j *jumperless.Jumperless, channel jumperlessv5alpha1.DACChannel) (string, error) {
	dacVoltage, err := j.ExecPythonCommand(fmt.Sprintf("dac_get(%d)", channel), 10*time.Millisecond)
	if err != nil {
		return "", fmt.Errorf("unable to get DAC voltage for channel %s: %w", channel, err)
	}

	result := strings.TrimSpace(dacVoltage)
	if !strings.HasSuffix(result, "V") {
		result += "V" // Ensure result is suffixed with "V"
	}

	return result, nil
}
