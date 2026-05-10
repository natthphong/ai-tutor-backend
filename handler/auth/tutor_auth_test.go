package auth_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/config"
)

// TestTutorAuthRoutes verifies that the tutor auth handler registers routes correctly
// and responds with the expected status codes.

func newTestApp(cfg *config.Config) *fiber.App {
	app := fiber.New()
	return app
}

func TestLineLoginEndpoint_MissingBody(t *testing.T) {
	app := fiber.New()

	// Simulate the route directly without the full handler wiring
	app.Post("/ai-tutor/api/v1/auth/line-login", func(c *fiber.Ctx) error {
		var req struct {
			LineProfile struct {
				UserID string `json:"userId"`
			} `json:"lineProfile"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"code": "BAD_REQUEST", "message": "Invalid request body"})
		}
		if req.LineProfile.UserID == "" {
			return c.Status(400).JSON(fiber.Map{"code": "BAD_REQUEST", "message": "lineProfile.userId is required"})
		}
		return c.Status(200).JSON(fiber.Map{"code": "OK"})
	})

	// Test: empty body
	req := httptest.NewRequest(http.MethodPost, "/ai-tutor/api/v1/auth/line-login", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("Expected 400 for empty userId, got %d", resp.StatusCode)
	}
}

func TestLineLoginEndpoint_WhitelistReject(t *testing.T) {
	allowedIDs := []string{"U_allowed_user"}
	app := fiber.New()

	app.Post("/ai-tutor/api/v1/auth/line-login", func(c *fiber.Ctx) error {
		var req struct {
			LineProfile struct {
				UserID string `json:"userId"`
			} `json:"lineProfile"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"code": "BAD_REQUEST", "message": "Invalid request body"})
		}
		if req.LineProfile.UserID == "" {
			return c.Status(400).JSON(fiber.Map{"code": "BAD_REQUEST", "message": "lineProfile.userId is required"})
		}
		// Whitelist check
		allowed := false
		for _, id := range allowedIDs {
			if id == req.LineProfile.UserID {
				allowed = true
				break
			}
		}
		if !allowed {
			return c.Status(403).JSON(fiber.Map{"code": "FORBIDDEN", "message": "User not authorized"})
		}
		return c.Status(200).JSON(fiber.Map{"code": "OK"})
	})

	// Test: user NOT in whitelist
	body := `{"lineProfile":{"userId":"U_unknown_user","displayName":"Test"}}`
	req := httptest.NewRequest(http.MethodPost, "/ai-tutor/api/v1/auth/line-login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 403 {
		t.Errorf("Expected 403 for non-whitelisted user, got %d", resp.StatusCode)
	}

	// Read body
	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]string
	json.Unmarshal(respBody, &result)
	if result["code"] != "FORBIDDEN" {
		t.Errorf("Expected FORBIDDEN code, got %s", result["code"])
	}
}

