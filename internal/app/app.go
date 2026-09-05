package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"os"
	"strings"
	"time"
	"tokoloop/contracts"
	"tokoloop/internal/config"
	"tokoloop/internal/content"
	"tokoloop/internal/gemini"
	"tokoloop/internal/security"
	"tokoloop/internal/storage"
)

type App struct {
	DB   *pgxpool.Pool
	Cfg  config.Config
	AI   *gemini.Client
	HTTP *fiber.App
}
type User struct {
	ID         string         `json:"id"`
	Username   string         `json:"username"`
	Role       string         `json:"role"`
	MustChange bool           `json:"must_change_password"`
	Profile    map[string]any `json:"profile"`
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	db, e := storage.Open(ctx, cfg.DatabaseURL)
	if e != nil {
		return nil, e
	}
	if e = storage.Migrate(ctx, db); e != nil {
		db.Close()
		return nil, e
	}
	a := &App{DB: db, Cfg: cfg, AI: gemini.New(cfg)}
	if e = a.seed(ctx); e != nil {
		db.Close()
		return nil, e
	}
	if e = a.refreshReviewCues(ctx, ""); e != nil {
		db.Close()
		return nil, e
	}
	if e = os.MkdirAll(cfg.AudioDir, 0700); e != nil {
		return nil, e
	}
	a.routes()
	return a, nil
}
func (a *App) seed(ctx context.Context) error {
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(7214903)"); e != nil {
		return e
	}
	var n int
	if e = tx.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&n); e != nil {
		return e
	}
	if n == 0 {
		_, e = tx.Exec(ctx, "INSERT INTO users(id,username,password_hash,role,must_change_password) VALUES($1,'admin',$2,'admin',true)", uuid.NewString(), security.Hash("password"))
		if e != nil {
			return e
		}
	}
	for _, l := range content.Lessons() {
		b, _ := json.Marshal(l)
		if _, e = tx.Exec(ctx, "INSERT INTO lessons(id,ordinal,data) VALUES($1,$2,$3) ON CONFLICT(id) DO UPDATE SET data=excluded.data,ordinal=excluded.ordinal", l.ID, l.Ordinal, b); e != nil {
			return e
		}
	}
	for _, s := range content.Scenarios() {
		b, _ := json.Marshal(s)
		if _, e = tx.Exec(ctx, "INSERT INTO scenarios(id,data) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET data=excluded.data", s.ID, b); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
func fail(c *fiber.Ctx, status int, msg string) error {
	return c.Status(status).JSON(fiber.Map{"error": msg})
}
func (a *App) routes() {
	f := fiber.New(fiber.Config{BodyLimit: 10 << 20, ReadTimeout: 30 * time.Second, WriteTimeout: 90 * time.Second, ErrorHandler: func(c *fiber.Ctx, e error) error {
		var fe *fiber.Error
		if errors.As(e, &fe) {
			return fail(c, fe.Code, fe.Message)
		}
		slog.Error("request failed", "path", c.Path(), "error", e)
		return fail(c, 500, "ระบบขัดข้อง กรุณาลองใหม่")
	}})
	a.HTTP = f
	f.Use(recover.New())
	f.Use(cors.New(cors.Config{AllowOrigins: strings.Join(a.Cfg.Origins, ","), AllowHeaders: "Origin, Content-Type, Authorization, X-Request-ID", AllowMethods: "GET,POST,PATCH,DELETE,OPTIONS"}))
	f.Use(func(c *fiber.Ctx) error {
		c.Set("Cache-Control", "no-store")
		c.Set("X-Content-Type-Options", "nosniff")
		return c.Next()
	})
	g := f.Group("/ai-tutor/api/v2")
	g.Get("/openapi.json", func(c *fiber.Ctx) error { c.Type("json"); return c.Send(contracts.OpenAPI) })
	g.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok", "release": a.Cfg.Version}) })
	g.Get("/readiness", func(c *fiber.Ctx) error {
		if e := a.DB.Ping(c.UserContext()); e != nil {
			return fail(c, 503, "database unavailable")
		}
		return c.JSON(fiber.Map{"status": "ready", "ai_configured": a.Cfg.GeminiKey != ""})
	})
	g.Post("/auth/login", limiter.New(limiter.Config{Max: 12, Expiration: time.Minute}), a.login)
	g.Post("/auth/register", limiter.New(limiter.Config{Max: 6, Expiration: time.Minute}), a.register)
	a.liveRoute(g)
	g.Use(a.authenticate)
	g.Get("/auth/me", func(c *fiber.Ctx) error { return c.JSON(user(c)) })
	g.Post("/auth/logout", a.logout)
	g.Post("/auth/change-password", a.changePassword)
	g.Use(func(c *fiber.Ctx) error {
		if user(c).MustChange {
			return fail(c, 403, "กรุณาเปลี่ยนรหัสผ่านเริ่มต้นก่อนใช้งาน")
		}
		return c.Next()
	})
	g.Post("/admin/invitations", a.createInvitation)
	g.Get("/admin/invitations", a.listInvitations)
	g.Delete("/admin/invitations/:id", a.revokeInvitation)
	g.Patch("/profile", a.profile)
	g.Get("/curriculum", a.curriculum)
	g.Get("/lessons/:id", a.lesson)
	g.Get("/daily-plan", a.daily)
	g.Get("/progress", a.progress)
	g.Get("/library", a.library)
	g.Post("/vocabulary", a.saveWord)
	g.Get("/scenarios", a.scenarios)
	g.Post("/scenarios", a.createScenario)
	g.Patch("/scenarios/:id", a.editScenario)
	g.Post("/sessions", a.createSession)
	g.Get("/sessions", a.sessions)
	g.Get("/sessions/:id", a.getSession)
	g.Post("/sessions/:id/turns", a.submitTurn)
	g.Post("/sessions/:id/hints", a.hint)
	g.Post("/sessions/:id/advance", a.advance)
	g.Post("/sessions/:id/complete", a.completeSession)
	g.Post("/sessions/:id/retry", a.retrySession)
	g.Post("/sessions/:id/live-ticket", a.liveTicket)
	g.Get("/review", a.reviews)
	g.Post("/review/:id/answer", a.reviewAnswer)
	g.Post("/review/:id/hint", a.reviewHint)
	g.Post("/audio/tts", a.ttsJob)
	g.Get("/audio/:id", a.audio)
	g.Get("/jobs/:id", a.job)
	g.Get("/usage", a.usage)
}
func user(c *fiber.Ctx) User { return c.Locals("user").(User) }
func (a *App) authenticate(c *fiber.Ctx) error {
	h := c.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return fail(c, 401, "กรุณาเข้าสู่ระบบ")
	}
	var u User
	var b []byte
	e := a.DB.QueryRow(c.UserContext(), "SELECT u.id::text,u.username,u.role,u.must_change_password,u.profile FROM users u JOIN auth_sessions s ON s.user_id=u.id WHERE s.token_hash=$1 AND s.expires_at>now()", security.Digest(strings.TrimPrefix(h, "Bearer "))).Scan(&u.ID, &u.Username, &u.Role, &u.MustChange, &b)
	if e == pgx.ErrNoRows {
		return fail(c, 401, "เซสชันหมดอายุ กรุณาเข้าสู่ระบบ")
	}
	if e != nil {
		return e
	}
	if e = json.Unmarshal(b, &u.Profile); e != nil {
		return e
	}
	c.Locals("user", u)
	return c.Next()
}
func validID(id string) bool { _, e := uuid.Parse(id); return e == nil }
func asJSON(v any) []byte    { b, _ := json.Marshal(v); return b }
func textValue(v any) string { s, _ := v.(string); return s }
func number(v any, def float64) float64 {
	f, ok := v.(float64)
	if !ok {
		return def
	}
	return f
}
