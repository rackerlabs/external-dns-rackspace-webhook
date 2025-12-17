package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/identity/v2/tokens"
	"github.com/gophercloud/gophercloud/v2/pagination"
	"github.com/rackerlabs/goclouddns/domains"
	"github.com/rackerlabs/goclouddns/records"
	"github.com/rackerlabs/goraxauth"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
)

const (
	defaultTokenLifetime   = 4 * time.Hour
	tokenRefreshBeforeTime = -1 * time.Hour
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
	mu            sync.RWMutex
	serviceClient ServiceClient
	authProvider  AuthProvider
	tokenExpiry   time.Time
	config        *RackspaceConfig
	DomainFilter  *endpoint.DomainFilter
	DryRun        bool
}

func NewRackspaceProvider(config *RackspaceConfig) (*RackspaceProvider, error) {
	authProvider := NewRackspaceAuthProvider()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	client, tokenExpiry, err := authenticateAndCreateClient(ctx, authProvider, config)
	if err != nil {
		return nil, err
	}

	dnsClient := NewRackspaceDNSClient(client)
	domainFilter := endpoint.NewDomainFilter(config.DomainFilter)

	log.Info("Initialized provider", "domainFilter", config.DomainFilter, "dryRun", config.DryRun)

	return &RackspaceProvider{
		serviceClient: dnsClient,
		authProvider:  authProvider,
		tokenExpiry:   tokenExpiry,
		config:        config,
		DomainFilter:  domainFilter,
		DryRun:        config.DryRun,
	}, nil
}

func (p *RackspaceProvider) getClient(ctx context.Context) ServiceClient {
	p.mu.Lock()
	defer p.mu.Unlock()
	if time.Now().Before(p.tokenExpiry.Add(tokenRefreshBeforeTime)) {
		return p.serviceClient
	}
	clientRaw, tokenExpiry, err := authenticateAndCreateClient(ctx, p.authProvider, p.config)
	if err != nil {
		log.Error("Failed to refresh Rackspace token", "error", err)
		return p.serviceClient
	}
	p.serviceClient = NewRackspaceDNSClient(clientRaw)
	p.tokenExpiry = tokenExpiry
	log.Info("Refreshed Rackspace token", "expiresAt", tokenExpiry)
	return p.serviceClient
}

func authenticateAndCreateClient(ctx context.Context, authProvider AuthProvider, config *RackspaceConfig) (*gophercloud.ServiceClient, time.Time, error) {
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

	provider, err := authProvider.Authenticate(ctx, authOpts)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to authenticate with Rackspace: %v", err)
	}

	tokenExpiry := time.Now().Add(defaultTokenLifetime)
	if provider.TokenID != "" {
		if authResult, ok := provider.GetAuthResult().(tokens.CreateResult); ok {
			token, err := authResult.ExtractToken()
			if err != nil {
				log.Warn("Failed to extract token, using default expiry", "error", err)
			} else if token != nil {
				tokenExpiry = token.ExpiresAt
			} else {
				log.Warn("Extracted token is nil, using default expiry")
			}
		}
	}

	client, err := authProvider.CreateDNSClient(provider, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to create Cloud DNS client: %v", err)
	}

	return client, tokenExpiry, nil
}

