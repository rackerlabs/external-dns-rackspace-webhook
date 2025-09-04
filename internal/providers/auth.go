package providers

import (
	"context"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/rackerlabs/goclouddns"
	"github.com/rackerlabs/goraxauth"
)

// AuthProvider interface abstracts the authentication process
type AuthProvider interface {
	Authenticate(ctx context.Context, opts goraxauth.AuthOptions) (*gophercloud.ProviderClient, error)
	CreateDNSClient(provider *gophercloud.ProviderClient, opts gophercloud.EndpointOpts) (*gophercloud.ServiceClient, error)
}

type RackspaceAuthProvider struct{}

func NewRackspaceAuthProvider() *RackspaceAuthProvider {
	return &RackspaceAuthProvider{}
}

func (r *RackspaceAuthProvider) Authenticate(ctx context.Context, opts goraxauth.AuthOptions) (*gophercloud.ProviderClient, error) {
	return goraxauth.AuthenticatedClient(ctx, opts)
}

func (r *RackspaceAuthProvider) CreateDNSClient(provider *gophercloud.ProviderClient, opts gophercloud.EndpointOpts) (*gophercloud.ServiceClient, error) {
	return goclouddns.NewCloudDNS(provider, opts)
}
