package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/rackerlabs/external-dns-rackspace-webhook/internal/handlers"
	"github.com/rackerlabs/external-dns-rackspace-webhook/internal/providers"
	"github.com/rackerlabs/external-dns-rackspace-webhook/internal/routes"
)

const (
	defaultPort             = 2020
	defaultIdentityEndpoint = "https://identity.api.rackspacecloud.com/v2.0/"
)

func main() {
	config := loadConfig()
	setupLogging(config.LogLevel)
	provider, err := providers.NewRackspaceProvider(config)
	if err != nil {
		log.Fatalf("Failed to create Rackspace provider: %v", err)
	}
	handler := handlers.NewHandler(provider)
	e := echo.New()
	e.HideBanner = true
	routes.ConfigureRoutes(e, handler)

	port, err := getStartPort()
	if err != nil {
		log.Fatalf("invalid port %s", err)
	}

	if err = e.Start(fmt.Sprintf(":%d", port)); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

func getStartPort() (int, error) {
	portStr := os.Getenv("PORT")
	if portStr == "" {
		return defaultPort, nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port format: %s", portStr)
	}
	return port, nil
}

func loadConfig() *providers.RackspaceConfig {
	config := &providers.RackspaceConfig{
		Username:         strings.TrimSpace(os.Getenv("RACKSPACE_USERNAME")),
		APIKey:           strings.TrimSpace(os.Getenv("RACKSPACE_API_KEY")),
		IdentityEndpoint: strings.TrimSpace(os.Getenv("RACKSPACE_IDENTITY_ENDPOINT")),
		DryRun:           false,
		LogLevel:         "info",
	}

	if domainFilter := os.Getenv("DOMAIN_FILTER"); domainFilter != "" {
		config.DomainFilter = strings.Split(domainFilter, ",")
	}

	if dryRun := os.Getenv("DRY_RUN"); dryRun == "true" {
		config.DryRun = true
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.LogLevel = logLevel
	}

	if config.IdentityEndpoint == "" {
		config.IdentityEndpoint = defaultIdentityEndpoint
	}

	// Validate required fields
	if config.Username == "" {
		log.Fatal("RACKSPACE_USERNAME is required and cannot be empty")
	}
	if config.APIKey == "" {
		log.Fatal("RACKSPACE_API_KEY is required and cannot be empty")
	}
	if _, err := url.Parse(config.IdentityEndpoint); err != nil {
		log.Fatalf("Invalid RACKSPACE_IDENTITY_ENDPOINT URL: %v", err)
	}

	return config
}

func setupLogging(level string) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("Log level set to: %s", level)
}
