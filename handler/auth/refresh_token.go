package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/api"
	"time"
)

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"` // Refresh token from the client
}

func RefreshTokenHandler(
	db *pgxpool.Pool,
	jwtSecret string,
	accessTokenDuration, refreshTokenDuration time.Duration,
) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req RefreshTokenRequest
		if err := c.BodyParser(&req); err != nil {
			return api.BadRequest(c, "Invalid input")
		}

		// Verify the refresh token
		token, err := jwt.Parse(req.RefreshToken, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.ErrUnauthorized
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			return api.Unauthorized(c)
		}

		// Extract claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || claims["userId"] == nil || claims["appCode"] == nil {
			return api.Unauthorized(c)
		}

		userID, ok1 := claims["userId"].(string)
		appCode, ok2 := claims["appCode"].(string)
		if !ok1 || !ok2 {
			return api.Unauthorized(c)
		}

		// Call GenerateJWTForUser to generate new tokens
		response, err := GenerateJWTForUser(db, userID, "", appCode, jwtSecret, accessTokenDuration, refreshTokenDuration, true)
		if err != nil {
			return err
		}

		return api.Ok(c, response)
	}
}
