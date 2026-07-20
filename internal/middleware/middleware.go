package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func Register(app *fiber.App) {
	app.Use(logger.New())
	app.Use(recover.New())
}