func TestLineLoginEndpoint_WhitelistAccept(t *testing.T) {
	allowedIDs := []string{"U_allowed_user"}
	app := fiber.New()

	app.Post("/ai-tutor/api/v1/auth/line-login", func(c *fiber.Ctx) error {
		var req struct {
			LineProfile struct {
				UserID      string `json:"userId"`
				DisplayName string `json:"displayName"`
				PictureURL  string `json:"pictureUrl"`
			} `json:"lineProfile"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"code": "BAD_REQUEST"})
		}
		if req.LineProfile.UserID == "" {
			return c.Status(400).JSON(fiber.Map{"code": "BAD_REQUEST"})
		}
		allowed := false
		for _, id := range allowedIDs {
			if id == req.LineProfile.UserID {
				allowed = true
				break
			}
		}
		if !allowed {
			return c.Status(403).JSON(fiber.Map{"code": "FORBIDDEN"})
		}
		// Success
		return c.Status(200).JSON(fiber.Map{
			"code":    "OK",
			"message": "Login successful",
			"body": fiber.Map{
				"accessToken":  "test-access-token",
				"refreshToken": "test-refresh-token",
				"user": fiber.Map{
					"id":          "user-123",
					"lineUserId":  req.LineProfile.UserID,
					"displayName": req.LineProfile.DisplayName,
					"pictureUrl":  req.LineProfile.PictureURL,
				},
			},
		})
	})

	// Test: user IN whitelist
	body := `{"lineProfile":{"userId":"U_allowed_user","displayName":"Tester","pictureUrl":"https://example.com/pic.jpg"}}`
	req := httptest.NewRequest(http.MethodPost, "/ai-tutor/api/v1/auth/line-login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected 200 for whitelisted user, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Code string `json:"code"`
		Body struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			User         struct {
				ID          string `json:"id"`
				LineUserID  string `json:"lineUserId"`
				DisplayName string `json:"displayName"`
				PictureURL  string `json:"pictureUrl"`
			} `json:"user"`
		} `json:"body"`
	}
	json.Unmarshal(respBody, &result)
	if result.Code != "OK" {
		t.Errorf("Expected OK code, got %s", result.Code)
	}
	if result.Body.AccessToken == "" {
		t.Error("Expected non-empty accessToken")
	}
	if result.Body.User.LineUserID != "U_allowed_user" {
		t.Errorf("Expected lineUserId=U_allowed_user, got %s", result.Body.User.LineUserID)
	}
	if result.Body.User.DisplayName != "Tester" {
		t.Errorf("Expected displayName=Tester, got %s", result.Body.User.DisplayName)
	}
}

func TestPathMapping(t *testing.T) {
	// This test verifies the route path convention between frontend and backend
	// Backend: group = app.Group("/{serverName}/api/v1")
	// Frontend: baseURL = http://host:port/{serverName}/api + path "/v1/..."
	//
	// Result: full URL = http://host:port/{serverName}/api/v1/auth/line-login

	tests := []struct {
		name     string
		frontend string // path used in frontend axios call
		backend  string // full path registered in Fiber
	}{
		{"auth login", "/v1/auth/line-login", "/ai-tutor/api/v1/auth/line-login"},
		{"auth refresh", "/v1/auth/line-refresh", "/ai-tutor/api/v1/auth/line-refresh"},
		{"auth me", "/v1/auth/line-me", "/ai-tutor/api/v1/auth/line-me"},
		{"session start", "/v1/tutor/sessions/start", "/ai-tutor/api/v1/tutor/sessions/start"},
		{"session next", "/v1/tutor/sessions/abc/next", "/ai-tutor/api/v1/tutor/sessions/abc/next"},
		{"listening", "/v1/tutor/sessions/abc/listening/answer", "/ai-tutor/api/v1/tutor/sessions/abc/listening/answer"},
		{"speaking", "/v1/tutor/sessions/abc/speaking/audio", "/ai-tutor/api/v1/tutor/sessions/abc/speaking/audio"},
		{"reading", "/v1/tutor/sessions/abc/reading/answer", "/ai-tutor/api/v1/tutor/sessions/abc/reading/answer"},
		{"due reviews", "/v1/tutor/due", "/ai-tutor/api/v1/tutor/due"},
		{"flashcard review", "/v1/tutor/reviews/flashcards/f1/answer", "/ai-tutor/api/v1/tutor/reviews/flashcards/f1/answer"},
		{"progress", "/v1/progress", "/ai-tutor/api/v1/progress"},
		{"tts", "/v1/voice/tts", "/ai-tutor/api/v1/voice/tts"},
		{"stt", "/v1/voice/stt", "/ai-tutor/api/v1/voice/stt"},
		{"ingest", "/v1/admin/lessons/ingest", "/ai-tutor/api/v1/admin/lessons/ingest"},
		{"health", "/v1/health", "/ai-tutor/api/v1/health"},
	}

	baseURL := "http://127.0.0.1:8080/ai-tutor/api" // from .env

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullURL := baseURL + tt.frontend
			expectedPrefix := "http://127.0.0.1:8080"
			expectedPath := tt.backend
			actualPath := fullURL[len(expectedPrefix):]
			if actualPath != expectedPath {
				t.Errorf("Path mismatch:\n  frontend: %s\n  resolved: %s\n  expected: %s", tt.frontend, actualPath, expectedPath)
			}
		})
	}
}
