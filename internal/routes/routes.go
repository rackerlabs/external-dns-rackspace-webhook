package routes

import (
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/rackerlabs/external-dns-rackspace-webhook/internal/handlers"
	"github.com/rackerlabs/external-dns-rackspace-webhook/internal/middleware"
)

// ConfigureWebhookRoutes sets up the external-dns webhook API routes (localhost only).
func ConfigureWebhookRoutes(e *echo.Echo, h *handlers.Handler) {
	e.Use(echoMiddleware.Recover())
	e.Pre(echoMiddleware.RemoveTrailingSlash())
	e.Use(middleware.ExternalDNSContentTypeMiddleware)

	// Domain filter negotiation endpoint
	e.GET("/", h.NegotiationHandler)
	// Get all DNS records
	e.GET("/records", h.HandleGetRecords)
	// Apply DNS record changes
	e.POST("/records", h.HandlePostRecords)
	// Adjust endpoints (optional preprocessing)
	e.POST("/adjustendpoints", h.HandleAdjustEndpoints)
}

// ConfigureOpsRoutes sets up operational endpoints exposed on all interfaces.
func ConfigureOpsRoutes(e *echo.Echo, h *handlers.Handler) {
	e.Use(echoMiddleware.Recover())
	e.GET("/healthz", h.HealthHandler)
}
