# Jumperless Emulator and Proxy

## Overview

This project provides a comprehensive testing ecosystem for the k8s-jumperless operator through two sophisticated utilities with extensive hardware emulation capabilities. Both utilities are organized as independent Go subpackages under `/utils/`:

- **`/utils/jumperless-emulator/`** - Comprehensive hardware emulator with realistic device simulation
- **`/utils/jumperless-proxy/`** - Recording proxy for capturing real device interactions
- **`/utils/test/`** - Integration test suite as independent Go module

Each subpackage has its own `go.mod` file and can be built independently while maintaining cross-compatibility with the main operator.

## Architecture

### Jumperless Emulator (`utils/jumperless-emulator`)

A sophisticated virtual Jumperless device that creates a pseudo-terminal (pty) based serial port with comprehensive hardware emulation:

#### Hardware Simulation
- **4 DAC Channels** (0, 1, TOP_RAIL, BOTTOM_RAIL) with -8V to +8V range and real-time state tracking
- **5 ADC Channels** (0-3: 0-8V, channel 4: 0-5V) with configurable voltage readings
- **2 INA219 Sensors** for current/voltage/power monitoring with realistic values
- **10 GPIO Pins** with direction control, pull resistor configuration, and state management
- **Node System** implementing the complete Jumperless node topology with constants and aliases
- **Connection Management** supporting dynamic node connections with validation

#### Advanced Response System
- **Multiple Response Support** with sequential, random, and weighted selection modes
- **Dynamic Placeholders** that reflect current hardware state (e.g., `{{dac_voltage:0}}`, `{{gpio_value:5}}`)
- **Command Processing** for `set_dac()`, `gpio_set()`, `connect()`, `disconnect()`, and `clear()` operations
- **Realistic Timing** with configurable delays, jitter, and response chunking

#### Enhanced CLI with Cobra & Viper
```bash
jumperless-emulator --config emulator.yaml \
  --port /tmp/jumperless \
  --baud-rate 115200 \
  --stop-bits 1 \
  --parity none \
  --verbose
```

### Jumperless Proxy (`utils/jumperless-proxy`)

A transparent recording proxy that captures communication patterns between applications and real hardware:

#### Recording Capabilities
- **Transparent Proxying** with full serial configuration support (parity, stop bits)
- **Communication Recording** in YAML, JSON, or human-readable log formats
- **Timing Capture** for realistic emulator configuration generation
- **Automatic Config Generation** from recorded sessions

#### Enhanced CLI
```bash
jumperless-proxy \
  --real-port /dev/ttyUSB0 \
  --virtual-port /tmp/jumperless-proxy \
  --recording-file session.yaml \
  --stop-bits 1 \
  --parity none
```

## Quick Start

### Building

```bash
# Build all binaries (manager + utilities)
make build

# Or build utilities individually
make build-emulator
make build-proxy
```

### Using the Emulator

1. **Generate a configuration with full hardware emulation:**
   ```bash
   ./bin/jumperless-emulator --generate-config examples/my-emulator-config.yaml
   ```

2. **Start the emulator with enhanced CLI:**
   ```bash
   ./bin/jumperless-emulator \
     --config examples/my-emulator-config.yaml \
     --port /tmp/jumperless \
     --verbose
   ```

3. **Test with the operator:**
   ```bash
   # The emulator creates a virtual serial port (default: /tmp/jumperless)
   # Configure your operator to use this port
   ./bin/manager --kubeconfig ~/.kube/config
   ```

### Using the Proxy for Recording

1. **Start the proxy with enhanced CLI:**
   ```bash
   ./bin/jumperless-proxy \
     --real-port /dev/ttyUSB0 \
     --virtual-port /tmp/jumperless-proxy \
     --recording-file recordings/session1.yaml \
     --verbose
   ```

2. **Connect your application to the virtual port and interact with the device**

3. **Stop the proxy (Ctrl+C) to save the recording**

## Configuration

### Emulator Configuration

The emulator supports comprehensive configuration through YAML/JSON/TOML files, environment variables (`JUMPERLESS_EMULATOR_*`), and CLI flags:

```yaml
serial:
  port: /tmp/jumperless          # Virtual port path
  baudRate: 115200               # Baud rate
  stopBits: 1                    # Stop bits (1 or 2)
  parity: "none"                 # Parity (none, odd, even, mark, space)
  bufferSize: 1024               # Buffer size

# Hardware state configuration
jumperlessConfig:
  dacChannels:
    - channel: 0
      voltage: "3.3V"
      save: true
    - channel: 1  
      voltage: "0V"
      save: false

  adcChannels:
    - channel: 0
      voltage: 2.5
    - channel: 4
      voltage: 3.3
      
  gpio:
    - pin: 0
      direction: "output"
      value: true
      pullMode: "none"

# Response mappings with hardware integration
mappings:
  - request: "?"                 # Firmware version query
    isRegex: false
    responses:
      - response: "Jumperless firmware version: 5.2.2.0\r\n"
        weight: 1
    responseMode: "sequential"   # sequential, random, or weighted
    responseConfig:
      delay: 10ms
      jitterMax: 5ms

  - request: 'dac_get\((\d+)\)'  # DAC queries with hardware state
    isRegex: true
    responses:
      - response: "{{dac_voltage:$1}}\r\n"  # Reflects actual DAC state
        weight: 1
    responseConfig:
      delay: 15ms
      jitterMax: 5ms
```

### Proxy Configuration

