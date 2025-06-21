package main

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/mohammadanang/uploads-api/handler"
)

func main() {
	app := fiber.New()
	app.Use(cors.New())
	// 3 requests per 10 seconds max
	app.Use(limiter.New(limiter.Config{
		Expiration: 10 * time.Second,
		Max:        3,
	}))
	app.Use(logger.New(logger.Config{
		Format: "${time} | ${status} | ${method} | ${path} | ${latency}\n",
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	apiHandler := handler.NewAPIHandler()
	app.Post("/upload-file", apiHandler.UploadFile)
	app.Post("/merge-chunk", apiHandler.MergeChunks)

	// Define an error handler
	app.Use(func(c *fiber.Ctx) error {
		if err := recover(); err != nil {
			// Handle the error and respond with an error message
			return c.Status(500).SendString("Internal Server Error")
		}

		return c.Next()
	})

	// Start the server
	log.Fatal(app.Listen(":3000"))
}
