# ConfigMap ReplicaSet Operator

[![CI](https://github.com/matanbaruch/configmap-rs-operator/actions/workflows/ci.yml/badge.svg)](https://github.com/matanbaruch/configmap-rs-operator/actions/workflows/ci.yml)
[![E2E Tests](https://github.com/matanbaruch/configmap-rs-operator/actions/workflows/e2e.yml/badge.svg)](https://github.com/matanbaruch/configmap-rs-operator/actions/workflows/e2e.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/matanbaruch/configmap-rs-operator)](https://goreportcard.com/report/github.com/matanbaruch/configmap-rs-operator)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

A Kubernetes operator that automatically adds ownerReferences to ConfigMaps when they are mounted as volumes by ReplicaSets. This enables Kubernetes garbage collection to automatically clean up unused ConfigMaps when their associated ReplicaSets are deleted.

## Features

- **Automatic Owner Reference Management**: Monitors ReplicaSets and adds ownerReferences to ConfigMaps they mount as volumes
- **Namespace Filtering**: Support for regex-based namespace selection to control which namespaces the operator monitors
- **Dry-Run Mode**: Test the operator's behavior without making actual changes
- **Debug and Trace Logging**: Comprehensive logging options for troubleshooting
- **Kubernetes Garbage Collection**: Leverages built-in Kubernetes GC for automatic ConfigMap cleanup
- **Multi-Architecture Support**: Available for AMD64 and ARM64 architectures

## How It Works

1. The operator watches for ReplicaSet creation and updates
2. When a ReplicaSet is detected, it analyzes the pod template for ConfigMap volume mounts
3. For each mounted ConfigMap, it adds the ReplicaSet as an owner reference
4. When the ReplicaSet is deleted, Kubernetes garbage collection automatically removes the ConfigMap

## Installation

### Using Helm (Recommended)

Add the Helm repository:

```bash
helm repo add configmap-rs-operator https://matanbaruch.github.io/configmap-rs-operator/helm-charts
helm repo update
```

Install the operator:

```bash
helm install configmap-rs-operator configmap-rs-operator/configmap-rs-operator \
  --namespace configmap-rs-operator-system \
  --create-namespace
```

### Using kubectl

```bash
kubectl apply -f https://github.com/matanbaruch/configmap-rs-operator/releases/latest/download/install.yaml
```

### From Source

1. Clone the repository:

```bash
git clone https://github.com/matanbaruch/configmap-rs-operator.git
cd configmap-rs-operator
```

2. Deploy using make:

```bash
make deploy IMG=ghcr.io/matanbaruch/configmap-rs-operator:latest
```

## Configuration

The operator supports several configuration options:

### Command Line Flags

- `--namespace-regex`: Comma-separated list of regex patterns to match namespaces (default: all namespaces)
- `--dry-run`: Enable dry-run mode (only log what would be done)
- `--debug`: Enable debug logging
- `--trace`: Enable trace logging (more verbose than debug)
- `--leader-elect`: Enable leader election (default: false)

### Environment Variables

- `NAMESPACE_REGEX`: Same as `--namespace-regex` flag
- `DRY_RUN`: Set to "true" to enable dry-run mode
- `DEBUG`: Set to "true" to enable debug logging
- `TRACE`: Set to "true" to enable trace logging

### Helm Values

```yaml
config:
  # List of namespace regex patterns
  namespaceRegex:
    - "^production-.*"
    - "^staging-.*"
  
  # Enable dry-run mode
  dryRun: false
  
  # Enable debug logging
  debug: false
  
  # Enable trace logging
  trace: false
```

## Examples

### Basic Usage

Deploy a ReplicaSet with a ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: default
data:
  config.yaml: |
    app:
      name: myapp
      debug: true
---
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: app-rs
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
      - name: app
        image: nginx
        volumeMounts:
        - name: config
          mountPath: /etc/config
      volumes:
      - name: config
        configMap:
          name: app-config
```

After deployment, the operator will automatically add the ReplicaSet as an owner of the ConfigMap. When you delete the ReplicaSet, the ConfigMap will be garbage collected automatically.

### Namespace Filtering

To monitor only specific namespaces, configure the operator with regex patterns:

```bash
# Monitor only production and staging namespaces
helm install configmap-rs-operator configmap-rs-operator/configmap-rs-operator \
  --set config.namespaceRegex[0]="^production-.*" \
  --set config.namespaceRegex[1]="^staging-.*"
```

### Dry Run Mode

Test the operator without making changes:

```bash
helm install configmap-rs-operator configmap-rs-operator/configmap-rs-operator \
  --set config.dryRun=true \
  --set config.debug=true
```

## Development

### Prerequisites

- Go 1.24+
- Docker
- kubectl
- Kind (for local testing)

### Running Locally

1. Install CRDs (none in this project, but keeping for consistency):

```bash
make install
```

2. Run the operator locally:

```bash
make run
```

### Testing

Run unit tests:

```bash
make test
```

Run e2e tests:

```bash
make test-e2e
```

### Building

Build the binary:

```bash
make build
```

Build and push the Docker image:

```bash
make docker-build docker-push IMG=<your-registry>/configmap-rs-operator:tag
```

## Monitoring and Observability

The operator exposes Prometheus metrics on port 8080 (configurable):

- `controller_runtime_reconcile_total`: Total number of reconciliations
- `controller_runtime_reconcile_duration_seconds`: Time spent in reconciliation
- Standard Go runtime metrics

Health checks are available on port 8081:

- `/healthz`: Liveness probe
- `/readyz`: Readiness probe

## Security

The operator follows security best practices:

- Runs as non-root user
- Uses distroless base image
- Minimal RBAC permissions
- Read-only root filesystem
- No privilege escalation

Required RBAC permissions:

```yaml
- apiGroups: ["apps"]
  resources: ["replicasets"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run tests and linting: `make test lint`
6. Submit a pull request

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

## Support

- [GitHub Issues](https://github.com/matanbaruch/configmap-rs-operator/issues)
- [GitHub Discussions](https://github.com/matanbaruch/configmap-rs-operator/discussions)