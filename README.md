# External DNS Rackspace Webhook

A webhook provider for [External DNS](https://github.com/kubernetes-sigs/external-dns) that enables automatic DNS record management for Rackspace Cloud DNS.

## Overview

This webhook integrates External DNS with Rackspace Cloud DNS, allowing Kubernetes services, ingresses, and Gateway API routes to automatically create, update, and delete DNS records in your Rackspace-managed domains.

## Features

- **Automatic DNS Management**: Creates and manages DNS records based on Kubernetes services, ingresses, and Gateway API routes
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

Use the official [external-dns Helm chart](https://kubernetes-sigs.github.io/external-dns/) from the kubernetes-sigs project, with this webhook running as a sidecar container. This repository publishes the Rackspace webhook image only; it no longer ships or releases a custom Helm chart.

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
      tag: 0.3.1
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
      - name: DOMAIN_FILTER
        value: example.com
    service:
      port: 8080
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
    securityContext:
      capabilities:
        drop:
          - ALL
      readOnlyRootFilesystem: true
      runAsNonRoot: true
      runAsUser: 65534

sources:
  - service
  - ingress
  - gateway-httproute
  - gateway-tlsroute
  - gateway-tcproute
  - gateway-udproute

extraArgs:
  webhook-provider-url: http://localhost:8888

domainFilters:
  - example.com   # replace with your domain(s)

policy: upsert-only
registry: noop
interval: 10m
logLevel: info
```

> **Note**: the Rackspace webhook exposes the provider API on `localhost:8888` by default and exposes health checks on container port `8080`. Keep `extraArgs.webhook-provider-url` pointed at `http://localhost:8888`; the `provider.webhook.service.port` value is for the chart-managed Service and health/metrics path.

For multiple managed DNS suffixes, keep the webhook `DOMAIN_FILTER` and external-dns `domainFilters` aligned:

```yaml
provider:
  webhook:
    env:
      - name: DOMAIN_FILTER
        value: example.com,iad3.example.com

domainFilters:
  - example.com
  - iad3.example.com
```

The example keeps `policy: upsert-only` and `registry: noop`, which means source removal does not automatically delete DNS records. Use `policy: sync` with a stable TXT registry configuration only when Kubernetes should be the deletion source of truth.

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

### Choosing policy and TXT flags

Start with these defaults unless you have a reason to change them:

```yaml
policy: upsert-only
registry: txt
txtOwnerId: my-cluster-prod
txtPrefix: external-dns-
```

| Flag | Use it when | Notes |
|------|-------------|-------|
| `--domain-filter=example.com` | Always | Keeps external-dns away from domains this install should not manage. |
| `--policy=upsert-only` | First install, testing, or any zone where deletes must be manual | Creates and updates records, but does not delete records when Kubernetes objects disappear. |
| `--policy=sync` | You want external-dns to remove stale records automatically | Use only with a tight `domain-filter` and a stable ownership registry. This is what removes old records. |
| `--registry=txt` | Production or any shared Rackspace zone | Creates TXT ownership records so external-dns only changes records owned by this install. |
| `--txt-owner-id=<id>` | Any time `registry=txt` is enabled | Pick one stable value per cluster/environment. Changing it makes existing records look unowned. |
| `--txt-prefix=external-dns-` | Recommended with TXT registry | Keeps ownership records separate from user TXT records and supports CNAME ownership cleanly. |
| `--registry=noop` | Short-lived tests where ownership records are unwanted | Pair with `upsert-only` unless you are intentionally managing every matching record. |
| `--source=service,ingress,...` | To choose what Kubernetes objects create DNS | Use only the sources you actually need. Fewer sources are easier to reason about. |
| `--interval=1m` or higher | Normal reconciliation | Short intervals are useful in tests; production usually does not need sub-minute polling. |

Use `sync` when Kubernetes should be the source of truth for the selected domain. Use `upsert-only` when DNS deletion needs a human review step. Use TXT ownership in production; use `noop` only when you deliberately do not want ownership records.

## Development

### Building

```bash
# Build binary
make build

# Build Docker image
make docker-build

# Run tests
make test

# Run Kubernetes e2e tests against OrbStack/current kubectl context
make e2e
```

### Running Locally

```bash
export RACKSPACE_USERNAME="your-username"
export RACKSPACE_API_KEY="your-api-key"
export DOMAIN_FILTER="example.com"
make run
```

### End-to-end testing

`make e2e` builds the webhook image, starts a mock Rackspace Identity/Cloud DNS API in Kubernetes, and runs ExternalDNS against the webhook in the current `kubectl` context. It is designed for OrbStack and also works with any cluster that can run locally built Docker images.

The e2e test covers:

- `upsert-only` with `noop` registry: creates records and confirms source deletion does not remove DNS.
- `sync` with `txt` registry, `--txt-owner-id`, and `--txt-prefix`: creates records, removes seeded older owned records, and removes records after the source is deleted.

Requirements: Docker, `kubectl`, `curl`, and `jq`.

## API Endpoints

- `GET /` - Negotiation endpoint (returns domain filter)
- `GET /records` - Retrieve all DNS records
- `POST /records` - Apply DNS record changes
- `POST /adjustendpoints` - Normalize and validate endpoints
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

### external-dns Compatibility (SRV Records)

SRV record support requires external-dns v0.21.0 or later:

- **Versions prior to 0.21** do not include SRV in the TXT registry's `getSupportedTypes()`. This prevents the registry from matching `srv-` prefixed TXT ownership records to their SRV data records, causing all SRV records to appear unowned. external-dns will not update or delete them.

- **Version 0.21.0** fixes the ownership matching but introduces contradictory SRV validation in the CRD source. The CRD validator rejects targets with a trailing dot, while `ValidateSRVRecord` requires one per RFC 2782. This makes it impossible to create SRV records via DNSEndpoint CRDs. See [kubernetes-sigs/external-dns#6357](https://github.com/kubernetes-sigs/external-dns/issues/6357) and the fix in [PR #6383](https://github.com/kubernetes-sigs/external-dns/pull/6383).

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
