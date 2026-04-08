package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/rackerlabs/external-dns-rackspace-webhook/internal/handlers"
	"github.com/rackerlabs/external-dns-rackspace-webhook/internal/providers"
	"github.com/rackerlabs/external-dns-rackspace-webhook/internal/routes"
)

const (
	//This is the internal port external-dns talks to the webhook on must always
	//scoped to localhost:<PORT> but the port is configurable via the external-dns chart
	defaultPort = 8888
	//external-dns hardcodes the public healthz/readyz/metrics port in the chart to 8080
	//this can't change unless the chart changes
	defaultHealthzPort      = 8080
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

	port, err := getStartPort()
	if err != nil {
		log.Fatalf("invalid port %s", err)
	}

	// Webhook API server — localhost only
	webhook := echo.New()
	webhook.HideBanner = true
	routes.ConfigureWebhookRoutes(webhook, handler)

	// Ops server — all interfaces
	ops := echo.New()
	ops.HideBanner = true
	routes.ConfigureOpsRoutes(ops, handler)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 2)

	// Always scope to localhost:<port>
	go func() { errCh <- webhook.Start(fmt.Sprintf("localhost:%d", port)) }()

	// Expose healthz/readyz/metrics endpoints publicly
	go func() { errCh <- ops.Start(fmt.Sprintf(":%d", defaultHealthzPort)) }()

	exitCode := 0
	select {
	case err = <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
			exitCode = 1
		}
	case <-ctx.Done():
		log.Println("shutdown signal received")
	}
	stop()

	// Allow in-flight requests to drain before K8s sends SIGKILL (default 30s grace).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	if err := webhook.Shutdown(shutdownCtx); err != nil {
		log.Printf("webhook shutdown error: %v", err)
	}
	if err := ops.Shutdown(shutdownCtx); err != nil {
		log.Printf("ops shutdown error: %v", err)
	}
	os.Exit(exitCode)
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
