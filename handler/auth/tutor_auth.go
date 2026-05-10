package auth

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/api"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/config"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/internal/tutor"
	"go.uber.org/zap"
)

var tutorJwtSecret = []byte("ai-tutor-loop-secret-key-2026")

type TutorAuthHandler struct {
	svc    *tutor.Service
	cfg    *config.Config
	logger *zap.Logger
}

func NewTutorAuthHandler(svc *tutor.Service, cfg *config.Config) *TutorAuthHandler {
	return &TutorAuthHandler{svc: svc, cfg: cfg, logger: zap.L()}
}

func RegisterTutorAuth(app fiber.Router, svc *tutor.Service, cfg *config.Config) {
	h := NewTutorAuthHandler(svc, cfg)
	authGroup := app.Group("/auth")
	authGroup.Post("/line-login", h.LineLogin())
	authGroup.Post("/line-refresh", h.LineRefresh())
	authGroup.Get("/line-me", TutorJWTMiddleware(), h.LineMe())
}

// LineLogin checks LINE userId against whitelist, creates user, returns JWT
func (h *TutorAuthHandler) LineLogin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req struct {
			LineProfile struct {
				UserID      string `json:"userId"`
				DisplayName string `json:"displayName"`
				PictureURL  string `json:"pictureUrl"`
			} `json:"lineProfile"`
		}
		if err := c.BodyParser(&req); err != nil {
			return api.BadRequest(c, "Invalid request body")
		}
		lineUserID := req.LineProfile.UserID
		if lineUserID == "" {
			return api.BadRequest(c, "lineProfile.userId is required")
		}

		// Check whitelist
		allowed := false
		for _, id := range h.cfg.Line.AllowedUserIDs {
			if id == lineUserID {
				allowed = true
				break
			}
		}
		if !allowed {
			h.logger.Warn("Login rejected: not in whitelist", zap.String("lineUserId", lineUserID))
			return c.Status(fiber.StatusForbidden).JSON(api.Err("FORBIDDEN", "User not authorized"))
		}

		// Ensure user in DB
		userID, err := h.svc.EnsureUser(c.Context(), lineUserID, req.LineProfile.DisplayName)
		if err != nil {
			return api.InternalError(c, "Failed to create user: "+err.Error())
		}

		accessToken, _ := generateTutorToken(userID, lineUserID, req.LineProfile.DisplayName, req.LineProfile.PictureURL, 24*time.Hour)
		refreshToken, _ := generateTutorToken(userID, lineUserID, req.LineProfile.DisplayName, req.LineProfile.PictureURL, 30*24*time.Hour)

		return api.OkWithMessage(c, "Login successful", fiber.Map{
			"accessToken":  accessToken,
			"refreshToken": refreshToken,
			"user": fiber.Map{
				"id":          userID,
				"lineUserId":  lineUserID,
				"displayName": req.LineProfile.DisplayName,
				"pictureUrl":  req.LineProfile.PictureURL,
			},
		})
	}
}

func (h *TutorAuthHandler) LineRefresh() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req struct {
			RefreshToken string `json:"refreshToken"`
		}
		if err := c.BodyParser(&req); err != nil {
			return api.BadRequest(c, "Invalid request body")
		}
		claims, err := validateTutorToken(req.RefreshToken)
		if err != nil {
			return api.JwtError(c, "Invalid refresh token")
		}
		accessToken, _ := generateTutorToken(claims.UserID, claims.LineUserID, claims.DisplayName, claims.PictureURL, 24*time.Hour)
		refreshToken, _ := generateTutorToken(claims.UserID, claims.LineUserID, claims.DisplayName, claims.PictureURL, 30*24*time.Hour)

		return api.OkWithMessage(c, "Token refreshed", fiber.Map{
			"accessToken":  accessToken,
			"refreshToken": refreshToken,
		})
	}
}

func (h *TutorAuthHandler) LineMe() fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims := c.Locals("tutorClaims").(*TutorClaims)
		return api.OkWithMessage(c, "User info", fiber.Map{
			"user": fiber.Map{
				"id":          claims.UserID,
				"lineUserId":  claims.LineUserID,
				"displayName": claims.DisplayName,
				"pictureUrl":  claims.PictureURL,
			},
		})
	}
}

// --- JWT Types ---

type TutorClaims struct {
	UserID      string `json:"userId"`
	LineUserID  string `json:"lineUserId"`
	DisplayName string `json:"displayName"`
	PictureURL  string `json:"pictureUrl"`
	jwt.RegisteredClaims
}

func generateTutorToken(userID, lineUserID, displayName, pictureURL string, duration time.Duration) (string, error) {
	claims := TutorClaims{
		UserID: userID, LineUserID: lineUserID, DisplayName: displayName, PictureURL: pictureURL,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "ai-tutor-loop",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(tutorJwtSecret)
}

func validateTutorToken(tokenStr string) (*TutorClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &TutorClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return tutorJwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*TutorClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func TutorJWTMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		if auth == "" || len(auth) < 8 {
			return api.JwtError(c, "Missing authorization")
		}
		claims, err := validateTutorToken(auth[7:])
		if err != nil {
			return api.JwtError(c, "Invalid or expired token")
		}
		c.Locals("tutorClaims", claims)
		c.Locals("userId", claims.UserID)
		c.Locals("lineUserId", claims.LineUserID)
		return c.Next()
	}
}
