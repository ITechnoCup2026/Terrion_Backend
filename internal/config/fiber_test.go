package config

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestNewFiber_ErrorHandlerFormatsFiberErrors(t *testing.T) {
	cfg := &Config{}
	cfg.App.Name = "test-app"
	cfg.Web.Prefork = false

	app := NewFiber(cfg)
	app.Get("/boom", func(ctx *fiber.Ctx) error {
		return fiber.ErrBadRequest
	})

	req := httptest.NewRequest("GET", "/boom", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusBadRequest)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "errors") {
		t.Errorf("body = %q, want it to contain %q", string(body), "errors")
	}
}
