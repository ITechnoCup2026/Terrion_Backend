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

func TestNewFiber_CorsAllowsCredentials(t *testing.T) {
	cfg := &Config{}
	cfg.App.Name = "test-app"
	cfg.Web.CorsOrigins = "http://localhost:3000"

	app := NewFiber(cfg)
	app.Get("/ping", func(ctx *fiber.Ctx) error { return ctx.SendString("pong") })

	req := httptest.NewRequest("GET", "/ping", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want %q", got, "true")
	}
}
