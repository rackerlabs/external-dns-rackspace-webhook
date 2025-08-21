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

- Kubernetes cluster with External DNS installed
- Rackspace Cloud DNS account with API access
- Rackspace username and API key## Qui
ck Start

### 1. Create Rackspace Credentials Secret

```bash
kubectl create secret generic external-dns-rackspace-webhook \
  --from-literal=username="your-rackspace-username" \
  --from-literal=api-key="your-rackspace-api-key" \
  -n external-dns
```

### 2. Deploy the Webhook

```bash
kubectl apply -f examples/deploy.yaml
```

### 3. Test with Sample Application

```bash
kubectl apply -f examples/nginx.yaml
```

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `RACKSPACE_USERNAME` | Yes | - | Rackspace account username |
| `RACKSPACE_API_KEY` | Yes | - | Rackspace API key |
| `DOMAIN_FILTER` | No | - | Comma-separated list of domains to manage |
| `DRY_RUN` | No | `false` | Enable dry run mode (no actual changes) |
| `LOG_LEVEL` | No | `info` | Log level (debug, info, warn, error) |
| `PORT` | No | `8888` | HTTP server port |

### External DNS Configuration

Configure External DNS to use this webhook:

```yaml
args:
  - --source=service
  - --source=ingress
  - --provider=webhook
  - --registry=noop
  - --webhook-provider-url=http://localhost:8888
  - --domain-filter=example.com
  - --policy=upsert-only
```

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

Enable debug logging to see detailed operation logs:

```yaml
env:
- name: LOG_LEVEL
  value: "debug"
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