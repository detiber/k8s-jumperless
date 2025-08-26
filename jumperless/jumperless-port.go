package jumperless

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
)

var ErrNilJumperlessPort = errors.New("called method on nil JumperlessPort")
var ErrUninitializedSerialPort = errors.New("serial port uninitialized")
var ErrPortAlreadyOpen = errors.New("serial port already open")
var ErrPortNotOpen = errors.New("serial port not open")

type JumperlessPort struct {
	portName string
	portLock sync.Mutex
	port     serial.Port
	mode     *serial.Mode
	version  string
}

func NewJumperlessPort(portName string, baudRate int) (*JumperlessPort, error) {
	if baudRate == 0 {
		baudRate = 115200
	}

	mode := &serial.Mode{
		BaudRate: baudRate,
	}

	j := &JumperlessPort{
		portName: portName,
		mode:     mode,
	}

	if err := j.Open(); err != nil {
		return nil, fmt.Errorf("unable to open serial port %s: %w", portName, err)
	}
	defer func() { _ = j.Close() }()

	ok, version, err := j.isJumperlessPort()
	if err != nil {
		return nil, fmt.Errorf("unable to check if port is Jumperless: %w", err)
	}

	if !ok {
		return nil, fmt.Errorf("port %s is not a Jumperless device: %w", portName, ErrNoJumperlessFound)
	}

	j.version = version

	return j, nil
}

func (p *JumperlessPort) Open() error {
	if p == nil {
		return ErrNilJumperlessPort
	}

	if p.port != nil {
		return ErrPortAlreadyOpen
	}

	p.portLock.Lock()
	defer p.portLock.Unlock()

	port, err := serial.Open(p.portName, p.mode)
	if err != nil {
		return fmt.Errorf("unable to open serial port %s: %w", p.portName, err)
	}

	p.port = port
	return nil
}

func (p *JumperlessPort) Close() error {
	if p == nil {
		return ErrNilJumperlessPort
	}

	if p.port == nil {
		return ErrPortNotOpen
	}

	p.portLock.Lock()
	defer p.portLock.Unlock()

	// If there's no closePort but the port is open, attempt to close it directly
	if err := p.port.Close(); err != nil {
		return fmt.Errorf("unable to close serial port %s: %w", p.portName, err)
	}

	// Clear the port reference
	p.port = nil

	return nil
}

func (p *JumperlessPort) isJumperlessPort() (bool, string, error) {
	result, err := p.execRawCommand("?", 10*time.Millisecond)
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

func (p *JumperlessPort) execRawCommand(command string, waitForRead time.Duration) (string, error) {
	if p == nil {
		return "", ErrNilJumperlessPort
	}

	if p.port == nil {
		return "", ErrUninitializedSerialPort
	}

	p.portLock.Lock()
	defer p.portLock.Unlock()

	// Reset input and output buffers to ensure clean state
	if err := p.port.ResetInputBuffer(); err != nil {
		return "", fmt.Errorf("unable to reset input buffer: %w", err)
	}

	if err := p.port.ResetOutputBuffer(); err != nil {
		return "", fmt.Errorf("unable to reset output buffer: %w", err)
	}

	if _, err := p.port.Write([]byte(command)); err != nil {
		return "", fmt.Errorf("unable to write to serial port %s: %w", p.portName, err)
	}

	if err := p.port.Drain(); err != nil {
		return "", fmt.Errorf("failed to drain serial port: %s: %w", p.portName, err)
	}

	if err := p.port.SetReadTimeout(time.Second); err != nil {
		return "", fmt.Errorf("unable to set read timeout on serial port %s: %w", p.portName, err)
	}

	time.Sleep(waitForRead)

	result := ""

	buff := make([]byte, 128)
	for {
		n, err := p.port.Read(buff)
		if err != nil {
			return "", fmt.Errorf("unable to read from serial port %s: %w", p.portName, err)
		}

		if n == 0 {
			break // No more data to read
		}

		result += string(buff[:n])
	}

	return result, nil
}