func (p *RackspaceProvider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	var endpoints []*endpoint.Endpoint
	opts := domains.ListOpts{}
	pager := p.getClient(ctx).ListDomains(ctx, opts)
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
			recordPager := p.getClient(ctx).ListRecords(ctx, domain.ID, records.ListOpts{})
			err := recordPager.EachPage(ctx, func(ctx context.Context, recordPage pagination.Page) (bool, error) {
				recordList, err := records.ExtractRecords(recordPage)
				if err != nil {
					return false, fmt.Errorf("failed to extract records: %w", err)
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
		return nil, fmt.Errorf("failed to fetch domains: %w", err)
	}

	return endpoints, nil
}

// ApplyChanges applies DNS record changes to Rackspace Cloud DNS
func (p *RackspaceProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	var errs []error
	log.Info("Applying changes",
		"create", len(changes.Create),
		"updateNew", len(changes.UpdateNew),
		"delete", len(changes.Delete),
	)
	if p.DryRun {
		log.Info("Dry run enabled, skipping changes")
		return nil
	}

	for _, ep := range changes.Delete {
		if err := p.deleteRecord(ctx, ep); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete record %s: %v", ep.DNSName, err))
		}
	}

	for _, ep := range changes.Create {
		if err := p.createRecord(ctx, ep); err != nil {
			errs = append(errs, fmt.Errorf("failed to create record %s: %v", ep.DNSName, err))
		}
	}

	for _, ep := range changes.UpdateNew {
		if err := p.updateRecord(ctx, ep); err != nil {
			errs = append(errs, fmt.Errorf("failed to update record %s: %v", ep.DNSName, err))
		}
	}

	if len(errs) == 0 {
		return nil
	}

	log.Error("collected errors while applying changes", "count", len(errs))
	for i, e := range errs {
		log.Error("collected error", "index", i, "err", e)
	}

	return errors.Join(errs...)
}

func convertRecordToEndpoint(record records.RecordList, domainName string) *endpoint.Endpoint {
	if record.Type == "NS" || record.Type == "SOA" {
		return nil
	}
	var labels map[string]string
	if record.Type == "TXT" && record.Comment != "" {
		if err := json.Unmarshal([]byte(record.Comment), &labels); err != nil {
			log.Warn("Failed to unmarshal TXT record labels", "name", record.Name, "comment", record.Comment, "error", err)
		}
	}
	ep := &endpoint.Endpoint{
		DNSName:    record.Name,
		RecordType: record.Type,
		Targets:    []string{record.Data},
		RecordTTL:  endpoint.TTL(record.TTL),
		Labels:     labels,
	}
	return ep
}

func (p *RackspaceProvider) createRecord(ctx context.Context, ep *endpoint.Endpoint) error {
	domain, err := p.findDomain(ctx, ep.DNSName)
	if err != nil {
		return err
	}
	fqdn := strings.TrimSuffix(strings.ToLower(ep.DNSName), ".")
	for _, target := range ep.Targets {
		var labels string
		if ep.RecordType == "TXT" {
			target = strings.Trim(target, `"`)
			if len(ep.Labels) > 0 {
				b, _ := json.Marshal(ep.Labels)
				labels = string(b)
			}
		}
		createOpts := records.CreateOpts{
			Name:    fqdn,
			Type:    ep.RecordType,
			Data:    target,
			Comment: labels,
		}
		if _, err := p.getClient(ctx).CreateRecord(ctx, domain.ID, createOpts); err != nil {
			return fmt.Errorf("failed to create record %s: %v", ep.DNSName, err)
		}
		log.Info("Created record", "dnsName", ep.DNSName, "type", ep.RecordType, "target", target)
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
	if domain == nil {
		return fmt.Errorf("domain cannot be nil")
	}
	wantName := strings.TrimSuffix(strings.ToLower(dnsName), ".")
	pager := p.getClient(ctx).ListRecords(ctx, domain.ID, records.ListOpts{})

	var errs []error
	err := pager.EachPage(ctx, func(ctx context.Context, page pagination.Page) (bool, error) {
		recordList, err := records.ExtractRecords(page)
		if err != nil {
			return false, err
		}

		for _, rec := range recordList {
			gotName := strings.TrimSuffix(strings.ToLower(rec.Name), ".")
			if gotName == wantName && strings.EqualFold(rec.Type, recordType) {
				if e := p.getClient(ctx).DeleteRecord(ctx, domain.ID, rec.ID); e != nil {
					errs = append(errs, fmt.Errorf("failed to delete record %s: %w", rec.Name, e))
				} else {
					log.Info("Deleted record", "dnsName", rec.Name, "type", recordType)
				}
			}
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("failed to list records: %w", err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (p *RackspaceProvider) findDomain(ctx context.Context, dnsName string) (*domains.DomainList, error) {
	if dnsName == "" {
		return nil, fmt.Errorf("DNS name cannot be empty")
	}
	dnsName = strings.TrimSuffix(strings.ToLower(dnsName), ".")
	opts := domains.ListOpts{}
	pager := p.getClient(ctx).ListDomains(ctx, opts)

	var bestMatch *domains.DomainList
	err := pager.EachPage(ctx, func(ctx context.Context, page pagination.Page) (bool, error) {
		domainList, err := domains.ExtractDomains(page)
		if err != nil {
			return false, fmt.Errorf("failed to extract domains: %w", err)
		}
		for _, domain := range domainList {
			domainName := strings.TrimSuffix(strings.ToLower(domain.Name), ".")
			if dnsName == domainName || strings.HasSuffix(dnsName, "."+domainName) {
				if bestMatch == nil || len(domainName) > len(strings.TrimSuffix(strings.ToLower(bestMatch.Name), ".")) {
					bestMatch = &domain
				}
			}
		}
		return true, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list domains: %w", err)
	}
	if bestMatch == nil {
		return nil, fmt.Errorf("no matching domain found for %s", dnsName)
	}
	return bestMatch, nil
}
