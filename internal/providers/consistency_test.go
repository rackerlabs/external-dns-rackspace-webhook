package providers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rackerlabs/goclouddns/records"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"

	th "github.com/gophercloud/gophercloud/v2/testhelper"
)

func TestConvertRecordToEndpoint_TrailingDot(t *testing.T) {
	tests := []struct {
		name    string
		record  records.RecordList
		wantDNS string
	}{
		{
			name:    "A record without trailing dot",
			record:  records.RecordList{Name: "myhost.example.com", Type: "A", Data: "10.0.0.1", TTL: 300},
			wantDNS: "myhost.example.com.",
		},
		{
			name:    "TXT record without trailing dot",
			record:  records.RecordList{Name: "a-myhost.example.com", Type: "TXT", Data: "heritage=external-dns", TTL: 300},
			wantDNS: "a-myhost.example.com.",
		},
		{
			name:    "already has trailing dot",
			record:  records.RecordList{Name: "myhost.example.com.", Type: "A", Data: "10.0.0.1", TTL: 300},
			wantDNS: "myhost.example.com.",
		},
		{
			name:    "SRV record without trailing dot",
			record:  records.RecordList{Name: "_mongodb._tcp.myrs.example.com", Type: "SRV", Data: "0 27017 node1.example.com", TTL: 300, Priority: 10},
			wantDNS: "_mongodb._tcp.myrs.example.com.",
		},
		{
			name:    "CNAME record without trailing dot",
			record:  records.RecordList{Name: "alias.example.com", Type: "CNAME", Data: "target.example.com", TTL: 300},
			wantDNS: "alias.example.com.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep := convertRecordToEndpoint(tt.record, "example.com")
			if ep == nil {
				t.Fatal("convertRecordToEndpoint returned nil")
			}
			if ep.DNSName != tt.wantDNS {
				t.Errorf("DNSName = %q, want %q", ep.DNSName, tt.wantDNS)
			}
		})
	}
}

func TestConvertRecordToEndpoint_SRVFormat(t *testing.T) {
	tests := []struct {
		name       string
		record     records.RecordList
		wantTarget string
	}{
		{
			name: "priority prepended to 3-part data",
			record: records.RecordList{
				Name: "_mongodb._tcp.myrs.example.com", Type: "SRV",
				Data: "0 27017 node1.example.com", TTL: 300, Priority: 10,
			},
			wantTarget: "10 0 27017 node1.example.com",
		},
		{
			name: "zero priority",
			record: records.RecordList{
				Name: "_sip._tcp.svc.example.com", Type: "SRV",
				Data: "5 5060 sip.example.com", TTL: 300, Priority: 0,
			},
			wantTarget: "0 5 5060 sip.example.com",
		},
		{
			name: "high priority value",
			record: records.RecordList{
				Name: "_http._tcp.web.example.com", Type: "SRV",
				Data: "1 443 web.example.com", TTL: 300, Priority: 100,
			},
			wantTarget: "100 1 443 web.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep := convertRecordToEndpoint(tt.record, "example.com")
			if ep == nil {
				t.Fatal("convertRecordToEndpoint returned nil")
			}
			if ep.Targets[0] != tt.wantTarget {
				t.Errorf("SRV target = %q, want %q", ep.Targets[0], tt.wantTarget)
			}
		})
	}
}

func TestConvertRecordToEndpoint_SkipsNSAndSOA(t *testing.T) {
	tests := []struct {
		name       string
		recordType string
	}{
		{name: "NS record", recordType: "NS"},
		{name: "SOA record", recordType: "SOA"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := records.RecordList{Name: "example.com", Type: tt.recordType, Data: "ns1.example.com", TTL: 300}
			ep := convertRecordToEndpoint(rec, "example.com")
			if ep != nil {
				t.Errorf("expected nil for %s record, got %+v", tt.recordType, ep)
			}
		})
	}
}

