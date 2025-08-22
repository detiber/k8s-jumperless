package jumperless

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
)

var ErrNoSerialPortFound = errors.New("no serial port found")
var ErrUnexpectedCommandOutput = errors.New("unexpected command output format")
var ErrParseNetLine = errors.New("unable to parse net line")

func isJumperlessPort(portName string) (bool, string, error) {
	result, err := execRawCommand(portName, "?", 10*time.Millisecond)
	if err != nil {
		return false, "", fmt.Errorf("failed to execute command: %w", err)
	}

	// Jumperless responds to "?" with a string containing "Jumperless firmware version:"
	expectedPrefix := "Jumperless firmware version:"
	if strings.Contains(result, expectedPrefix) {
		version := strings.TrimSpace(strings.Replace(result, expectedPrefix, "", 1))
		return true, version, nil
	}

	return false, "", nil
}

func execRawCommand(portName string, command string, waitForRead time.Duration) (string, error) {
	mode := &serial.Mode{
		BaudRate: 115200,
	}

	s, err := serial.Open(portName, mode)
	if err != nil {
		return "", fmt.Errorf("unable to open serial port %s: %w", portName, err)
	}
	defer s.Close() //nolint:errcheck

	// Reset input and output buffers to ensure clean state
	if err := s.ResetInputBuffer(); err != nil {
		return "", fmt.Errorf("unable to reset input buffer: %w", err)
	}

	if err := s.ResetOutputBuffer(); err != nil {
		return "", fmt.Errorf("unable to reset output buffer: %w", err)
	}

	if _, err := s.Write([]byte(command)); err != nil {
		return "", fmt.Errorf("unable to write to serial port %s: %w", portName, err)
	}

	if err := s.Drain(); err != nil {
		return "", fmt.Errorf("failed to drain serial port: %s: %w", portName, err)
	}

	if err := s.SetReadTimeout(time.Second); err != nil {
		return "", fmt.Errorf("unable to set read timeout on serial port %s: %w", portName, err)
	}

	time.Sleep(waitForRead)

	result := ""

	buff := make([]byte, 128)
	for {
		n, err := s.Read(buff)
		if err != nil {
			return "", fmt.Errorf("unable to read from serial port %s: %w", portName, err)
		}

		if n == 0 {
			break // No more data to read
		}

		result += string(buff[:n])
	}

	return result, nil
}

func EnumerateSerialPorts() ([]*enumerator.PortDetails, error) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, fmt.Errorf("unable to list serial ports: %w", err)
	}

	if len(ports) == 0 {
		return nil, ErrNoSerialPortFound
	}

	return ports, nil
}

func FindJumperlessPort(ctx context.Context, ports []*enumerator.PortDetails) (*enumerator.PortDetails, string, error) {
	errs := []error{}

	for _, port := range ports {
		// Check if this is a Jumperless port
		isJumperless, version, err := isJumperlessPort(port.Name)
		if err != nil {
			errs = append(errs, fmt.Errorf("unable to determine if port is Jumperless %w", err))
			continue
		}

		if isJumperless {
			return port, version, nil
		}
	}

	if len(errs) > 0 {
		return nil, "", kerrors.NewAggregate(errs)
	}

	return nil, "", nil
}
