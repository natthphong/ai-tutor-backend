package app

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"regexp"
	"strings"
	"time"
	"tokoloop/internal/security"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{2,39}$`)

func (a *App) login(c *fiber.Ctx) error {
	var p struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if c.BodyParser(&p) != nil || len(p.Password) > 256 {
		return fail(c, 400, "ข้อมูลเข้าสู่ระบบไม่ถูกต้อง")
	}
	var id, hash string
	e := a.DB.QueryRow(c.UserContext(), "SELECT id::text,password_hash FROM users WHERE username=$1", strings.ToLower(strings.TrimSpace(p.Username))).Scan(&id, &hash)
	if e != nil || !security.Verify(hash, p.Password) {
		return fail(c, 401, "ชื่อผู้ใช้หรือรหัสผ่านไม่ถูกต้อง")
	}
	t := security.Token()
	_, e = a.DB.Exec(c.UserContext(), "INSERT INTO auth_sessions(token_hash,user_id,expires_at) VALUES($1,$2,$3)", security.Digest(t), id, time.Now().Add(30*24*time.Hour))
	if e != nil {
		return e
	}
	return c.JSON(fiber.Map{"token": t, "expires_in": 30 * 86400})
}
func (a *App) register(c *fiber.Ctx) error {
	var p struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		Invitation string `json:"invitation"`
	}
	if c.BodyParser(&p) != nil {
		return fail(c, 400, "ข้อมูลไม่ถูกต้อง")
	}
	p.Username = strings.ToLower(strings.TrimSpace(p.Username))
	if !usernamePattern.MatchString(p.Username) || len(p.Password) < 10 || len(p.Password) > 256 {
		return fail(c, 400, "ชื่อผู้ใช้ 3–40 ตัวอักษร a-z/0-9 และรหัสผ่านอย่างน้อย 10 ตัวอักษร")
	}
	tx, e := a.DB.Begin(c.UserContext())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.UserContext())
	var iid string
	e = tx.QueryRow(c.UserContext(), "SELECT id::text FROM invitations WHERE token_hash=$1 AND expires_at>now() AND used_by IS NULL AND NOT revoked FOR UPDATE", security.Digest(strings.TrimSpace(p.Invitation))).Scan(&iid)
	if e == pgx.ErrNoRows {
		return fail(c, 400, "รหัสเชิญใช้ไม่ได้หรือหมดอายุ")
	}
	if e != nil {
		return e
	}
	id := uuid.NewString()
	_, e = tx.Exec(c.UserContext(), "INSERT INTO users(id,username,password_hash) VALUES($1,$2,$3)", id, p.Username, security.Hash(p.Password))
	if e != nil {
		return fail(c, 409, "ชื่อผู้ใช้นี้ถูกใช้แล้ว")
	}
	if _, e = tx.Exec(c.UserContext(), "UPDATE invitations SET used_by=$1 WHERE id=$2", id, iid); e != nil {
		return e
	}
	if e = tx.Commit(c.UserContext()); e != nil {
		return e
	}
	return c.Status(201).JSON(fiber.Map{"id": id})
}
func (a *App) logout(c *fiber.Ctx) error {
	_, e := a.DB.Exec(c.UserContext(), "DELETE FROM auth_sessions WHERE token_hash=$1", security.Digest(strings.TrimPrefix(c.Get("Authorization"), "Bearer ")))
	if e != nil {
		return e
	}
	return c.JSON(fiber.Map{"ok": true})
}
func (a *App) changePassword(c *fiber.Ctx) error {
	var p struct {
		Current  string `json:"current"`
		Password string `json:"password"`
	}
	if c.BodyParser(&p) != nil || len(p.Password) < 10 || len(p.Password) > 256 || p.Password == p.Current {
		return fail(c, 400, "รหัสใหม่ต้องต่างจากเดิมและยาวอย่างน้อย 10 ตัวอักษร")
	}
	u := user(c)
	var h string
	if e := a.DB.QueryRow(c.UserContext(), "SELECT password_hash FROM users WHERE id=$1", u.ID).Scan(&h); e != nil {
		return e
	}
	if !security.Verify(h, p.Current) {
		return fail(c, 400, "รหัสผ่านปัจจุบันไม่ถูกต้อง")
	}
	tx, e := a.DB.Begin(c.UserContext())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.UserContext())
	if _, e = tx.Exec(c.UserContext(), "UPDATE users SET password_hash=$1,must_change_password=false WHERE id=$2", security.Hash(p.Password), u.ID); e != nil {
		return e
	}
	if _, e = tx.Exec(c.UserContext(), "DELETE FROM auth_sessions WHERE user_id=$1 AND token_hash<>$2", u.ID, security.Digest(strings.TrimPrefix(c.Get("Authorization"), "Bearer "))); e != nil {
		return e
	}
	if e = tx.Commit(c.UserContext()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"ok": true})
}
func (a *App) createInvitation(c *fiber.Ctx) error {
	if user(c).Role != "admin" {
		return fail(c, 403, "สำหรับผู้ดูแลเท่านั้น")
	}
	t, id := security.Token(), uuid.NewString()
	expires := time.Now().AddDate(0, 0, 7)
	_, e := a.DB.Exec(c.UserContext(), "INSERT INTO invitations(id,token_hash,created_by,expires_at) VALUES($1,$2,$3,$4)", id, security.Digest(t), user(c).ID, expires)
	if e != nil {
		return e
	}
	return c.Status(201).JSON(fiber.Map{"id": id, "code": t, "expires_at": expires})
}
func (a *App) listInvitations(c *fiber.Ctx) error {
	if user(c).Role != "admin" {
		return fail(c, 403, "สำหรับผู้ดูแลเท่านั้น")
	}
	return a.jsonRows(c, "SELECT coalesce(jsonb_agg(jsonb_build_object('id',id,'expires_at',expires_at,'used',used_by IS NOT NULL,'revoked',revoked) ORDER BY created_at DESC),'[]'::jsonb) FROM invitations WHERE created_by=$1", user(c).ID)
}
func (a *App) revokeInvitation(c *fiber.Ctx) error {
	if user(c).Role != "admin" {
		return fail(c, 403, "สำหรับผู้ดูแลเท่านั้น")
	}
	if !validID(c.Params("id")) {
		return fail(c, 404, "ไม่พบรหัส")
	}
	_, e := a.DB.Exec(c.UserContext(), "UPDATE invitations SET revoked=true WHERE id=$1 AND created_by=$2", c.Params("id"), user(c).ID)
	if e != nil {
		return e
	}
	return c.JSON(fiber.Map{"ok": true})
}
func (a *App) profile(c *fiber.Ctx) error {
	var p map[string]any
	if c.BodyParser(&p) != nil {
		return fail(c, 400, "ข้อมูลไม่ถูกต้อง")
	}
	allowed := map[string]bool{"daily_minutes": true, "thai_support": true, "voice": true, "speed": true, "monthly_budget": true, "live_minutes": true, "onboarded": true, "goal": true}
	for k := range p {
		if !allowed[k] {
			delete(p, k)
		}
	}
	if v, ok := p["monthly_budget"]; ok && (number(v, 0) < 1 || number(v, 0) > 1000) {
		return fail(c, 400, "งบต้องอยู่ระหว่าง 1–1,000 บาท")
	}
	if v, ok := p["live_minutes"]; ok && (number(v, 0) < 1 || number(v, 0) > 60) {
		return fail(c, 400, "เวลา Live ต้องอยู่ระหว่าง 1–60 นาที")
	}
	if v, ok := p["speed"]; ok && (number(v, 0) < .5 || number(v, 0) > 1.5) {
		return fail(c, 400, "ความเร็วไม่ถูกต้อง")
	}
	if v, ok := p["daily_minutes"]; ok && (number(v, 0) != 15 && number(v, 0) != 30 && number(v, 0) != 60) {
		return fail(c, 400, "เลือก 15, 30 หรือ 60 นาที")
	}
	if v, ok := p["voice"]; ok && textValue(v) != "Kore" && textValue(v) != "Puck" && textValue(v) != "Aoede" {
		return fail(c, 400, "เสียงไม่ถูกต้อง")
	}
	for _, k := range []string{"thai_support", "onboarded"} {
		if v, ok := p[k]; ok {
			if _, ok = v.(bool); !ok {
				return fail(c, 400, "ข้อมูลไม่ถูกต้อง")
			}
		}
	}
	if len(textValue(p["goal"])) > 1000 {
		return fail(c, 400, "เป้าหมายยาวเกินไป")
	}
	_, e := a.DB.Exec(c.UserContext(), "UPDATE users SET profile=profile || $1::jsonb WHERE id=$2", asJSON(p), user(c).ID)
	if e != nil {
		return e
	}
	return c.JSON(fiber.Map{"ok": true})
}
