package providers

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/rackerlabs/goclouddns/records"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"

	th "github.com/gophercloud/gophercloud/v2/testhelper"
)

func TestConvertRecordToEndpoint_NoTrailingDot(t *testing.T) {
	tests := []struct {
		name    string
		record  records.RecordList
		wantDNS string
	}{
		{
			name:    "A record returned as-is",
			record:  records.RecordList{Name: "myhost.example.com", Type: "A", Data: "10.0.0.1", TTL: 300},
			wantDNS: "myhost.example.com",
		},
		{
			name:    "TXT record returned as-is",
			record:  records.RecordList{Name: "a-myhost.example.com", Type: "TXT", Data: "heritage=external-dns", TTL: 300},
			wantDNS: "a-myhost.example.com",
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
			wantTarget: "10 0 27017 node1.example.com.",
		},
		{
			name: "zero priority",
			record: records.RecordList{
				Name: "_sip._tcp.svc.example.com", Type: "SRV",
				Data: "5 5060 sip.example.com", TTL: 300, Priority: 0,
			},
			wantTarget: "0 5 5060 sip.example.com.",
		},
		{
			name: "high priority value",
			record: records.RecordList{
				Name: "_http._tcp.web.example.com", Type: "SRV",
				Data: "1 443 web.example.com", TTL: 300, Priority: 100,
			},
			wantTarget: "100 1 443 web.example.com.",
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

func TestConvertRecordToEndpoint_TXTDataUnmodified(t *testing.T) {
	tests := []struct {
		name       string
		data       string
		wantTarget string
	}{
		{
			name:       "heritage string returned as-is",
			data:       "heritage=external-dns,external-dns/owner=test",
			wantTarget: "heritage=external-dns,external-dns/owner=test",
		},
		{
			name:       "SPF record returned as-is",
			data:       "v=spf1 include:example.com ~all",
			wantTarget: "v=spf1 include:example.com ~all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := records.RecordList{Name: "a-myhost.example.com", Type: "TXT", Data: tt.data, TTL: 300}
			ep := convertRecordToEndpoint(rec, "example.com")
			if ep == nil {
				t.Fatal("convertRecordToEndpoint returned nil")
			}
			if ep.Targets[0] != tt.wantTarget {
				t.Errorf("TXT target = %q, want %q", ep.Targets[0], tt.wantTarget)
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
			comment:    `{"ownedRecord":"app.example.com."}`,
			wantLabels: map[string]string{"ownedRecord": "app.example.com."},
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

func TestRecords_MergesMultipleTargets(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/domains", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"domains":[{"id":"111","name":"example.com"}]}`)
	})
	th.Mux.HandleFunc("/domains/111/records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"records":[
			{"id":"r1","name":"multi.example.com","type":"A","data":"10.0.0.1","ttl":300},
			{"id":"r2","name":"multi.example.com","type":"A","data":"10.0.0.2","ttl":300},
			{"id":"r3","name":"multi.example.com","type":"A","data":"10.0.0.3","ttl":300},
			{"id":"r4","name":"single.example.com","type":"A","data":"10.0.0.4","ttl":300}
		]}`)
	})

	p := newTestProvider(t)
	eps, err := p.Records(context.Background())
	if err != nil {
		t.Fatalf("Records() error: %v", err)
	}
	if len(eps) != 2 {
		t.Fatalf("expected 2 merged endpoints, got %d", len(eps))
	}

	for _, ep := range eps {
		if ep.DNSName == "multi.example.com" {
			if len(ep.Targets) != 3 {
				t.Errorf("expected 3 targets for multi.example.com, got %d: %v", len(ep.Targets), ep.Targets)
			}
		}
		if ep.DNSName == "single.example.com" {
			if len(ep.Targets) != 1 {
				t.Errorf("expected 1 target for single.example.com, got %d", len(ep.Targets))
			}
		}
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

func TestApplyChanges_DryRun(t *testing.T) {
	p := &RackspaceProvider{
		DryRun:       true,
		DomainFilter: endpoint.NewDomainFilter([]string{"example.com"}),
	}
	changes := &plan.Changes{
		Create: []*endpoint.Endpoint{
			{DNSName: "new.example.com", RecordType: "A", Targets: []string{"10.0.0.1"}, RecordTTL: 300},
		},
	}
	if err := p.ApplyChanges(context.Background(), changes); err != nil {
		t.Errorf("ApplyChanges with DryRun should not error, got: %v", err)
	}
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
