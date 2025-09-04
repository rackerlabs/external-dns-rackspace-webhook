package providers

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/gophercloud/gophercloud/v2"
	"sigs.k8s.io/external-dns/endpoint"

	th "github.com/gophercloud/gophercloud/v2/testhelper"
)

func FakeDNSClient() *gophercloud.ServiceClient {
	return &gophercloud.ServiceClient{
		ProviderClient: &gophercloud.ProviderClient{TokenID: "cbc36478b0bd8e67e89469c7749d4127"},
		Endpoint:       th.Endpoint(),
	}
}

func TestRackspaceProvider_findDomain(t *testing.T) {
	tests := []struct {
		name           string
		dnsName        string
		mockResponse   string
		wantErr        bool
		wantDomainID   string
		wantDomainName string
		setupMock      func()
	}{
		{
			name:    "happy path - subdomain matching",
			dnsName: "test.example.com",
			mockResponse: `{
				"domains": [
					{
						"id": "123456",
						"name": "example.com",
						"ttl": 3600,
						"emailAddress": "admin@example.com",
						"updated": "2023-01-01T00:00:00.000+0000",
						"created": "2023-01-01T00:00:00.000+0000"
					}
				]
			}`,
			wantErr:        false,
			wantDomainID:   "123456",
			wantDomainName: "example.com",
		},
		{
			name:    "exact domain match",
			dnsName: "example.com",
			mockResponse: `{
				"domains": [
					{
						"id": "123456",
						"name": "example.com",
						"ttl": 3600,
						"emailAddress": "admin@example.com",
						"updated": "2023-01-01T00:00:00.000+0000",
						"created": "2023-01-01T00:00:00.000+0000"
					}
				]
			}`,
			wantErr:        false,
			wantDomainID:   "123456",
			wantDomainName: "example.com",
		},
		{
			name:    "multiple domains - longest match wins",
			dnsName: "api.sub.example.com",
			mockResponse: `{
				"domains": [
					{
						"id": "123456",
						"name": "example.com",
						"ttl": 3600,
						"emailAddress": "admin@example.com",
						"updated": "2023-01-01T00:00:00.000+0000",
						"created": "2023-01-01T00:00:00.000+0000"
					},
					{
						"id": "789012",
						"name": "sub.example.com",
						"ttl": 3600,
						"emailAddress": "admin@sub.example.com",
						"updated": "2023-01-01T00:00:00.000+0000",
						"created": "2023-01-01T00:00:00.000+0000"
					}
				]
			}`,
			wantErr:        false,
			wantDomainID:   "789012",
			wantDomainName: "sub.example.com",
		},
		{
			name:    "case insensitive matching",
			dnsName: "TEST.EXAMPLE.COM",
			mockResponse: `{
				"domains": [
					{
						"id": "123456",
						"name": "example.com",
						"ttl": 3600,
						"emailAddress": "admin@example.com",
						"updated": "2023-01-01T00:00:00.000+0000",
						"created": "2023-01-01T00:00:00.000+0000"
					}
				]
			}`,
			wantErr:        false,
			wantDomainID:   "123456",
			wantDomainName: "example.com",
		},
		{
			name:    "trailing dot in dns name",
			dnsName: "test.example.com.",
			mockResponse: `{
				"domains": [
					{
						"id": "123456",
						"name": "example.com",
						"ttl": 3600,
						"emailAddress": "admin@example.com",
						"updated": "2023-01-01T00:00:00.000+0000",
						"created": "2023-01-01T00:00:00.000+0000"
					}
				]
			}`,
			wantErr:        false,
			wantDomainID:   "123456",
			wantDomainName: "example.com",
		},
		{
			name:    "trailing dot in domain name",
			dnsName: "test.example.com",
			mockResponse: `{
				"domains": [
					{
						"id": "123456",
						"name": "example.com.",
						"ttl": 3600,
						"emailAddress": "admin@example.com",
						"updated": "2023-01-01T00:00:00.000+0000",
						"created": "2023-01-01T00:00:00.000+0000"
					}
				]
			}`,
			wantErr:        false,
			wantDomainID:   "123456",
			wantDomainName: "example.com.",
		},
		{
			name:    "no matching domain",
			dnsName: "nonexistent.com",
			mockResponse: `{
				"domains": [
					{
						"id": "123456",
						"name": "example.com",
						"ttl": 3600,
						"emailAddress": "admin@example.com",
						"updated": "2023-01-01T00:00:00.000+0000",
						"created": "2023-01-01T00:00:00.000+0000"
					}
				]
			}`,
			wantErr: true,
		},
		{
			name:    "empty domains list",
			dnsName: "test.example.com",
			mockResponse: `{
				"domains": []
			}`,
			wantErr: true,
		},
		{
			name:    "partial domain name match should not match",
			dnsName: "notexample.com",
			mockResponse: `{
				"domains": [
					{
						"id": "123456",
						"name": "example.com",
						"ttl": 3600,
						"emailAddress": "admin@example.com",
						"updated": "2023-01-01T00:00:00.000+0000",
						"created": "2023-01-01T00:00:00.000+0000"
					}
				]
			}`,
			wantErr: true,
		},
		{
			name:    "deep subdomain matching",
			dnsName: "very.deep.sub.example.com",
			mockResponse: `{
				"domains": [
					{
						"id": "123456",
						"name": "example.com",
						"ttl": 3600,
						"emailAddress": "admin@example.com",
						"updated": "2023-01-01T00:00:00.000+0000",
						"created": "2023-01-01T00:00:00.000+0000"
					}
				]
			}`,
			wantErr:        false,
			wantDomainID:   "123456",
			wantDomainName: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th.SetupHTTP()
			defer th.TeardownHTTP()

			// Mock the domains list API response
			th.Mux.HandleFunc("/domains", func(w http.ResponseWriter, r *http.Request) {
				th.TestMethod(t, r, "GET")
				th.TestHeader(t, r, "X-Auth-Token", "cbc36478b0bd8e67e89469c7749d4127")

				w.Header().Add("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, tt.mockResponse)
			})

			p := &RackspaceProvider{
				serviceClient: NewRackspaceDNSClient(FakeDNSClient()),
				authProvider:  nil,
				DomainFilter:  endpoint.NewDomainFilter([]string{}),
				DryRun:        false,
			}

			got, err := p.findDomain(context.Background(), tt.dnsName)

			if (err != nil) != tt.wantErr {
				t.Errorf("RackspaceProvider.findDomain() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got == nil {
				t.Errorf("RackspaceProvider.findDomain() returned nil domain when expecting a match")
				return
			}

			if !tt.wantErr && got != nil {
				if got.ID != tt.wantDomainID {
					t.Errorf("RackspaceProvider.findDomain() returned unexpected domain ID: got=%s, want=%s", got.ID, tt.wantDomainID)
				}
				if got.Name != tt.wantDomainName {
					t.Errorf("RackspaceProvider.findDomain() returned unexpected domain name: got=%s, want=%s", got.Name, tt.wantDomainName)
				}
			}

			if tt.wantErr && got != nil {
				t.Errorf("RackspaceProvider.findDomain() expected error but got domain: %+v", got)
			}
		})
	}
}
