package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/rackerlabs/external-dns-rackspace-webhook/internal/handlers"
)

func ConfigureRoutes(e *echo.Echo, h *handlers.Handler) {
	// Health check endpoint
	e.GET("/healthz", h.HealthHandler)

	// Domain filter negotiation endpoint
	e.GET("/", h.NegotiationHandler)
	// Get all DNS records
	e.GET("/records", h.HandleGetRecords)
	// Apply DNS record changes
	e.POST("/records", h.HandlePostRecords)
	// Adjust endpoints (optional preprocessing)
	e.POST("/adjustendpoints", h.HandleAdjustEndpoints)
}
