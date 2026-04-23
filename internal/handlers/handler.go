package handlers

import (
	"encoding/json"
	"net/http"

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

// HandleAdjustEndpoints returns endpoints unchanged. Rackspace does not
// require any provider-specific canonicalization. Transforming endpoints
// here (adding trailing dots, modifying TTL, stripping TXT quotes) caused
// mismatches with what Records() returns and with the TXT registry's
// internal representation, leading to constant update churn.
func (h *Handler) HandleAdjustEndpoints(c echo.Context) error {
	defer c.Request().Body.Close()
	var endpoints []*endpoint.Endpoint
	if err := json.NewDecoder(c.Request().Body).Decode(&endpoints); err != nil {
		log.Error("Failed to decode input", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
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
