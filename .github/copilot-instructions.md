# k8s-jumperless
A Kubernetes operator built with Kubebuilder v4.7.1 that declaratively manages [Jumperless v5](https://jumperless-docs.readthedocs.io/) hardware. This operator provides Custom Resources for managing device configuration through Kubernetes APIs.

The project includes comprehensive testing utilities:
- **k8s-jumperless manager**: The main Kubernetes controller
- **utils**: Testing and development utilities including:
  - **emulator**: Hardware emulator for testing without physical devices
  - **proxy**: Recording proxy for capturing real device interactions
  - **generator**: Runs scripted commands to generate test data (not a long-running utility), can be used with the proxy to generate emulator configs

**ALWAYS reference these instructions first and fallback to search or bash commands only when you encounter unexpected information that does not match the info here.**

## Working Effectively

### Bootstrap and Build (Required Steps)
Run these commands in order to set up the development environment:

1. **Download dependencies**: `go mod tidy` -- takes 40 seconds
   - For all modules: `make tidy-all` -- takes 2 minutes
2. **Build the project**: `make build-all` -- takes 3 minutes. NEVER CANCEL. Set timeout to 5+ minutes.
   - Generates Go code with stringer
   - Generates CRDs and RBAC manifests with controller-gen  
   - Formats and vets Go code
   - Builds manager binary to `bin/manager`
   - Builds jumperless-utils binary to `bin/jumperless-utils`
3. **Run tests**: `make test-all` -- takes 2 minutes. NEVER CANCEL. Set timeout to 4+ minutes.
   - Downloads and sets up the latest release of envtest binaries for latest release of Kubernetes
   - Runs unit tests for manager and utils modules
4. **Run linter**: `make lint-all` -- takes 8 minutes. NEVER CANCEL. Set timeout to 10+ minutes.
   - Downloads golangci-lint v2.4.0
   - Runs extensive linting with 60+ enabled linters across all modules

### Development Workflow
- **Format code**: `make fmt-all` -- runs `go fmt ./...` across all modules
- **Vet code**: `make vet-all` -- runs `go vet ./...` across all modules
- **Tidy modules**: `make tidy-all` -- runs `go mod tidy` across all modules  
- **Generate manifests**: `make manifests` -- regenerates CRDs and RBAC
- **Generate Go code**: `make gen-go` -- runs `go generate` for type definitions

### Utility Development
- **Build utilities**: `make build-utils` -- builds utility binary
- **Test utilities**: `make test-utils` -- runs utility tests
- **Lint utilities**: `make lint-utils` -- lints utility code

### Running the Operator
- **Install manifests**: `make install` -- installs manifests to Kubernetes cluster
  - **REQUIRES**: Active Kubernetes cluster connection via kubeconfig
  - **FAILS**: If no cluster available with error about connection refused
- **Local development**: `make run` -- starts manager locally
  - **REQUIRES**: Active Kubernetes cluster connection via kubeconfig
  - **FAILS**: If no cluster available with error about KUBERNETES_SERVICE_HOST
- **Binary execution**: `./bin/manager --help` -- shows available flags
  - Key flags: `--kubeconfig`, `--metrics-bind-address`, `--health-probe-bind-address`

## Validation

### Always Run Before Committing
1. **Module validation**: `make tidy-all` && check no changes with `git diff` (if changes exist, stage and commit them)
1. **Build validation**: `make build-all` -- ensure clean build for all binaries
2. **Test validation**: `make test-all` -- ensure all tests pass across all modules  
3. **Lint validation**: `make lint-all` -- ensure code style compliance across all modules
4. **Format validation**: `make fmt-all` && check no changes with `git diff` (if changes exist, stage and commit them)

### Manual Testing Scenarios
After making changes to the operator:

1. **Test CRD generation**: 
   - Run `make manifests`
   - Check `config/crd/bases/jumperless.detiber.us_jumperlesses.yaml` for expected changes
   - **Note**: kubectl validation requires active cluster connection (use Kind if no cluster access)

2. **Test sample resources**:
   - Examine `config/samples/jumperless_v5alpha1_jumperless.yaml`
   - **Note**: kubectl validation requires active cluster connection (use Kind if no cluster access)

3. **Test controller logic**:
   - Run `make test` to test controller logic
   - Review controller tests in `internal/controller/`
   - Add new test cases for new functionality
   - Ensure test coverage remains above 25% (use `go test -cover ./...` to check coverage)

4. **Test generated code**:
   - Run `make gen-go` to regenerate string methods
   - Verify `api/v5alpha1/dacchannel_string.go` contains String() method
   - Check that DAC channel constants work: `DAC0`, `DAC1`, `TOP_RAIL`, `BOTTOM_RAIL`

5. **Test utilities**:
   - Run `make test-utils` to test all utilities
   - Test utils: `./bin/jumperless-utils --help` should show cobra-based CLI for the utils root command
      - Test emulator: `./bin/jumperless-utils emulator --help` should show configuration options for the emulator
      - Test proxy: `./bin/jumperless-utils proxy --help` should show configuration options for the proxy
      - Test generator: `./bin/jumperless-utils generator --help` should show configuration options for the generator

### End-to-End Testing (Optional)
E2E tests require Kind cluster setup but may fail in some environments:
- **Setup cluster**: `make setup-test-e2e` -- creates Kind cluster named 'k8s-jumperless-test-e2e'
- **Run E2E tests**: `make test-e2e` -- runs full operator deployment tests
- **Cleanup**: `make cleanup-test-e2e` -- removes test cluster
- **WARNING**: Docker runtime issues may prevent Kind cluster creation

## Common Tasks

### Key File Locations
- **API definitions**: `api/v5alpha1/`
- **Controller logic**: `internal/controller/`
- **Main entry point**: `cmd/main.go`
- **CRD manifests**: `config/crd/bases/`
- **Sample resources**: `config/samples/`
- **Build configuration**: `Makefile`
- **Go module**: `go.mod` (Go 1.25+ required)
- **Jumperless utility**: `utils/` (independent Go module)

### Repository Structure Quick Reference
```
.
├── api/v5alpha1/          # API type definitions and CRDs
├── cmd/                   # Main application entry point
├── config/                # Kubernetes manifests and configurations
│   ├── crd/               # Custom Resource Definitions  
│   ├── manager/           # Manager deployment config
│   ├── rbac/              # Role-based access control
│   └── samples/           # Example Jumperless resources
├── internal/controller/   # Controller implementation
├── jumperless/            # Common code for interacting with Jumperless devices
├── utils/                 # Testing and development utilities (independent Go module)
│   ├── cmd/               # Utility main entry point and commands
│   ├── internal/emulator  # Emulator implementation
│   ├── internal/proxy     # Proxy implementation
│   └── internal/generator # Test data generation scripts
├── test/                  # E2E and utility test code
├── Makefile               # Build automation
└── PROJECT                # Kubebuilder project metadata
```

### Understanding the Jumperless CRD
The operator manages `Jumperless` resources with these key fields:
- `spec.host.hostname`: Target Jumperless device hostname
- `spec.dacSettings[]`: Array of DAC channel configurations
  - `channel`: One of DAC0, DAC1, TOP_RAIL, BOTTOM_RAIL
  - `voltage`: String with voltage value (e.g., "3.3V", range -8V to +8V)
  - `save`: Boolean to persist settings across power cycles

### Working with Testing Utilities

#### Jumperless Utils
A collection of utilities for testing and development, including an emulator and a proxy.
- **CLI**: Cobra/Viper-based with comprehensive configuration support

```bash
# Build and run jumperless-utils
make build-utils
./bin/jumperless-utils --config examples/jumperless-utils.yml --verbose
```

##### Generator (`jumperless-utils generator`)
Generator for scripting commands against real hardware to generate test data:

```bash
# Build jumperless-utils and run generator
make build-utils
./bin/jumperless-utils generator --config examples/jumperless-utils.yml --port /dev/ttyACM0 --verbose
```

##### Emulator (`jumperless-utils emulator`)
Emulator for simulating Jumperless hardware in testing/development:

```bash
# Build jumperless-utils and run emulator
make build-utils
./bin/jumperless-utils emulator --config examples/jumperless-utils.yml --virtual-port /tmp/jumperless-emulator --verbose
```

##### Proxy (`jumperless-utils proxy`)
Recording proxy for capturing real device interactions:
- **Transparent proxying**: Full serial configuration support
- **Recording**: YAML formats with timing capture
- **Config generation**: Automatic emulator config from recorded sessions

```bash
# Build jumperless-utils and run proxy
make build-utils
./bin/jumperless-utils proxy --config examples/jumperless-utils.yml --real-port /dev/ttyACM0 --virtual-port /tmp/jumperless-proxy --verbose
```

### Development Tips
- Always run code generation (`make generate`, `make gen-go`, `make manifests`) after modifying API types
- The stringer tool auto-generates string methods for DACChannel enum
- Controller-gen automatically generates CRD schemas from Go struct tags
- Use `+kubebuilder:` comment annotations to customize CRD generation
- Check `Makefile` and `go.mod` for current tool versions if build errors occur due to version drift
- Each utils subpackage has its own `go.mod` and can be built independently
- Use `make tidy-all` to clean up all Go modules after dependency changes
- The utils use cobra/viper for CLI and configuration management

### CI Integration
This development workflow mirrors the CI process. CI enforces build, test, and lint validation for all contributions across all modules (manager and utils).

### Common Error Scenarios
- **Build fails**: Run `go mod tidy` or `make tidy-all` first, ensure Go 1.25+ installed (check `go.mod` for current version requirements)
- **Tests fail**: Check envtest setup, may need different Kubernetes version
- **Lint fails**: Review `.golangci.yml` for enabled linters, use `make lint-fix-all` for auto-fixes across all modules
- **Manager won't start**: Ensure valid kubeconfig and cluster connectivity, try creating a kind cluster
- **E2E tests fail**: Kind cluster creation issues, requires working Docker runtime
- **Utils build fails**: Check `utils/go.mod` for module dependencies, run `make tidy-all`

### Performance Expectations
- **Initial setup**: ~10-12 minutes (mod download + build + test + lint across all modules)
- **Incremental builds**: 30-60 seconds for manager, 15-30 seconds for utilities after code changes
- **Test cycles**: 2 minutes for all tests (`make test-all`)
- **Full CI validation**: ~8-10 minutes (build + test + lint across all modules)

**CRITICAL**: Always use long timeouts (5+ minutes for builds, 4+ minutes for tests, 10+ minutes for linting) and NEVER CANCEL long-running operations. Build and lint operations are expected to take several minutes across all modules.