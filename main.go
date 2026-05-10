package main

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"gitlab.com/home-server7795544/home-server/iam/iam-backend/api"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/config"
	handler "gitlab.com/home-server7795544/home-server/iam/iam-backend/handler/tutor"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/internal/ai"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/internal/db"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/internal/logz"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/internal/tutor"
)

func main() {
	currentTime := time.Now()
	versionDeploy := currentTime.Unix()
	ctx := context.Background()
	config.InitTimeZone()

	cfg, err := config.InitConfig()
	if err != nil {
		panic("Unable to initial config: " + err.Error())
	}

	logz.Init(cfg.LogConfig.Level, cfg.Server.Name)
	defer logz.Drop()
	logger := zap.L()
	logger.Info("AI Tutor Loop starting", zap.Int64("version", versionDeploy))

	// Database
	dbPool, err := db.Open(ctx, cfg.DBConfig)
	if err != nil {
		logger.Fatal("DB connect failed", zap.Error(err))
	}
	defer dbPool.Close()
	logger.Info("DB connected")

	// AI Router (Gemini → OpenRouter → OpenAI fallback)
	aiRouter := ai.NewRouter(cfg)

	// Tutor Services
	tutorSvc := tutor.NewService(dbPool, aiRouter)
	ingestSvc := tutor.NewIngestService(dbPool)

	// Fiber App
	app := initFiber()
	group := app.Group(fmt.Sprintf("/%s/api/v1", cfg.Server.Name))

	// Health
	group.Get("/health", func(c *fiber.Ctx) error {
		return api.OkWithMessage(c, "Service is healthy", fiber.Map{"status": "ok", "version": versionDeploy})
	})

	// Metrics
	group.Get("/metric", metrics())

	// Tutor Handler (registers all tutor routes)
	tutorHandler := handler.NewTutorHandler(tutorSvc, ingestSvc, aiRouter, cfg)
	tutorHandler.Register(group)

	// LINE Webhook placeholder
	group.Post("/line/webhook", func(c *fiber.Ctx) error {
		return api.Ok(c, fiber.Map{"received": true})
	})

	// Config info (allowed user IDs)
	_, _ = json.Marshal(cfg.Line.AllowedUserIDs)
	logger.Info(fmt.Sprintf("/%s/api/v1", cfg.Server.Name), zap.Any("port", cfg.Server.Port))
	logger.Info("Allowed LINE users", zap.Strings("userIds", cfg.Line.AllowedUserIDs))

	if err = app.Listen(fmt.Sprintf(":%v", cfg.Server.Port)); err != nil {
		logger.Fatal(err.Error())
	}
}

func initFiber() *fiber.App {
	app := fiber.New(fiber.Config{
		ReadTimeout:           30 * time.Second,
		WriteTimeout:          30 * time.Second,
		IdleTimeout:           60 * time.Second,
		DisableStartupMessage: false,
		CaseSensitive:         true,
		StrictRouting:         false,
		BodyLimit:             50 * 1024 * 1024, // 50MB for audio uploads
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization,traceId,requestId",
	}))
	app.Use(SetHeaderID())
	return app
}

func SetHeaderID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		traceId := c.Get("traceId")
		if traceId == "" {
			traceId = uuid.New().String()
		}
		c.Accepts(fiber.MIMEApplicationJSON)
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		c.Request().Header.Set("traceId", traceId)
		return c.Next()
	}
}

func metrics() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		return api.Ok(c, map[string]interface{}{
			"memory": map[string]interface{}{
				"alloc":      fmt.Sprintf("%.2f MB", float64(mem.Alloc)/1024/1024),
				"totalAlloc": fmt.Sprintf("%.2f MB", float64(mem.TotalAlloc)/1024/1024),
				"sysAlloc":   fmt.Sprintf("%.2f MB", float64(mem.Sys)/1024/1024),
			},
		})
	}
}

// unused but kept for compatibility
func toMB(b uint64) string {
	return fmt.Sprintf("%.2f MB", float64(b)/(1024*1024))
}

func init() {
	_ = strconv.Itoa(0) // keep import
}
