# Jumperless Emulator and Proxy

This directory contains the Jumperless hardware emulator and proxy/recording utilities for testing and development.

## Overview

The Jumperless emulator and proxy tools provide a way to test the k8s-jumperless operator without requiring physical Jumperless hardware. This is particularly useful for:

- Continuous Integration (CI) testing
- Development without hardware access
- Recording real device interactions for replay testing
- Generating test configurations

## Components

### 1. Jumperless Emulator (`cmd/jumperless-emulator`)

A virtual Jumperless device that:
- Creates a virtual serial port using pseudo-terminals (pty)
- Responds to Jumperless commands based on configurable request/response mappings
- Supports configurable delays, jitter, and response chunking
- Compatible with the existing `internal/controller/local` package

**Features:**
- **Virtual Serial Port**: Creates a virtual serial port that appears as a real device
- **Configurable Responses**: Define custom request/response pairs via YAML/JSON configuration
- **Realistic Delays**: Configurable delays and jitter to simulate real device behavior
- **Chunked Responses**: Option to send responses in chunks with delays between chunks
- **Regex Support**: Pattern matching for dynamic requests (e.g., DAC commands with parameters)

### 2. Jumperless Proxy (`cmd/jumperless-proxy`)

A recording proxy that:
- Acts as a bridge between a virtual serial port and a real Jumperless device
- Records all communication for analysis and replay
- Can generate emulator configuration files from recorded sessions
- Supports multiple output formats (YAML, JSON, plain text logs)

**Features:**
- **Transparent Proxy**: Forwards all communication between client and real device
- **Communication Recording**: Records all requests and responses with timestamps
- **Config Generation**: Automatically generates emulator configurations from recordings
- **Multiple Formats**: Save recordings as YAML, JSON, or human-readable logs
- **Response Timing**: Captures actual response times for realistic emulation

## Quick Start

### Building

```bash
# Build all binaries
make build-all

# Or build individually
make build-emulator
make build-proxy
```

### Using the Emulator

1. **Generate a default configuration:**
   ```bash
   ./bin/jumperless-emulator -generate-config examples/my-emulator-config.yaml
   ```

2. **Start the emulator:**
   ```bash
   ./bin/jumperless-emulator -config examples/my-emulator-config.yaml -verbose
   ```

3. **Test with the operator:**
   ```bash
   # The emulator creates a virtual serial port (default: /tmp/jumperless)
   # Configure your operator to use this port
   ./bin/manager --kubeconfig ~/.kube/config
   ```

### Using the Proxy for Recording

1. **Generate a default proxy configuration:**
   ```bash
   ./bin/jumperless-proxy -generate-config examples/my-proxy-config.yaml
   ```

2. **Start the proxy:**
   ```bash
   ./bin/jumperless-proxy \
     -real-port /dev/ttyUSB0 \
     -virtual-port /tmp/jumperless-proxy \
     -recording-file recordings/session1.yaml \
     -verbose
   ```

3. **Connect your application to the virtual port and interact with the device**

4. **Stop the proxy (Ctrl+C) to save the recording and optionally generate an emulator config:**
   ```bash
   ./bin/jumperless-proxy \
     -real-port /dev/ttyUSB0 \
     -virtual-port /tmp/jumperless-proxy \
     -recording-file recordings/session1.yaml \
     -generate-emulator-config examples/generated-emulator-config.yaml \
     -verbose
   ```

## Configuration

### Emulator Configuration

The emulator uses YAML or JSON configuration files to define:

```yaml
serial:
  port: /tmp/jumperless          # Virtual port path
  baudRate: 115200               # Baud rate
  bufferSize: 1024              # Buffer size

mappings:
  - request: "?"                 # Literal string match
    isRegex: false
    response: "Jumperless firmware version: 5.2.2.0\r\n"
    responseConfig:
      delay: 10ms               # Response delay
      jitterMax: 5ms            # Random jitter (0 to jitterMax)
      chunked: false            # Whether to chunk response
      chunkSize: 32             # Chunk size (if chunked)
      chunkDelay: 10ms          # Delay between chunks

  - request: '>dac_get\((\d+)\)' # Regex pattern match
    isRegex: true
    response: "Python> >dac_get($1)\r\n3.3V\r\n"  # $1 = captured group
    responseConfig:
      delay: 10ms
      jitterMax: 5ms
```

