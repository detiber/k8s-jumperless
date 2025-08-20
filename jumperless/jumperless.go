package jumperless

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"go.bug.st/serial/enumerator"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
)

var ErrNoSerialPortFound = errors.New("no serial port found")
var ErrNoJumperlessFound = errors.New("no Jumperless device found")
var ErrUnexpectedCommandOutput = errors.New("unexpected command output format")

type Jumperless struct {
	port *JumperlessPort
}

func NewJumperless(ctx context.Context, portName string, baudRate int) (*Jumperless, error) {
	// If a port name is provided, verify that it's a jumperless device
	if portName != "" {
		port, err := NewJumperlessPort(portName, baudRate)
		if err != nil {
			return nil, err
		}

		if port == nil {
			return nil, ErrNoJumperlessFound
		}

		return &Jumperless{port: port}, nil
	}

	// otherwise, enumerate ports and find a Jumperless device
	ports, err := enumerateSerialPorts()
	if err != nil {
		return nil, fmt.Errorf("unable to enumerate serial ports: %w", err)
	}

	detectedPort, err := findJumperlessPort(ports, baudRate)
	if err != nil {
		return nil, fmt.Errorf("unable to find Jumperless port: %w", err)
	}

	if detectedPort == nil {
		return nil, ErrNoJumperlessFound
	}

	return &Jumperless{port: detectedPort}, nil
}

func (j *Jumperless) GetVersion() string {
	if j == nil || j.port == nil {
		return ""
	}

	return j.port.version
}

func (j *Jumperless) GetPort() string {
	if j == nil || j.port == nil {
		return ""
	}

	return j.port.portName
}

func (j *Jumperless) OpenPort() error {
	if j == nil || j.port == nil {
		return ErrNilJumperlessPort
	}

	return j.port.Open()
}

func (j *Jumperless) ClosePort() error {
	if j == nil || j.port == nil {
		return ErrNilJumperlessPort
	}

	return j.port.Close()
}

func (j *Jumperless) ExecPythonCommand(command string, waitForRead time.Duration) (string, error) {
	result, err := j.ExecRawCommand(">"+command, waitForRead)
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %w", err)
	}

	result = ansi.Strip(result) // Remove ANSI escape codes

	// Split the output and strip the first and last lines
	// Example output:
	// Python> >dac_get(0)\r\n3.3V\r\n
	// The first line is the command prompt, the last line is empty.
	// The first line may also contain repeated substrings of the command and prompt
	// since Jumperless is streaming the prompt back using ANSI escape codes.
	resultLines := strings.Split(result, "\r\n")
	if len(resultLines) < 3 {
		return "", fmt.Errorf(
			"unexpected command output format: expected 3 lines, got %d %w",
			len(resultLines),
			ErrUnexpectedCommandOutput,
		)
	}

	filtered := slices.Collect(func(yield func(string) bool) {
		for _, line := range resultLines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "Python>") {
				if !yield(trimmed) {
					return
				}
			}
		}
	})

	switch len(filtered) {
	case 0:
		return "", fmt.Errorf(
			"unexpected command output format: no output lines after filtering %w",
			ErrUnexpectedCommandOutput,
		)
	case 1:
		return filtered[0], nil
	default:
		return strings.Join(filtered, "\n"), nil
	}
}

func (j *Jumperless) ExecRawCommand(command string, waitForRead time.Duration) (string, error) {
	if j == nil {
		return "", ErrNilJumperlessPort
	}
	if j.port == nil {
		return "", ErrUninitializedSerialPort
	}

	return j.port.execRawCommand(command, waitForRead)
}

func enumerateSerialPorts() ([]*enumerator.PortDetails, error) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, fmt.Errorf("unable to list serial ports: %w", err)
	}

	if len(ports) == 0 {
		return nil, ErrNoSerialPortFound
	}

	return ports, nil
}

func findJumperlessPort(ports []*enumerator.PortDetails, baudRate int) (*JumperlessPort, error) {
	errs := []error{}

	for _, port := range ports {
		port, err := NewJumperlessPort(port.Name, baudRate)
		if err != nil {
			errs = append(errs, fmt.Errorf("unable to determine if port is Jumperless %w", err))
			continue
		}

		if port == nil {
			continue
		}

		return port, nil
	}

	if len(errs) > 0 {
		return nil, kerrors.NewAggregate(errs)
	}

	return nil, ErrNoJumperlessFound
}
