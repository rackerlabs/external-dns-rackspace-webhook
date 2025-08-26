# External DNS Rackspace Webhook Helm Chart

This Helm chart deploys [ExternalDNS](https://github.com/kubernetes-sigs/external-dns) with a Rackspace webhook provider to manage DNS records in a Kubernetes cluster. It automates DNS record creation, updates, and deletions for services and ingresses using the Rackspace DNS API via a custom webhook.

## Prerequisites

- Kubernetes cluster (version 1.19 or higher recommended)
- Helm 3.8.0 or higher
- Rackspace account with API key and username
- Access to the OCI registry: `oci://ghcr.io/rackerlabs/external-dns-rackspace-webhook/charts/external-dns-rackspace-webhook`

## Installation

To install the Helm chart, run the following command, replacing `YOUR_USERNAME` and `YOUR_API_KEY` with your Rackspace credentials:

```bash
helm install external-dns oci://ghcr.io/rackerlabs/external-dns-rackspace-webhook/charts/external-dns-rackspace-webhook \
  --create-namespace \
  --namespace external-dns \
  --set-string rackspaceWebhook.secret.username="YOUR_USERNAME" \
  --set-string rackspaceWebhook.secret.apiKey="YOUR_API_KEY"
```

### Optional: Custom Namespace and Release Name

To install in a different namespace or with a custom release name:

```bash
helm install my-release oci://ghcr.io/rackerlabs/external-dns-rackspace-webhook/charts/external-dns-rackspace-webhook \
  --create-namespace \
  --namespace my-namespace \
  --set-string rackspaceWebhook.secret.username="YOUR_USERNAME" \
  --set-string rackspaceWebhook.secret.apiKey="YOUR_API_KEY"
```

## Chart Overview

The chart deploys:
- **ExternalDNS**: Manages DNS records based on Kubernetes services and ingresses.
- **Rackspace Webhook**: A sidecar container that interfaces with the Rackspace DNS API.
- **Kubernetes Resources**: Includes a Deployment, Service, ServiceAccount, ClusterRole, ClusterRoleBinding, and Secret for secure operation.

### Key Features
- Configurable domain filtering to limit DNS management to specific domains.
- RBAC support for secure cluster access.
- Health and readiness probes for both ExternalDNS and the Rackspace webhook.
- Secure secret management for Rackspace credentials.
- Customizable container security contexts for restricted permissions.
- Support for `noop` registry to disable TXT record management.

## Configuration

The chart is highly configurable via the `values.yaml` file. Below is a table of all configurable options, followed by examples of overriding key values.

### Configuration Options

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `namespace` | string | `external-dns` | Namespace for deploying the chart resources. |
| `domainFilter` | string | `example.com` | Domain to filter DNS records (e.g., `example.com`). |
| `externalDns.enabled` | bool | `true` | Enable or disable the ExternalDNS deployment. |
| `externalDns.image.repository` | string | `registry.k8s.io/external-dns/external-dns` | ExternalDNS container image repository. |
| `externalDns.image.tag` | string | `v0.18.0` | ExternalDNS container image tag. |
| `externalDns.image.pullPolicy` | string | `IfNotPresent` | Image pull policy for ExternalDNS. |
| `externalDns.replicas` | int | `1` | Number of ExternalDNS replicas. |
| `externalDns.serviceAccount.create` | bool | `true` | Create a ServiceAccount for ExternalDNS. |
| `externalDns.serviceAccount.name` | string | `external-dns` | Name of the ServiceAccount. |
| `externalDns.rbac.create` | bool | `true` | Create RBAC resources (ClusterRole and ClusterRoleBinding). |
| `externalDns.service.enabled` | bool | `true` | Enable the ExternalDNS service. |
| `externalDns.service.type` | string | `ClusterIP` | Service type (e.g., `ClusterIP`, `LoadBalancer`). |
| `externalDns.service.port` | int | `7979` | Service port for ExternalDNS. |
| `externalDns.args` | list | See `values.yaml` | Arguments passed to the ExternalDNS container. |
| `externalDns.securityContext.fsGroup` | int | `65534` | Filesystem group ID for ExternalDNS. |
| `externalDns.containerSecurityContext` | object | See `values.yaml` | Security context for the ExternalDNS container. |
| `externalDns.livenessProbe` | object | See `values.yaml` | Liveness probe configuration for ExternalDNS. |
| `externalDns.readinessProbe` | object | See `values.yaml` | Readiness probe configuration for ExternalDNS. |
| `externalDns.resources` | object | `{}` | Resource limits and requests for ExternalDNS. |
| `rackspaceWebhook.enabled` | bool | `true` | Enable or disable the Rackspace webhook sidecar. |
| `rackspaceWebhook.image.repository` | string | `ghcr.io/rackerlabs/external-dns-rackspace-webhook` | Rackspace webhook container image repository. |
| `rackspaceWebhook.image.tag` | string | `latest` | Rackspace webhook container image tag. |
| `rackspaceWebhook.image.pullPolicy` | string | `IfNotPresent` | Image pull policy for the webhook. |
| `rackspaceWebhook.secret.create` | bool | `true` | Create a Secret for Rackspace credentials. |
| `rackspaceWebhook.secret.name` | string | `external-dns-rackspace-webhook` | Name of the Secret for Rackspace credentials. |
| `rackspaceWebhook.secret.username` | string | `""` | Rackspace username (must be provided). |
| `rackspaceWebhook.secret.apiKey` | string | `""` | Rackspace API key (must be provided). |
| `rackspaceWebhook.env.LOG_LEVEL` | string | `info` | Log level for the webhook (`info`, `debug`, etc.). |
| `rackspaceWebhook.env.DRY_RUN` | string | `"false"` | Enable dry run mode for the webhook (`"true"` or `"false"`). |
| `rackspaceWebhook.containerSecurityContext` | object | See `values.yaml` | Security context for the webhook container. |
| `rackspaceWebhook.livenessProbe` | object | See `values.yaml` | Liveness probe configuration for the webhook. |
| `rackspaceWebhook.readinessProbe` | object | See `values.yaml` | Readiness probe configuration for the webhook. |
| `rackspaceWebhook.resources` | object | `{}` | Resource limits and requests for the webhook. |

### Example Overrides

Below are examples of overriding values during installation or upgrade.

#### Example 1: Custom Domain Filter and ExternalDNS Args
To change the domain filter to `mydomain.com` and modify `externalDns.args` to include a different source and interval:

```bash
helm install external-dns oci://ghcr.io/rackerlabs/external-dns-rackspace-webhook/charts/external-dns-rackspace-webhook \
  --create-namespace \
  --namespace external-dns \
  --set-string rackspaceWebhook.secret.username="YOUR_USERNAME" \
  --set-string rackspaceWebhook.secret.apiKey="YOUR_API_KEY" \
  --set-string domainFilter="mydomain.com" \
  --set externalDns.args="{--source=ingress,--interval=10m,--provider=webhook,--webhook-provider-url=http://localhost:8888,--policy=sync,--log-level=info,--registry=txt,--txt-owner-id=my-id}"
```

This sets the domain filter to `mydomain.com` and overrides the `externalDns.args` to use only `ingress` as the source, a 10-minute sync interval, and enables TXT registry with a custom owner ID.

#### Example 2: Increase Replicas and Add Resource Limits
To increase ExternalDNS replicas to 2 and set resource limits for both containers:

```bash
helm install external-dns oci://ghcr.io/rackerlabs/external-dns-rackspace-webhook/charts/external-dns-rackspace-webhook \
  --create-namespace \
  --namespace external-dns \
  --set-string rackspaceWebhook.secret.username="YOUR_USERNAME" \
  --set-string rackspaceWebhook.secret.apiKey="YOUR_API_KEY" \
  --set externalDns.replicas=2 \
  --set externalDns.resources.limits.cpu="500m" \
  --set externalDns.resources.limits.memory="512Mi" \
  --set externalDns.resources.requests.cpu="100m" \
  --set externalDns.resources.requests.memory="128Mi" \
  --set rackspaceWebhook.resources.limits.cpu="300m" \
  --set rackspaceWebhook.resources.limits.memory="256Mi" \
  --set rackspaceWebhook.resources.requests.cpu="50m" \
  --set rackspaceWebhook.resources.requests.memory="64Mi"
```

This increases replicas and sets CPU/memory limits and requests for both ExternalDNS and the webhook.

#### Example 3: Disable Webhook and Use Custom Service Type
To disable the Rackspace webhook and change the service type to `LoadBalancer`:

```bash
helm install external-dns oci://ghcr.io/rackerlabs/external-dns-rackspace-webhook/charts/external-dns-rackspace-webhook \
  --create-namespace \
  --namespace external-dns \
  --set rackspaceWebhook.enabled=false \
  --set externalDns.service.type=LoadBalancer
```

This disables the webhook (requiring an alternative provider) and exposes ExternalDNS via a LoadBalancer.

## Upgrading the Chart

To upgrade the chart with new values:

```bash
helm upgrade external-dns oci://ghcr.io/rackerlabs/external-dns-rackspace-webhook/charts/external-dns-rackspace-webhook \
  --namespace external-dns \
  --set-string rackspaceWebhook.secret.username="YOUR_USERNAME" \
  --set-string rackspaceWebhook.secret.apiKey="YOUR_API_KEY"
```