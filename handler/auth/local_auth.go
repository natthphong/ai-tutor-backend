package auth

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"gitlab.com/home-server7795544/home-server/iam/iam-backend/api"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/config"
)

// LocalAuthHandler exposes username/password authentication endpoints so the
// app can be tested without LINE LIFF. The local user is a row in tutor_users
// with auth_kind = 'local' and a bcrypt password hash.
type LocalAuthHandler struct {
	db     *pgxpool.Pool
	cfg    *config.Config
	logger *zap.Logger
}

func NewLocalAuthHandler(db *pgxpool.Pool, cfg *config.Config) *LocalAuthHandler {
	return &LocalAuthHandler{db: db, cfg: cfg, logger: zap.L()}
}

// RegisterLocalAuth mounts /auth/register, /auth/login, /auth/me, /auth/logout.
// These coexist with the existing /auth/line-* endpoints.
func RegisterLocalAuth(group fiber.Router, db *pgxpool.Pool, cfg *config.Config) {
	h := NewLocalAuthHandler(db, cfg)
	g := group.Group("/auth")
	g.Post("/register", h.Register())
	g.Post("/login", h.Login())
	g.Get("/me", TutorJWTMiddleware(), h.Me())
	g.Post("/logout", TutorJWTMiddleware(), h.Logout())
}

type registerReq struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

func (h *LocalAuthHandler) Register() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req registerReq
		if err := c.BodyParser(&req); err != nil {
			return api.BadRequest(c, "Invalid request body")
		}
		req.Username = strings.TrimSpace(strings.ToLower(req.Username))
		if req.Username == "" || len(req.Password) < 6 {
			return api.BadRequest(c, "username and password (>=6 chars) are required")
		}
		if req.DisplayName == "" {
			req.DisplayName = req.Username
		}

		var existing string
		err := h.db.QueryRow(c.Context(), `SELECT id FROM tutor_users WHERE username = $1`, req.Username).Scan(&existing)
		if err == nil {
			return c.Status(fiber.StatusConflict).JSON(api.Err("USERNAME_TAKEN", "username already exists"))
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return api.InternalError(c, "hash error")
		}

		userID := uuid.New().String()
		_, err = h.db.Exec(c.Context(),
			`INSERT INTO tutor_users (id, username, display_name, auth_kind, password_hash)
			 VALUES ($1, $2, $3, 'local', $4)`,
			userID, req.Username, req.DisplayName, string(hash))
		if err != nil {
			return api.InternalError(c, "create user: "+err.Error())
		}

		access, _ := generateTutorToken(userID, "local:"+req.Username, req.DisplayName, "", 24*time.Hour)
		refresh, _ := generateTutorToken(userID, "local:"+req.Username, req.DisplayName, "", 30*24*time.Hour)
		return api.OkWithMessage(c, "Registered", fiber.Map{
			"accessToken":  access,
			"refreshToken": refresh,
			"user": fiber.Map{
				"id":          userID,
				"lineUserId":  "local:" + req.Username,
				"username":    req.Username,
				"displayName": req.DisplayName,
				"authKind":    "local",
			},
		})
	}
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *LocalAuthHandler) Login() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req loginReq
		if err := c.BodyParser(&req); err != nil {
			return api.BadRequest(c, "Invalid request body")
		}
		req.Username = strings.TrimSpace(strings.ToLower(req.Username))
		if req.Username == "" || req.Password == "" {
			return api.BadRequest(c, "username and password are required")
		}

		var (
			userID   string
			pwHash   string
			display  string
			authKind string
		)
		err := h.db.QueryRow(c.Context(),
			`SELECT id, COALESCE(password_hash,''), COALESCE(display_name,''), COALESCE(auth_kind,'local')
			   FROM tutor_users WHERE username = $1`, req.Username).
			Scan(&userID, &pwHash, &display, &authKind)
		if err != nil || pwHash == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(api.Err("INVALID_CREDENTIALS", "Invalid username or password"))
		}
		if bcrypt.CompareHashAndPassword([]byte(pwHash), []byte(req.Password)) != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(api.Err("INVALID_CREDENTIALS", "Invalid username or password"))
		}

		_, _ = h.db.Exec(c.Context(),
			`UPDATE tutor_users SET last_active_at = now() WHERE id = $1`, userID)

		access, _ := generateTutorToken(userID, "local:"+req.Username, display, "", 24*time.Hour)
		refresh, _ := generateTutorToken(userID, "local:"+req.Username, display, "", 30*24*time.Hour)
		return api.OkWithMessage(c, "Login successful", fiber.Map{
			"accessToken":  access,
			"refreshToken": refresh,
			"user": fiber.Map{
				"id":          userID,
				"lineUserId":  "local:" + req.Username,
				"username":    req.Username,
				"displayName": display,
				"authKind":    authKind,
			},
		})
	}
}

func (h *LocalAuthHandler) Me() fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims := c.Locals("tutorClaims").(*TutorClaims)
		var username, authKind string
		_ = h.db.QueryRow(c.Context(),
			`SELECT COALESCE(username,''), COALESCE(auth_kind,'')
			   FROM tutor_users WHERE id = $1`, claims.UserID).Scan(&username, &authKind)
		return api.OkWithMessage(c, "User info", fiber.Map{
			"user": fiber.Map{
				"id":          claims.UserID,
				"lineUserId":  claims.LineUserID,
				"username":    username,
				"displayName": claims.DisplayName,
				"pictureUrl":  claims.PictureURL,
				"authKind":    authKind,
			},
		})
	}
}

func (h *LocalAuthHandler) Logout() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Tokens are stateless JWTs – nothing to invalidate server-side.
		// We return OK so the frontend can clear local storage.
		return api.OkWithMessage(c, "Logged out", fiber.Map{"ok": true})
	}
}

// EnsureLocalSeed creates the dev test user if missing. Safe to call at
// startup; runs no migration logic, just guarantees the password is set.
func EnsureLocalSeed(ctx context.Context, db *pgxpool.Pool) {
	var id string
	err := db.QueryRow(ctx, `SELECT id FROM tutor_users WHERE username = 'test'`).Scan(&id)
	if err == nil {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("test1234"), bcrypt.DefaultCost)
	if err != nil {
		return
	}
	_, _ = db.Exec(ctx,
		`INSERT INTO tutor_users (id, username, display_name, auth_kind, password_hash)
		 VALUES ($1, 'test', 'Test User', 'local', $2)
		 ON CONFLICT (username) DO NOTHING`, uuid.New().String(), string(hash))
}
