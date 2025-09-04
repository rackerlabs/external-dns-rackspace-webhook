package providers

import (
	"context"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/pagination"
	"github.com/rackerlabs/goclouddns/domains"
	"github.com/rackerlabs/goclouddns/records"
)

// ServiceClient interface abstracts the Rackspace DNS client operations
type ServiceClient interface {
	ListDomains(ctx context.Context, opts domains.ListOpts) pagination.Pager
	ListRecords(ctx context.Context, domainID string, opts records.ListOpts) pagination.Pager
	CreateRecord(ctx context.Context, domainID string, opts records.CreateOpts) (*records.RecordList, error)
	DeleteRecord(ctx context.Context, domainID, recordID string) error
}

// RackspaceDNSClient implements DNSClient interface
type RackspaceDNSClient struct {
	client *gophercloud.ServiceClient
}

func NewRackspaceDNSClient(client *gophercloud.ServiceClient) *RackspaceDNSClient {
	return &RackspaceDNSClient{client: client}
}

func (r *RackspaceDNSClient) ListDomains(ctx context.Context, opts domains.ListOpts) pagination.Pager {
	return domains.List(ctx, r.client, opts)
}

func (r *RackspaceDNSClient) ListRecords(ctx context.Context, domainID string, opts records.ListOpts) pagination.Pager {
	return records.List(ctx, r.client, domainID, opts)
}

func (r *RackspaceDNSClient) CreateRecord(ctx context.Context, domainID string, opts records.CreateOpts) (*records.RecordList, error) {
	return records.Create(ctx, r.client, domainID, opts).Extract()
}

func (r *RackspaceDNSClient) DeleteRecord(ctx context.Context, domainID, recordID string) error {
	return records.Delete(ctx, r.client, domainID, recordID).ExtractErr()
}
