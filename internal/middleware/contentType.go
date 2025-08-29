// File: middleware/content_type.go
package middleware

import (
	"github.com/labstack/echo/v4"
)

// ExternalDNSContentType is the required media type for ExternalDNS webhook responses.
const ExternalDNSContentType = "application/external.dns.webhook+json;version=1"

// ExternalDNSContentTypeMiddleware ensures the Content-Type header is set correctly
func ExternalDNSContentTypeMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set(echo.HeaderContentType, ExternalDNSContentType)
		err := next(c)
		return err
	}
}
