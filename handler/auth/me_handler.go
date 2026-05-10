package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/api"
	"strings"
)

func MeHandler(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract token from Authorization header
		tokenString := c.Get("Authorization")
		if len(tokenString) == 0 {
			return api.Unauthorized(c)
		}
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
		// Parse and validate JWT
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
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
		if !ok {
			return api.Unauthorized(c)
		}

		// Return user details from the token
		userDetails := fiber.Map{
			"userId":      claims["userId"],
			"firstNameTh": claims["firstNameTh"],
			"lastNameTh":  claims["lastNameTh"],
			"appCode":     claims["appCode"],
			"companyCode": claims["companyCode"],
			"accountName": claims["accountName"],
			"status":      claims["status"],
			"roles":       claims["roles"],
		}
		response := map[string]interface{}{
			"jwtBody": userDetails,
		}
		return api.Ok(c, response)
	}
}