```yaml
virtualPort:
  port: /tmp/jumperless-proxy   # Virtual port for clients
  baudRate: 115200
  stopBits: 1                   # Stop bits configuration
  parity: "none"                # Parity configuration
  bufferSize: 1024

realPort:
  port: /dev/ttyUSB0           # Real device port
  baudRate: 115200
  stopBits: 1
  parity: "none"
  bufferSize: 1024

recording:
  enabled: true
  outputFile: recording.yaml    # Output file
  outputFormat: yaml           # yaml, json, or log
  includeTimestamps: true
  bufferSize: 0                # 0 = unbuffered
```

## Docker Support

Both utilities support multi-stage Docker builds with distroless base images:

### Building Docker Images

```bash
# Build all Docker images
make docker-build

# Or build individually from subdirectories
cd utils/jumperless-emulator && docker build -t jumperless-emulator .
cd utils/jumperless-proxy && docker build -t jumperless-proxy .
```

### Running in Docker

```bash
# Run emulator in Docker
docker run -v $(pwd)/examples:/config \
  -v /tmp:/tmp \
  jumperless-emulator --config /config/emulator-config.yaml --verbose

# Run proxy in Docker (requires device access)
docker run --privileged -v /dev:/dev \
  -v $(pwd)/recordings:/recordings \
  jumperless-proxy --real-port /dev/ttyUSB0 \
    --recording-file /recordings/session.yaml
```

## Hardware Emulation Features

### DAC Channel Emulation
- **4 channels**: DAC0, DAC1, TOP_RAIL, BOTTOM_RAIL
- **Voltage range**: -8V to +8V
- **State persistence**: Tracks current voltage settings
- **Command integration**: Responds to `set_dac()` and `dac_get()` commands

### ADC Channel Emulation  
- **5 channels**: 0-3 (0-8V range), 4 (0-5V range)
- **Configurable readings**: Set baseline voltages in config
- **Dynamic responses**: `adc_get()` returns configured values

### GPIO Pin Emulation
- **10 pins**: Full direction, value, and pull resistor control
- **State tracking**: Maintains pin configuration across interactions
- **Command support**: `gpio_set()`, `gpio_get()`, `gpio_set_dir()`, etc.

### Node and Connection System
- **Complete node topology**: All Jumperless node constants and aliases
- **Dynamic connections**: Support for `connect()`, `disconnect()`, `clear()`
- **Connection validation**: Prevents invalid node connections
- **State queries**: `is_connected()` reflects actual connection state

## Development Workflow

### 1. Record Real Device Interactions

```bash
# Start proxy with real device
./bin/jumperless-proxy \
  --real-port /dev/ttyUSB0 \
  --virtual-port /tmp/recording \
  --recording-file recordings/baseline.yaml \
  --verbose

# Run your tests against /tmp/recording
# Stop proxy to save recording
```

### 2. Generate Emulator Config (Optional)

The proxy can auto-generate emulator configurations from recordings.

### 3. Test with Emulator

```bash
# Start emulator with generated or custom config
./bin/jumperless-emulator \
  --config examples/test-config.yaml \
  --verbose

# Run the same tests against /tmp/jumperless
```

### 4. Integration Testing

```bash
# Full integration test suite
make test-all

# Individual module testing
make test           # Main operator
make test-emulator  # Emulator tests
make test-proxy     # Proxy tests
make test-test      # Integration tests
```

## CI/CD Integration

### GitHub Actions Example

```yaml
- name: Setup Jumperless Emulator
  run: |
    ./bin/jumperless-emulator --config examples/ci-config.yaml &
    sleep 2  # Wait for emulator to start

- name: Run Tests
  run: make test-all

- name: Cleanup
  run: pkill jumperless-emulator || true
```

### Testing with Kind

The emulator works seamlessly with Kubernetes-in-Docker (Kind):

```bash
# Start Kind cluster
kind create cluster --name jumperless-test

# Start emulator
./bin/jumperless-emulator --config examples/emulator-config.yaml &

# Deploy operator
kubectl apply -f dist/install.yaml

# Create test resources
kubectl apply -f examples/jumperless-test.yaml
```

## Integration Examples

The emulator successfully integrates with the existing `internal/controller/local` package:

```go
// Emulator responds correctly to firmware queries
foundPort, version, err := local.FindJumperlessPort(ctx, testPorts)

// Hardware state accurately reflected in responses  
voltage, err := local.GetDAC(ctx, emulatorPort, 0)
// Returns current DAC0 voltage from emulated hardware state

// Dynamic configuration support
configSections, err := local.GetConfig(ctx, emulatorPort)
// Returns properly formatted configuration with current hardware settings
```

## Troubleshooting

### Permission Issues

```bash
# Add user to dialout group (logout/login required)
sudo usermod -a -G dialout $USER

# Or run with appropriate permissions
sudo ./bin/jumperless-proxy --real-port /dev/ttyUSB0 ...
```

### Port Conflicts

```bash
# Use different port paths
./bin/jumperless-emulator --port /tmp/jumperless-test
./bin/jumperless-proxy --virtual-port /tmp/proxy-test
```

### Build Issues

```bash
# Ensure dependencies are current
make tidy-all

# Build with verbose output
./bin/jumperless-emulator --verbose
./bin/jumperless-proxy --verbose
```

This comprehensive testing ecosystem enables full development and testing of the k8s-jumperless operator without requiring physical hardware, supporting both development workflows and automated CI/CD pipelines with realistic hardware simulation.
