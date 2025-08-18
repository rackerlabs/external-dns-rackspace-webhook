package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/labstack/echo/v4"
	"github.com/rackerlabs/external-dns-rackspace-webhook/internal/providers"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
)

type Handler struct {
	provider *providers.RackspaceProvider
}

func NewHandler(rackspaceProvider *providers.RackspaceProvider) *Handler {
	return &Handler{
		provider: rackspaceProvider,
	}
}

func (h *Handler) NegotiationHandler(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/external.dns.webhook+json;version=1")
	return c.JSON(http.StatusOK, h.provider.DomainFilter)
}

func (h *Handler) HandleGetRecords(c echo.Context) error {
	endpoints, err := h.provider.Records(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch records"})
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/external.dns.webhook+json;version=1")
	return c.JSON(http.StatusOK, endpoints)
}

func (h *Handler) HandleAdjustEndpoints(c echo.Context) error {
	var endpoints []*endpoint.Endpoint
	b, err := io.ReadAll(c.Request().Body)
	if err != nil {
		log.Error("Failed to decode input", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to decode input"})
	}
	if err := json.Unmarshal(b, &endpoints); err != nil {
		log.Error("Failed to decode endpoints", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to decode endpoints"})
	}
	log.Debug("Adjusting endpoints", "count", len(endpoints))
	adjusted := make([]*endpoint.Endpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		if ep == nil || ep.DNSName == "" || len(ep.Targets) == 0 {
			log.Warn("Skipping invalid endpoint", "dnsName", ep.DNSName)
			continue
		}

		dnsName := strings.ToLower(strings.TrimSuffix(ep.DNSName, ".") + ".")
		if !h.provider.DomainFilter.Match(dnsName) {
			log.Warn("Endpoint outside domain filter", "dnsName", dnsName)
			continue
		}

		if ep.RecordType == "NS" || ep.RecordType == "SOA" {
			log.Warn("Skipping unsupported record type", "dnsName", dnsName, "recordType", ep.RecordType)
			continue
		}

		ttl := ep.RecordTTL
		if ttl.IsConfigured() && ttl < 300 {
			log.Debug("Adjusting TTL to 300s", "dnsName", dnsName, "originalTTL", ttl)
			ttl = 300
		}

		adjusted = append(adjusted, &endpoint.Endpoint{
			DNSName:          dnsName,
			Targets:          ep.Targets,
			RecordType:       ep.RecordType,
			RecordTTL:        ttl,
			ProviderSpecific: ep.ProviderSpecific,
		})
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/external.dns.webhook+json;version=1")
	return c.JSON(http.StatusOK, adjusted)
}

func (h *Handler) HandlePostRecords(c echo.Context) error {
	var changes plan.Changes
	b, err := io.ReadAll(c.Request().Body)
	if err != nil {
		log.Error("Failed to decode input", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to decode input"})
	}
	if err := json.Unmarshal(b, &changes); err != nil {
		log.Error("Failed to decode endpoints", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to decode endpoints"})
	}

	if err := h.provider.ApplyChanges(c.Request().Context(), &changes); err != nil {
		log.Error("Failed to apply changes", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to apply changes"})
	}

	return c.NoContent(http.StatusNoContent)
}