func TestConvertRecordToEndpoint_TXTLabels(t *testing.T) {
	tests := []struct {
		name       string
		comment    string
		wantLabels map[string]string
	}{
		{
			name:       "valid JSON labels in comment",
			comment:    `{"external-dns/owner":"test-owner"}`,
			wantLabels: map[string]string{"external-dns/owner": "test-owner"},
		},
		{
			name:       "empty comment",
			comment:    "",
			wantLabels: nil,
		},
		{
			name:       "invalid JSON comment",
			comment:    "not-json",
			wantLabels: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := records.RecordList{Name: "txt.example.com", Type: "TXT", Data: "v=spf1", TTL: 300, Comment: tt.comment}
			ep := convertRecordToEndpoint(rec, "example.com")
			if ep == nil {
				t.Fatal("convertRecordToEndpoint returned nil")
			}
			if tt.wantLabels == nil && ep.Labels != nil {
				t.Errorf("expected nil labels, got %v", ep.Labels)
			}
			for k, v := range tt.wantLabels {
				if ep.Labels[k] != v {
					t.Errorf("label %q = %q, want %q", k, ep.Labels[k], v)
				}
			}
		})
	}
}

// TestRecordsAdjustEndpointsIdempotent verifies the core invariant: passing
// Records() output through AdjustEndpoints logic must produce identical results.
// A mismatch here means external-dns sees "changes" every sync → churning.
func TestRecordsAdjustEndpointsIdempotent(t *testing.T) {
	tests := []struct {
		name          string
		apiRecords    string // JSON records array from Rackspace
		wantEndpoints int
	}{
		{
			name: "A and TXT records",
			apiRecords: `[
				{"id":"r1","name":"myhost.example.com","type":"A","data":"10.0.0.1","ttl":300},
				{"id":"r2","name":"a-myhost.example.com","type":"TXT","data":"heritage=external-dns,external-dns/owner=test","ttl":300}
			]`,
			wantEndpoints: 2,
		},
		{
			name: "SRV record with separate priority",
			apiRecords: `[
				{"id":"r3","name":"_mongodb._tcp.myrs.example.com","type":"SRV","data":"0 27017 node1.example.com","ttl":300,"priority":10}
			]`,
			wantEndpoints: 1,
		},
		{
			name: "mixed record types",
			apiRecords: `[
				{"id":"r1","name":"myhost.example.com","type":"A","data":"10.0.0.1","ttl":300},
				{"id":"r2","name":"a-myhost.example.com","type":"TXT","data":"heritage=external-dns","ttl":300},
				{"id":"r3","name":"_mongodb._tcp.myrs.example.com","type":"SRV","data":"0 27017 node1.example.com","ttl":300,"priority":10},
				{"id":"r4","name":"alias.example.com","type":"CNAME","data":"target.example.com","ttl":600}
			]`,
			wantEndpoints: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th.SetupHTTP()
			defer th.TeardownHTTP()

			th.Mux.HandleFunc("/domains", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"domains":[{"id":"111","name":"example.com"}]}`)
			})
			th.Mux.HandleFunc("/domains/111/records", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"records":%s}`, tt.apiRecords)
			})

			p := newTestProvider(t)
			eps, err := p.Records(context.Background())
			if err != nil {
				t.Fatalf("Records() error: %v", err)
			}
			if len(eps) != tt.wantEndpoints {
				t.Fatalf("Records() returned %d endpoints, want %d", len(eps), tt.wantEndpoints)
			}

			adjusted := simulateAdjustEndpoints(eps, p.DomainFilter)
			if len(adjusted) != len(eps) {
				t.Fatalf("AdjustEndpoints returned %d, Records returned %d", len(adjusted), len(eps))
			}

			for i := range eps {
				if eps[i].DNSName != adjusted[i].DNSName {
					t.Errorf("[%d] DNSName mismatch: Records=%q Adjusted=%q", i, eps[i].DNSName, adjusted[i].DNSName)
				}
				for j := range eps[i].Targets {
					if eps[i].Targets[j] != adjusted[i].Targets[j] {
						t.Errorf("[%d] Target[%d] mismatch: Records=%q Adjusted=%q", i, j, eps[i].Targets[j], adjusted[i].Targets[j])
					}
				}
				if eps[i].RecordTTL != adjusted[i].RecordTTL {
					t.Errorf("[%d] TTL mismatch: Records=%d Adjusted=%d", i, eps[i].RecordTTL, adjusted[i].RecordTTL)
				}
			}
		})
	}
}

