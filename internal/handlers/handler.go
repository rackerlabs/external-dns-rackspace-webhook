package handlers

import (
	"encoding/json"
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
	return c.JSON(http.StatusOK, h.provider.DomainFilter)
}

func (h *Handler) HandleGetRecords(c echo.Context) error {
	endpoints, err := h.provider.Records(c.Request().Context())
	if err != nil {
		log.Error("Failed to get records", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, endpoints)
}

func (h *Handler) HandleAdjustEndpoints(c echo.Context) error {
	defer c.Request().Body.Close()
	var endpoints []*endpoint.Endpoint
	if err := json.NewDecoder(c.Request().Body).Decode(&endpoints); err != nil {
		log.Error("Failed to decode input", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	log.Debug("Adjusting endpoints", "count", len(endpoints))
	adjusted := make([]*endpoint.Endpoint, 0, len(endpoints))

	for _, ep := range endpoints {
		if ep == nil {
			log.Warn("Skipping nil endpoint")
			continue
		}
		if ep == nil || ep.DNSName == "" || len(ep.Targets) == 0 {
			log.Warn("Skipping invalid endpoint", "dnsName", ep.DNSName)
			continue
		}
		// Canonicalize DNS name
		dnsName := strings.ToLower(strings.TrimSuffix(ep.DNSName, ".")) + "."
		if !h.provider.DomainFilter.Match(dnsName) {
			log.Warn("Endpoint outside domain filter", "dnsName", dnsName)
			continue
		}

		// Skip unsupported record types
		if ep.RecordType == "NS" || ep.RecordType == "SOA" {
			log.Warn("Skipping unsupported record type", "dnsName", dnsName, "recordType", ep.RecordType)
			continue
		}

		// Normalize TTL: ensure it’s at least 300, default 300 if not set
		ttl := ep.RecordTTL
		if !ttl.IsConfigured() {
			ttl = endpoint.TTL(300)
		} else if ttl < 300 {
			log.Debug("Adjusting TTL to 300s", "dnsName", dnsName, "originalTTL", ttl)
			ttl = endpoint.TTL(300)
		}

		// Normalize TXT targets (Rackspace may return them already quoted)
		targets := make([]string, 0, len(ep.Targets))
		for _, t := range ep.Targets {
			if ep.RecordType == "TXT" {
				t = strings.Trim(t, `"`)
			}
			targets = append(targets, t)
		}

		adjusted = append(adjusted, &endpoint.Endpoint{
			DNSName:    dnsName,
			Targets:    targets,
			RecordType: ep.RecordType,
			RecordTTL:  ttl,
			Labels:     ep.Labels,
		})
	}

	return c.JSON(http.StatusOK, adjusted)
}

func (h *Handler) HandlePostRecords(c echo.Context) error {
	defer c.Request().Body.Close()
	var changes plan.Changes
	if err := json.NewDecoder(c.Request().Body).Decode(&changes); err != nil {
		log.Error("Failed to decode input", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if err := h.provider.ApplyChanges(c.Request().Context(), &changes); err != nil {
		log.Error("Failed to apply changes", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}
