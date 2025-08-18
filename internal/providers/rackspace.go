package providers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/identity/v2/tokens"
	"github.com/gophercloud/gophercloud/v2/pagination"
	"github.com/rackerlabs/goclouddns"
	"github.com/rackerlabs/goclouddns/domains"
	"github.com/rackerlabs/goclouddns/records"
	"github.com/rackerlabs/goraxauth"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
)

type RackspaceConfig struct {
	IdentityEndpoint string
	Username         string
	APIKey           string
	TenantID         string
	Listen           string
	DomainFilter     []string
	DryRun           bool
	LogLevel         string
}

type RackspaceProvider struct {
	Client       *gophercloud.ServiceClient
	DomainFilter *endpoint.DomainFilter
	DryRun       bool
}

func NewRackspaceProvider(config *RackspaceConfig) (*RackspaceProvider, error) {
	if config.Username == "" || config.APIKey == "" {
		return nil, fmt.Errorf("RACKSPACE_USERNAME and RACKSPACE_API_KEY are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	authOpts := goraxauth.AuthOptions{
		AuthOptions: tokens.AuthOptions{
			IdentityEndpoint: config.IdentityEndpoint,
			Username:         config.Username,
		},
		ApiKey: config.APIKey,
	}
	if config.TenantID != "" {
		authOpts.TenantID = config.TenantID
	}

	provider, err := goraxauth.AuthenticatedClient(ctx, authOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with Rackspace: %v", err)
	}

	client, err := goclouddns.NewCloudDNS(provider, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to create Cloud DNS client: %v", err)
	}

	domainFilter := endpoint.NewDomainFilter(config.DomainFilter)
	log.Info("Initialized provider", "domainFilter", config.DomainFilter, "dryRun", config.DryRun)

	return &RackspaceProvider{
		Client:       client,
		DomainFilter: domainFilter,
		DryRun:       config.DryRun,
	}, nil
}

func (p *RackspaceProvider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	var endpoints []*endpoint.Endpoint
	opts := domains.ListOpts{}
	pager := domains.List(ctx, p.Client, opts)
	start := time.Now()

	err := pager.EachPage(ctx, func(ctx context.Context, page pagination.Page) (bool, error) {
		domainList, err := domains.ExtractDomains(page)
		if err != nil {
			return false, err
		}
		for _, domain := range domainList {
			if !p.DomainFilter.Match(domain.Name) {
				continue
			}
			recordOpts := records.ListOpts{}
			recordPager := records.List(ctx, p.Client, domain.ID, recordOpts)
			err := recordPager.EachPage(ctx, func(ctx context.Context, recordPage pagination.Page) (bool, error) {
				recordList, err := records.ExtractRecords(recordPage)
				if err != nil {
					return false, err
				}
				for _, record := range recordList {
					if ep := convertRecordToEndpoint(record, domain.Name); ep != nil {
						endpoints = append(endpoints, ep)
					}
				}
				return true, nil
			})
			if err != nil {
				return false, fmt.Errorf("failed to list records for domain %s: %w", domain.Name, err)
			}
		}
		return true, nil
	})
	log.Debug("Fetched records", "count", len(endpoints), "elapsed", time.Since(start))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch domains: %v", err)
	}

	return endpoints, nil
}

// ApplyChanges applies DNS record changes to Rackspace Cloud DNS
func (p *RackspaceProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	log.Info("Applying changes", "create", len(changes.Create), "updateNew", len(changes.UpdateNew), "delete", len(changes.Delete))
	if p.DryRun {
		log.Info("Dry run enabled, skipping changes")
		return nil
	}
	for _, ep := range changes.Delete {
		if err := p.deleteRecord(ctx, ep); err != nil {
			return fmt.Errorf("failed to delete record %s: %v", ep.DNSName, err)
		}
	}

	for _, ep := range changes.Create {
		if err := p.createRecord(ctx, ep); err != nil {
			return fmt.Errorf("failed to create record %s: %v", ep.DNSName, err)
		}
	}

	for _, ep := range changes.UpdateNew {
		if err := p.updateRecord(ctx, ep); err != nil {
			return fmt.Errorf("failed to update record %s: %v", ep.DNSName, err)
		}
	}

	return nil
}

func (p *RackspaceProvider) createRecord(ctx context.Context, endpoint *endpoint.Endpoint) error {
	domain, err := p.findDomain(ctx, endpoint.DNSName)
	if err != nil {
		return err
	}
	recordName := getRecordName(endpoint.DNSName, domain.Name)
	for _, target := range endpoint.Targets {
		createOpts := records.CreateOpts{
			Name: recordName + "." + domain.Name,
			Type: endpoint.RecordType,
			Data: target,
		}
		if endpoint.RecordTTL.IsConfigured() {
			ttl := uint(endpoint.RecordTTL)
			if ttl < 300 {
				ttl = 300 // Rackspace minimum TTL
			}
			createOpts.TTL = ttl
		}

		_, err = records.Create(ctx, p.Client, domain.ID, createOpts).Extract()
		if err != nil {
			return fmt.Errorf("failed to create record %s: %v", endpoint.DNSName, err)
		}
		log.Info("Created record", "dnsName", endpoint.DNSName, "type", endpoint.RecordType, "target", target)
	}
	return nil
}

