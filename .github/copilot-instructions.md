# k8s-jumperless
A Kubernetes operator built with Kubebuilder v4.7.1 that declaratively manages [Jumperless v5](https://jumperless-docs.readthedocs.io/) hardware. This operator provides Custom Resources for managing device configuration through Kubernetes APIs.

**ALWAYS reference these instructions first and fallback to search or bash commands only when you encounter unexpected information that does not match the info here.**

## Working Effectively

### Bootstrap and Build (Required Steps)
Run these commands in order to set up the development environment:

1. **Download dependencies**: `go mod tidy` -- takes 40 seconds
2. **Build the project**: `make build` -- takes 3 minutes. NEVER CANCEL. Set timeout to 5+ minutes.
   - Generates Go code with stringer
   - Generates CRDs and RBAC manifests with controller-gen  
   - Formats and vets Go code
   - Builds manager binary to `bin/manager`
3. **Run tests**: `make test` -- takes 40 seconds. NEVER CANCEL. Set timeout to 2+ minutes.
   - Downloads and sets up the latest release of envtest binaries for latest release of Kubernetes
   - Runs unit tests
4. **Run linter**: `make lint` -- takes 4 minutes. NEVER CANCEL. Set timeout to 6+ minutes.
   - Downloads golangci-lint v2.1.6
   - Runs extensive linting with 60+ enabled linters

### Development Workflow
- **Format code**: `make fmt` -- runs `go fmt ./...`
- **Vet code**: `make vet` -- runs `go vet ./...`  
- **Generate manifests**: `make manifests` -- regenerates CRDs and RBAC
- **Generate Go code**: `make gen-go` -- runs `go generate` for type definitions

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
1. **Build validation**: `make build` -- ensure clean build
2. **Test validation**: `make test` -- ensure all tests pass  
3. **Lint validation**: `make lint` -- ensure code style compliance
4. **Format validation**: `make fmt` && check no changes with `git diff`

### Manual Testing Scenarios
After making changes to the operator:

1. **Test CRD generation**: 
   - Run `make manifests`
   - Check `config/crd/bases/jumperless.detiber.us_jumperlesses.yaml` for expected changes
   - **Note**: kubectl validation requires active cluster connection

2. **Test sample resources**:
   - Examine `config/samples/jumperless_v5alpha1_jumperless.yaml`
   - **Note**: kubectl validation requires active cluster connection

3. **Test controller logic**:
   - Review controller tests in `internal/controller/`
   - Add new test cases for new functionality
   - Ensure test coverage remains above 25%

4. **Test generated code**:
   - Run `make gen-go` to regenerate string methods
   - Verify `api/v5alpha1/dacchannel_string.go` contains String() method
   - Check that DAC channel constants work: `DAC0`, `DAC1`, `TOP_RAIL`, `BOTTOM_RAIL`

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
- **Go module**: `go.mod` (Go 1.24+ required)

### Repository Structure Quick Reference
```
.
├── api/v5alpha1/           # API type definitions and CRDs
├── cmd/                    # Main application entry point
├── config/                 # Kubernetes manifests and configurations
│   ├── crd/               # Custom Resource Definitions  
│   ├── manager/           # Manager deployment config
│   ├── rbac/              # Role-based access control
│   └── samples/           # Example Jumperless resources
├── internal/controller/    # Controller implementation
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

### Development Tips
- Always run code generation (`make generate`, `make gen-go`, `make manifests`) after modifying API types
- The stringer tool auto-generates string methods for DACChannel enum
- Controller-gen automatically generates CRD schemas from Go struct tags
- Use `+kubebuilder:` comment annotations to customize CRD generation

### Common Error Scenarios
- **Build fails**: Run `go mod tidy` first, ensure Go 1.24+ installed
- **Tests fail**: Check envtest setup, may need different Kubernetes version
- **Lint fails**: Review `.golangci.yml` for enabled linters, use `make lint-fix` for auto-fixes
- **Manager won't start**: Ensure valid kubeconfig and cluster connectivity, try creating a kind cluster
- **E2E tests fail**: Kind cluster creation issues, requires working Docker runtime

### Performance Expectations
- **Initial setup**: ~6-7 minutes (mod download + build + test + lint)
- **Incremental builds**: 30-60 seconds after code changes
- **Test cycles**: 40 seconds for unit tests
- **Full CI validation**: ~4-5 minutes (build + test + lint)

**CRITICAL**: Always use long timeouts (5+ minutes for builds, 2+ minutes for tests, 6+ minutes for linting) and NEVER CANCEL long-running operations. Build and lint operations are expected to take several minutes.