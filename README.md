# K8s Ingress Meta Sync

A Kubernetes controller that dynamically syncs SaaS Cloud provider IP ranges to various ingress services.

![Architecture Overview](docs/images/architecture-overview.png)

## Overview

SaaS providers (GitHub, AWS, etc.) frequently update the IP ranges origin their services. This creates a challenge for organizations that need to maintain accurate firewall rules, ingress configurations, and security policies incresing attack vectors velocity.

**K8s Ingress Meta Sync** solves this problem by:

1. Continuously monitoring IP range metadata services, published by SaaS providers
2. Detecting changes in published IP ranges
3. Automatically and ideomatically updating ingress layer 7 components (Cloudflare App FW, Istio) rules accordingly.
4. Supporting X-Forwarded-For header enrichment for proper origin client IP tracking

## Features

- **Multi-Provider Support**: Initially supports GitHub, with a pluggable architecture for adding more providers (AWS, etc.)
- **Multi-Ingress Support**: 
  - Cloudflare: Updates firewall rules with latest IP ranges
  - Istio: Configures EnvoyFilters for proper X-Forwarded-For handling
- **Kubernetes Native**: Implemented as a Kubernetes operator with custom resources
- **Filtering Capabilities**: Include/exclude specific IP ranges or services
- **Reconciliation Loop**: Regular checks ensure configurations stay in sync
- **Robust Error Handling**: Exponential backoff, circuit breaker patterns, and configurable failure modes
- **Detailed Status Reporting**: Clear visibility into sync status and errors

## Architecture

The controller follows a modular design with three main components:

1. **Provider Subsystem**: Responsible for fetching IP range metadata from sources
2. **Ingress Subsystem**: Handles applying IP ranges to different ingress services
3. **Reconciliation Engine**: Manages the synchronization process and error handling

Custom Resource Definitions (CRDs):
- `ProviderConfig`: Defines IP metadata sources
- `IngressConfig`: Defines ingress services
- `SyncConfig`: Maps providers to ingress services, defines sync policies

![Component Diagram](docs/images/component-diagram.png)

## Installation

### Prerequisites

- Kubernetes cluster (v1.19+)
- kubectl v1.19+
- For Istio ingress: Istio installed in your cluster
- For Cloudflare ingress: Cloudflare API token with appropriate permissions

### Installing CRDs

```bash
kubectl apply -f config/crds/providerconfig.yaml
kubectl apply -f config/crds/ingressconfig.yaml
kubectl apply -f config/crds/syncconfig.yaml
```

### Installing the Controller

```bash
# Create namespace and RBAC resources
kubectl apply -f config/rbac.yaml

# Deploy the controller
kubectl apply -f config/deployment.yaml
```

## Configuration

### GitHub Provider Configuration

```yaml
apiVersion: ingress-meta-sync.k8s.io/v1alpha1
kind: ProviderConfig
metadata:
  name: github-ip-ranges
spec:
  type: github
  github:
    # Set to true for GitHub Enterprise (default) or false for public GitHub
    enterprise: true
    api:
      secretRef:
        name: github-api-token
        namespace: ingress-meta-sync-system
    # Check for updates every 15 minutes
    pollingInterval: 15m
```

### Cloudflare Ingress Configuration

```yaml
apiVersion: ingress-meta-sync.k8s.io/v1alpha1
kind: IngressConfig
metadata:
  name: cloudflare-ingress
spec:
  type: cloudflare
  cloudflare:
    api:
      secretRef:
        name: cloudflare-api-token
        namespace: ingress-meta-sync-system
    ruleConfig:
      zoneId: "your-cloudflare-zone-id"
      ruleName: "github-ip-ranges"
      description: "GitHub IP ranges automatically managed by ingress-meta-sync"
      action: "allow"
      priority: 100
    updateStrategy: "direct"
```

### Istio Ingress Configuration

```yaml
apiVersion: ingress-meta-sync.k8s.io/v1alpha1
kind: IngressConfig
metadata:
  name: istio-ingress
spec:
  type: istio
  istio:
    namespace: istio-system
    # Configure x-forwarded-for header handling
    xForwardedForConfig:
      enabled: true
      headerName: "X-Forwarded-For"
    # Configure which Istio gateway to target
    gatewaySelector:
      name: "ingressgateway"
      namespace: "istio-system"
      labels:
        app: "istio-ingressgateway"
```

### Sync Configuration

```yaml
apiVersion: ingress-meta-sync.k8s.io/v1alpha1
kind: SyncConfig
metadata:
  name: github-to-ingress
spec:
  providers:
    - name: github-ip-ranges
      # Optional: only include specific IP ranges
      includeRanges:
        - "web"
        - "api"
        - "git"
  ingress:
    - name: cloudflare-ingress
    - name: istio-ingress
  syncPolicy:
    failureMode: continue
    retryConfig:
      maxRetries: 3
      backoffMultiplier: 2
      initialDelaySeconds: 5
```

## Complete Examples

Check the `examples/` directory for complete configuration examples:

- [GitHub to Cloudflare](examples/github-to-cloudflare.yaml)
- [GitHub to Istio](examples/github-to-istio.yaml)

## Development

### Prerequisites

- Go 1.21+
- Docker
- Kubernetes cluster for testing

### Building

```bash
# Build the binary
go build -o bin/manager cmd/manager/main.go

# Build the Docker image
docker build -t your-registry/k8s-ingress-meta-sync:latest .
```

### Testing

```bash
# Run unit tests
go test ./...

# Run integration tests
make test-integration
```

## Troubleshooting

### Common Issues

#### Provider IP Ranges Not Fetched

Check the SyncConfig status:

```bash
kubectl get syncconfig github-to-ingress -o yaml
```

Look at the `status.providerStatus` section for errors.

#### Ingress Not Updated

Check the SyncConfig status:

```bash
kubectl get syncconfig github-to-ingress -o yaml
```

Look at the `status.ingressStatus` section for errors.

### Logs

```bash
kubectl logs -n ingress-meta-sync-system deployment/ingress-meta-sync-controller
```

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

This project is licensed under the [MIT License](LICENSE).
