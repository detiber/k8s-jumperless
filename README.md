# k8s-jumperless

A Kubernetes controller to declaratively manage [Jumperless v5](https://jumperless-docs.readthedocs.io/)

## Description

**Jumperless v5** is a programmable electronic prototyping device built around an **RP2350B microcontroller** that replaces traditional breadboards with software-controlled connections, enabling automated circuit prototyping and testing through:

- **Programmable connections**: Create and modify fully analog electrical connections between nodes using crosspoint matrix switches - no physical jumper wires needed
- **Arduino Nano compatibility**: Full support for Arduino Nano form factor with UART passthrough and Arduino sketch flashing capabilities
- **Voltage control**: Set precise voltages (-8V to +8V) on DAC (Digital-to-Analog Converter) channels for power rails and signal generation
- **Comprehensive I/O**: On-board ADC, current sensing hardware, GPIO, and PWM support for complete circuit control
- **Scripting and automation**: Built-in MicroPython interpreter with REPL support for scripting, automation, and extending device functionality
- **Remote management**: Control the device via serial communication for automated testing and configuration
- **Configuration persistence**: Save and restore circuit configurations and device settings

This **Kubernetes controller** provides declarative management of Jumperless v5 devices, allowing you to:

- Define circuit configurations as Kubernetes resources
- Manage multiple Jumperless devices across your infrastructure  
- Set DAC voltages and network connections through YAML manifests
- Monitor device status and firmware versions
- Integrate electronic prototyping into your CI/CD pipelines

For more information about Jumperless v5 hardware, visit the [official documentation](https://jumperless-docs.readthedocs.io/).

## Features

- **Declarative Management**: Define Jumperless device configurations using Kubernetes Custom Resources
- **Hardware Abstraction**: Manage Jumperless devices through standard Kubernetes APIs
- **Multi-host Support**: Connect to Jumperless devices over SSH or locally
- **Status Reporting**: Real-time device status and configuration reporting

## Components

- **k8s-jumperless manager**: The main Kubernetes controller
- **Jumperless emulator**: Virtual device for testing ([docs](docs/emulator-proxy.md))
- **Jumperless proxy**: Recording proxy for generating test configurations ([docs](docs/emulator-proxy.md))

## Getting Started

### Prerequisites
- go version v1.25.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/k8s-jumperless:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/k8s-jumperless:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Development Tools

The project includes testing utilities in the `/utils/` directory, each as independent Go submodules:

- **`/utils/emulator/`** - Comprehensive hardware emulator with realistic device simulation
- **`/utils/proxy/`** - Recording proxy for capturing real device interactions  

### Building Tools

Build all binaries including the operator, emulator and proxy:

```sh
make build-all  # Builds manager + emulator + proxy
```

Or build components individually:

```sh
make build            # Build k8s-jumperless manager only
make build-emulator   # Build emulator
make build-proxy      # Build proxy  
```

### Testing with Emulator

The emulator provides hardware simulation:

```sh
# create a kind cluster
kind create cluster

# ensure the controller dependencies are built
make

# install the crds
make install

# build the emulator docker image
make docker-build-proxy

docker run --privileged -d --rm --name jumperless-emulator -v /dev:/dev -v ./examples:/examples jumperless-emulator:latest --virtual-port /examples/jumperless-port --config /examples/emulator-data.yml

cat <<EOF | kubectl apply -f -
apiVersion: jumperless.detiber.us/v5alpha1
kind: Jumperless
metadata:
  name: jumperless-emulated
spec:
  host:
    local:
      port: ./examples/jumperless-port
EOF

make run # wait for reconciliation to complete, then ctrl-c

docker stop jumperless-emulator
```

### Recording with Proxy

To generate an emulator config using the controller
```sh
# create a kind cluster
kind create cluster

# ensure the controller dependencies are built
make

# install the crds
make install

# build the proxy docker image
make docker-build-proxy

docker run --privileged -d --rm --name jumperless-proxy -v /dev:/dev -v ./examples:/examples jumperless-proxy:latest --virtual-port /examples/jumperless-port --emulator-config /examples/emulator-data.yml

cat <<EOF | kubectl apply -f -
apiVersion: jumperless.detiber.us/v5alpha1
kind: Jumperless
metadata:
  name: jumperless-record
spec:
  host:
    local:
      port: ./examples/jumperless-port
EOF

make run # wait for reconciliation to complete, then ctrl-c

docker stop jumperless-proxy
```

### Docker Support

Each utility has its own Docker support with multi-stage builds:

```sh
# Build utility Docker images
make docker-build-all  # Builds all images (manager + emulator + proxy)

# Or build individually
make docker-build           # Build k8s-jumperless image only
make docker-build-emulator  # Build jumperless-emulator image
make docker-build-proxy     # Build jumperless-proxy image
```

See [docs/emulator-proxy.md](docs/emulator-proxy.md) for detailed documentation.

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/k8s-jumperless:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/k8s-jumperless/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

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

