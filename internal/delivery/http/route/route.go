package route

import "github.com/gofiber/fiber/v2"

type RouteConfig struct {
	App *fiber.App
}

// Setup registers all HTTP routes. It is intentionally empty until the
// first domain controller exists — add routes here following the pattern
// documented in PROJECT_STRUCTURE.md.
func (c *RouteConfig) Setup() {
}
