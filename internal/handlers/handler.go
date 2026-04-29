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

	log.Info("GET /records", "count", len(endpoints))
	return c.JSON(http.StatusOK, endpoints)
}

// HandleAdjustEndpoints normalises provider-specific endpoint details.
// For SRV records, RFC 2782 requires the target host to be an absolute
// FQDN (trailing dot).  Sources that omit the dot would be rejected by
// external-dns's ValidateSRVRecord, so we append it here as a safety net.
func (h *Handler) HandleAdjustEndpoints(c echo.Context) error {
	defer c.Request().Body.Close()
	var endpoints []*endpoint.Endpoint
	if err := json.NewDecoder(c.Request().Body).Decode(&endpoints); err != nil {
		log.Error("Failed to decode input", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	for _, ep := range endpoints {
		if ep.RecordType == "SRV" {
			for i, target := range ep.Targets {
				parts := strings.SplitN(target, " ", 4)
				if len(parts) == 4 && !strings.HasSuffix(parts[3], ".") {
					ep.Targets[i] = parts[0] + " " + parts[1] + " " + parts[2] + " " + parts[3] + "."
				}
			}
		}
	}

	log.Info("POST /adjustendpoints", "count", len(endpoints))
	return c.JSON(http.StatusOK, endpoints)
}

func (h *Handler) HandlePostRecords(c echo.Context) error {
	defer c.Request().Body.Close()
	var changes plan.Changes
	if err := json.NewDecoder(c.Request().Body).Decode(&changes); err != nil {
		log.Error("Failed to decode input", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	log.Info("POST /records", "create", len(changes.Create), "updateNew", len(changes.UpdateNew), "delete", len(changes.Delete))
	if err := h.provider.ApplyChanges(c.Request().Context(), &changes); err != nil {
		log.Error("Failed to apply changes", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}
