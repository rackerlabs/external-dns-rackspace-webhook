package routes

import (
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/rackerlabs/external-dns-rackspace-webhook/internal/handlers"
	"github.com/rackerlabs/external-dns-rackspace-webhook/internal/middleware"
)

func ConfigureRoutes(e *echo.Echo, h *handlers.Handler) {
	e.Pre(echoMiddleware.RemoveTrailingSlash())
	e.Use(middleware.ExternalDNSContentTypeMiddleware)

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