### Proxy Configuration

```yaml
virtualPort:
  port: /tmp/jumperless-proxy   # Virtual port for clients
  baudRate: 115200
  bufferSize: 1024

realPort:
  port: /dev/ttyUSB0           # Real device port
  baudRate: 115200
  bufferSize: 1024

recording:
  enabled: true
  outputFile: recording.yaml    # Output file
  outputFormat: yaml           # yaml, json, or log
  includeTimestamps: true
  bufferSize: 0                # 0 = unbuffered
```

## Docker Support

Both utilities can run in Docker containers:

### Building Docker Images

```bash
# Build emulator image
docker build -f Dockerfile.emulator -t jumperless-emulator .

# Build proxy image
docker build -f Dockerfile.proxy -t jumperless-proxy .
```

### Using Docker Compose

```bash
# Start services
docker-compose up

# View logs
docker-compose logs -f jumperless-emulator
docker-compose logs -f jumperless-proxy
```

### Docker Environment Variables

Both containers support the same command-line arguments. Mount configuration files as volumes:

```bash
docker run -v $(pwd)/examples:/config \
  -v /tmp:/tmp \
  jumperless-emulator -config /config/emulator-config.yaml -verbose
```

## Development Workflow

### 1. Record Real Device Interactions

```bash
# Start proxy with real device
./bin/jumperless-proxy \
  -real-port /dev/ttyUSB0 \
  -virtual-port /tmp/recording \
  -recording-file recordings/baseline.yaml

# Run your tests against /tmp/recording
# Stop proxy to save recording
```

### 2. Generate Emulator Config

```bash
# Generate emulator config from recording
./bin/jumperless-proxy \
  -recording-file recordings/baseline.yaml \
  -generate-emulator-config examples/test-config.yaml
```

### 3. Test with Emulator

```bash
# Start emulator with generated config
./bin/jumperless-emulator -config examples/test-config.yaml

# Run the same tests against /tmp/jumperless
```

### 4. Iterate and Refine

Edit the generated configuration to:
- Adjust response delays
- Add more realistic jitter
- Create variations for different test scenarios
- Add chunked responses for large data

## CI/CD Integration

### GitHub Actions Example

```yaml
- name: Setup Jumperless Emulator
  run: |
    ./bin/jumperless-emulator -config examples/ci-config.yaml &
    sleep 2  # Wait for emulator to start

- name: Run Tests
  run: make test

- name: Cleanup
  run: pkill jumperless-emulator || true
```

### Testing with Kind

The emulator works well with Kubernetes-in-Docker (Kind) for testing the full operator:

```bash
# Start Kind cluster
kind create cluster --name jumperless-test

# Start emulator
./bin/jumperless-emulator -config examples/emulator-config.yaml &

# Deploy operator
kubectl apply -f dist/install.yaml

# Create test resources
kubectl apply -f examples/jumperless-test.yaml
```

## Troubleshooting

### Permission Issues

If you get permission errors accessing serial ports:

```bash
# Add user to dialout group (logout/login required)
sudo usermod -a -G dialout $USER

# Or run with appropriate permissions
sudo ./bin/jumperless-proxy -real-port /dev/ttyUSB0 ...
```

### Port Conflicts

If virtual ports conflict:

```bash
# Use different port paths
./bin/jumperless-emulator -port /tmp/jumperless-test
./bin/jumperless-proxy -virtual-port /tmp/proxy-test
```

### Docker Volume Mounts

For Docker, ensure proper volume mounts for device access:

```bash
docker run --privileged -v /dev:/dev jumperless-proxy ...
```

## Examples

See the `examples/` directory for:
- `emulator-config.yaml` - Basic emulator configuration
- `proxy-config.yaml` - Basic proxy configuration
- `docker-compose.yaml` - Docker compose setup
- Various test configurations

## API Compatibility

Both utilities are designed to be fully compatible with the existing `internal/controller/local` package. They implement the same serial communication protocol as the real Jumperless device:

- Firmware version query (`?`)
- Configuration query (`~`)
- Python command execution (`>command`)
- DAC queries (`>dac_get(channel)`)
- Network listing (`>print_nets()`)

The response formats exactly match the real device output, ensuring seamless integration with existing code.