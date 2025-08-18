package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *Handler) HealthHandler(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/external.dns.webhook+json;version=1")
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
