# External DNS Rackspace Webhook

A webhook provider for [External DNS](https://github.com/kubernetes-sigs/external-dns) that enables automatic DNS record management for Rackspace Cloud DNS.

## Overview

This webhook integrates External DNS with Rackspace Cloud DNS, allowing Kubernetes services and ingresses to automatically create, update, and delete DNS records in your Rackspace-managed domains.

## Features

- **Automatic DNS Management**: Creates and manages DNS records based on Kubernetes services and ingresses
- **Multiple Record Types**: Supports A, AAAA, CNAME, TXT, and other standard DNS record types
- **Domain Filtering**: Configure which domains the webhook should manage
- **TTL Management**: Configurable TTL values with automatic normalization (minimum 300s)
- **Dry Run Mode**: Test changes without actually modifying DNS records
- **Health Checks**: Built-in health endpoints for monitoring

## Prerequisites

- Kubernetes cluster
- Helm 3.8.0 or higher
- Rackspace Cloud DNS account with API access
- Rackspace username and API key

## Installation

The recommended installation method uses the official [external-dns Helm chart](https://kubernetes-sigs.github.io/external-dns/) from the kubernetes-sigs project, with this webhook running as a sidecar container.

### 1. Add the external-dns Helm repository

```bash
helm repo add external-dns https://kubernetes-sigs.github.io/external-dns/
helm repo update
```

### 2. Create a secret with your Rackspace credentials

```bash
kubectl create namespace external-dns

kubectl create secret generic rackspace-credentials \
  --namespace external-dns \
  --from-literal=username="YOUR_USERNAME" \
  --from-literal=api-key="YOUR_API_KEY"
```

### 3. Create a values file

```yaml
# external-dns-rackspace-values.yaml
provider:
  name: webhook
  webhook:
    image:
      repository: ghcr.io/rackerlabs/external-dns-rackspace-webhook
      tag: v0.2.2
    env:
      - name: RACKSPACE_USERNAME
        valueFrom:
          secretKeyRef:
            name: rackspace-credentials
            key: username
      - name: RACKSPACE_API_KEY
        valueFrom:
          secretKeyRef:
            name: rackspace-credentials
            key: api-key
      - name: LOG_LEVEL
        value: info
      - name: DRY_RUN
        value: "false"
    service:
      port: 8888
    livenessProbe:
      httpGet:
        path: /healthz
        port: http-webhook
      initialDelaySeconds: 10
      periodSeconds: 10
      timeoutSeconds: 5
      failureThreshold: 2
    readinessProbe:
      httpGet:
        path: /healthz
        port: http-webhook
      initialDelaySeconds: 5
      periodSeconds: 10
      timeoutSeconds: 5
      failureThreshold: 6

sources:
  - service
  - ingress

domainFilters:
  - example.com   # replace with your domain(s)

policy: upsert-only
logLevel: info
```

> **Note**: `service.port: 8888` must match the webhook's `PORT` env var (default `8888`). The chart uses this value to configure `--webhook-provider-url` automatically and to route health probe traffic.

### 4. Install

```bash
helm upgrade --install external-dns external-dns/external-dns \
  --namespace external-dns \
  -f external-dns-rackspace-values.yaml
```

### Upgrading

```bash
helm upgrade external-dns external-dns/external-dns \
  --namespace external-dns \
  -f external-dns-rackspace-values.yaml
```

## Configuration

### Webhook environment variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `RACKSPACE_USERNAME` | Yes | - | Rackspace account username |
| `RACKSPACE_API_KEY` | Yes | - | Rackspace API key |
| `RACKSPACE_TENANT_ID` | No | - | Rackspace tenant ID |
| `DOMAIN_FILTER` | No | - | Comma-separated list of domains to manage |
| `DRY_RUN` | No | `false` | Enable dry run mode (no actual changes) |
| `LOG_LEVEL` | No | `info` | Log level (debug, info, warn, error) |
| `PORT` | No | `8888` | HTTP server port |

### Common external-dns chart values

See the [external-dns chart documentation](https://kubernetes-sigs.github.io/external-dns/latest/charts/external-dns/) for the full list. Commonly used values:

| Value | Description |
|-------|-------------|
| `domainFilters` | List of domains to manage |
| `policy` | `upsert-only` (safe default) or `sync` (enables deletions) |
| `sources` | Kubernetes resource types to watch (`service`, `ingress`, `crd`, etc.) |
| `interval` | Sync interval (default `1m`) |
| `logLevel` | external-dns log level |
| `resources` | CPU/memory limits for the external-dns container |
| `provider.webhook.resources` | CPU/memory limits for the webhook sidecar |

## Development

### Building

```bash
# Build binary
make build

# Build Docker image
make docker-build

# Run tests
make test
```

### Running Locally

```bash
export RACKSPACE_USERNAME="your-username"
export RACKSPACE_API_KEY="your-api-key"
export DOMAIN_FILTER="example.com"
make run
```

## API Endpoints

- `GET /` - Negotiation endpoint (returns domain filter)
- `GET /records` - Retrieve all DNS records
- `POST /records` - Apply DNS record changes
- `POST /adjustEndpoints` - Normalize and validate endpoints
- `GET /healthz` - Health check endpoint

## Docker

### Using Pre-built Image

```bash
docker run -e RACKSPACE_USERNAME=user -e RACKSPACE_API_KEY=key \
  ghcr.io/rackerlabs/external-dns-rackspace-webhook:latest
```

### Building Your Own

```bash
docker build -t external-dns-rackspace-webhook .
```

## Troubleshooting

### Common Issues

1. **Authentication Errors**: Verify your Rackspace username and API key
2. **Domain Not Found**: Ensure the domain exists in your Rackspace Cloud DNS
3. **Permission Denied**: Check that your API key has DNS management permissions
4. **TTL Too Low**: The webhook enforces a minimum TTL of 300 seconds

### Debug Mode

Enable debug logging by setting `LOG_LEVEL` in your values file:

```yaml
provider:
  webhook:
    env:
      - name: LOG_LEVEL
        value: debug
logLevel: debug
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the terms specified in the LICENSE file.

## Support

For issues and questions:
- Create an issue in this repository
- Check the External DNS documentation
- Review Rackspace Cloud DNS API documentation