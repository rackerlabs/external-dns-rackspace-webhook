package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestHealthHandler(t *testing.T) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Create handler instance
	h := &Handler{}

	// Execute
	err := h.HealthHandler(c)

	// Assertions
	if err != nil {
		t.Errorf("HealthHandler() returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}

	expectedContentType := "application/external.dns.webhook+json;version=1"
	actualContentType := rec.Header().Get("Content-Type")
	if actualContentType != expectedContentType {
		t.Errorf("Expected Content-Type %q, got %q", expectedContentType, actualContentType)
	}

	// Parse and verify JSON response
	var response map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse JSON response: %v", err)
	}

	expectedStatus := "ok"
	if response["status"] != expectedStatus {
		t.Errorf("Expected status %q, got %q", expectedStatus, response["status"])
	}
}

func TestHealthHandler_ResponseFormat(t *testing.T) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := &Handler{}

	// Execute
	err := h.HealthHandler(c)

	// Assertions
	if err != nil {
		t.Fatalf("HealthHandler() returned unexpected error: %v", err)
	}

	// Verify HTTP status code
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}

	// Verify Content-Type header
	expectedContentType := "application/external.dns.webhook+json;version=1"
	actualContentType := rec.Header().Get("Content-Type")
	if actualContentType != expectedContentType {
		t.Errorf("Content-Type header mismatch. Expected: %q, Got: %q", expectedContentType, actualContentType)
	}

	// Verify JSON response structure
	body := strings.TrimSpace(rec.Body.String())
	if !strings.Contains(body, `"status"`) || !strings.Contains(body, `"ok"`) {
		t.Errorf("Response body should contain status field with 'ok' value. Got: %s", body)
	}

	// Verify it's valid JSON
	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Errorf("Response should be valid JSON. Error: %v, Body: %s", err, body)
	}

	// Verify response contains only expected fields
	if len(response) != 1 {
		t.Errorf("Response should contain exactly 1 field, got %d fields: %v", len(response), response)
	}
}