func (p *RackspaceProvider) updateRecord(ctx context.Context, endpoint *endpoint.Endpoint) error {
	domain, err := p.findDomain(ctx, endpoint.DNSName)
	if err != nil {
		return err
	}

	if err := p.deleteRecordByName(ctx, domain, endpoint.DNSName, endpoint.RecordType); err != nil {
		log.Warn("Failed to delete existing record during update", "dnsName", endpoint.DNSName, "error", err)
	}

	return p.createRecord(ctx, endpoint)
}

func (p *RackspaceProvider) deleteRecord(ctx context.Context, endpoint *endpoint.Endpoint) error {
	domain, err := p.findDomain(ctx, endpoint.DNSName)
	if err != nil {
		return err
	}
	return p.deleteRecordByName(ctx, domain, endpoint.DNSName, endpoint.RecordType)
}

func (p *RackspaceProvider) deleteRecordByName(ctx context.Context, domain *domains.DomainList, dnsName, recordType string) error {
	recordName := getRecordName(dnsName, domain.Name)
	pager := records.List(ctx, p.Client, domain.ID, records.ListOpts{})

	var errors []error
	err := pager.EachPage(ctx, func(ctx context.Context, page pagination.Page) (bool, error) {
		recordList, err := records.ExtractRecords(page)
		if err != nil {
			return false, err
		}

		for _, record := range recordList {
			if record.Name == recordName+"."+domain.Name+"." && record.Type == recordType {
				err = records.Delete(ctx, p.Client, domain.ID, record.ID).ExtractErr()
				if err != nil {
					errors = append(errors, fmt.Errorf("failed to delete record %s: %v", record.Name, err))
				} else {
					log.Info("Deleted record", "dnsName", record.Name, "type", recordType)
				}
			}
		}
		return true, nil
	})

	if len(errors) > 0 {
		return fmt.Errorf("errors during deletion: %v", errors)
	}
	if err != nil {
		return fmt.Errorf("failed to list records: %v", err)
	}
	return nil
}

func (p *RackspaceProvider) findDomain(ctx context.Context, dnsName string) (*domains.DomainList, error) {
	dnsName = strings.TrimSuffix(strings.ToLower(dnsName), ".")
	opts := domains.ListOpts{}
	pager := domains.List(ctx, p.Client, opts)

	var bestMatch *domains.DomainList
	err := pager.EachPage(ctx, func(ctx context.Context, page pagination.Page) (bool, error) {
		domainList, err := domains.ExtractDomains(page)
		if err != nil {
			return false, err
		}
		for _, domain := range domainList {
			if strings.HasSuffix(dnsName, strings.TrimSuffix(domain.Name, ".")) {
				if bestMatch == nil || len(domain.Name) > len(bestMatch.Name) {
					bestMatch = &domain
				}
			}
		}
		return true, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list domains: %v", err)
	}
	if bestMatch == nil {
		return nil, fmt.Errorf("no matching domain found for %s", dnsName)
	}
	return bestMatch, nil
}

func convertRecordToEndpoint(record records.RecordList, domainName string) *endpoint.Endpoint {
	if record.Type == "NS" || record.Type == "SOA" {
		return nil
	}

	domainName = strings.TrimSuffix(domainName, ".")
	recordName := strings.TrimSuffix(record.Name, ".")

	var dnsName string
	if recordName == "" || recordName == domainName {
		dnsName = domainName + "."
	} else if strings.HasSuffix(recordName, "."+domainName) {
		dnsName = recordName + "."
	} else {
		dnsName = recordName + "." + domainName + "."
	}

	ep := &endpoint.Endpoint{
		DNSName:    dnsName,
		RecordType: record.Type,
		Targets:    []string{record.Data},
	}
	if record.TTL != 0 {
		ep.RecordTTL = endpoint.TTL(record.TTL)
	}

	return ep
}

// Records retrieves all DNS records from Rackspace Cloud DNS

func getRecordName(dnsName, domainName string) string {
	dnsName = strings.TrimSuffix(strings.ToLower(dnsName), ".")
	domainName = strings.TrimSuffix(strings.ToLower(domainName), ".")
	if dnsName == domainName {
		return "@"
	}
	if strings.HasSuffix(dnsName, "."+domainName) {
		return strings.TrimSuffix(dnsName, "."+domainName)
	}
	return dnsName
}
