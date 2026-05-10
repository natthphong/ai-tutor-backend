package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

func Register(app fiber.Router, dbPool *pgxpool.Pool, jwtSecret string, accessTokenDuration, refreshTokenDuration time.Duration) {

	authGroup := app.Group("/auth")
	authGroup.Post("/login", LoginHandler(dbPool, jwtSecret, accessTokenDuration, refreshTokenDuration))
	authGroup.Get("/me", MeHandler(jwtSecret))
	authGroup.Post("/refreshToken", RefreshTokenHandler(dbPool, jwtSecret, accessTokenDuration, refreshTokenDuration))
}
