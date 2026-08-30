package route

import (
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRouteConfig_SetupDoesNotPanic(t *testing.T) {
	cfg := &RouteConfig{App: fiber.New()}

	cfg.Setup()
}