func TestRecords_FiltersNSAndSOA(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/domains", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"domains":[{"id":"111","name":"example.com"}]}`)
	})
	th.Mux.HandleFunc("/domains/111/records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"records":[
			{"id":"r1","name":"example.com","type":"NS","data":"ns1.example.com","ttl":3600},
			{"id":"r2","name":"example.com","type":"SOA","data":"ns1.example.com admin.example.com","ttl":3600},
			{"id":"r3","name":"myhost.example.com","type":"A","data":"10.0.0.1","ttl":300}
		]}`)
	})

	p := newTestProvider(t)
	eps, err := p.Records(context.Background())
	if err != nil {
		t.Fatalf("Records() error: %v", err)
	}
	if len(eps) != 1 {
		t.Fatalf("expected 1 endpoint (NS/SOA filtered), got %d", len(eps))
	}
	if eps[0].RecordType != "A" {
		t.Errorf("expected A record, got %s", eps[0].RecordType)
	}
}

func TestRecords_SkipsNonMatchingDomains(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/domains", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"domains":[
			{"id":"111","name":"example.com"},
			{"id":"222","name":"other.com"}
		]}`)
	})
	th.Mux.HandleFunc("/domains/111/records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"records":[{"id":"r1","name":"myhost.example.com","type":"A","data":"10.0.0.1","ttl":300}]}`)
	})

	p := newTestProvider(t)
	eps, err := p.Records(context.Background())
	if err != nil {
		t.Fatalf("Records() error: %v", err)
	}
	if len(eps) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(eps))
	}
}

func TestApplyChanges_DryRun(t *testing.T) {
	p := &RackspaceProvider{
		DryRun:       true,
		DomainFilter: endpoint.NewDomainFilter([]string{"example.com"}),
	}
	changes := &plan.Changes{
		Create: []*endpoint.Endpoint{
			{DNSName: "new.example.com.", RecordType: "A", Targets: []string{"10.0.0.1"}, RecordTTL: 300},
		},
	}
	if err := p.ApplyChanges(context.Background(), changes); err != nil {
		t.Errorf("ApplyChanges with DryRun should not error, got: %v", err)
	}
}

// simulateAdjustEndpoints replicates HandleAdjustEndpoints logic for test assertions.
func simulateAdjustEndpoints(endpoints []*endpoint.Endpoint, df *endpoint.DomainFilter) []*endpoint.Endpoint {
	var out []*endpoint.Endpoint
	for _, ep := range endpoints {
		if ep == nil || ep.DNSName == "" || len(ep.Targets) == 0 {
			continue
		}
		dnsName := strings.ToLower(strings.TrimSuffix(ep.DNSName, ".")) + "."
		if !df.Match(dnsName) {
			continue
		}
		if ep.RecordType == "NS" || ep.RecordType == "SOA" {
			continue
		}
		ttl := ep.RecordTTL
		if !ttl.IsConfigured() {
			ttl = endpoint.TTL(300)
		} else if ttl < 300 {
			ttl = endpoint.TTL(300)
		}
		targets := make([]string, 0, len(ep.Targets))
		for _, t := range ep.Targets {
			if ep.RecordType == "TXT" {
				t = strings.Trim(t, `"`)
			}
			targets = append(targets, t)
		}
		out = append(out, &endpoint.Endpoint{
			DNSName:    dnsName,
			Targets:    targets,
			RecordType: ep.RecordType,
			RecordTTL:  ttl,
			Labels:     ep.Labels,
		})
	}
	return out
}

func newTestProvider(t *testing.T) *RackspaceProvider {
	t.Helper()
	return &RackspaceProvider{
		serviceClient: NewRackspaceDNSClient(FakeDNSClient()),
		authProvider:  NewRackspaceAuthProvider(),
		tokenExpiry:   time.Now().Add(24 * time.Hour),
		config: &RackspaceConfig{
			IdentityEndpoint: th.Endpoint(),
			Username:         "test",
			APIKey:           "test",
		},
		DomainFilter: endpoint.NewDomainFilter([]string{"example.com"}),
		DryRun:       false,
	}
}
