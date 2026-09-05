package app

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"tokoloop/internal/gemini"
	"tokoloop/internal/learning"
	"tokoloop/internal/security"
)

type Input struct {
	RequestID string `json:"request_id"`
	Text      string `json:"text"`
	Kind      string `json:"kind"`
	RetryOf   string `json:"retry_of"`
	HintLevel int    `json:"hint_level"`
}

func (a *App) readInput(c *fiber.Ctx) (Input, []byte, string, float64, error) {
	var p Input
	if strings.HasPrefix(c.Get("Content-Type"), "multipart/form-data") {
		p = Input{RequestID: c.FormValue("request_id"), Kind: "audio", RetryOf: c.FormValue("retry_of")}
		p.HintLevel, _ = strconv.Atoi(c.FormValue("hint_level"))
		f, e := c.FormFile("audio")
		if e != nil {
			return p, nil, "", 0, fmt.Errorf("กรุณาแนบเสียง")
		}
		if f.Size == 0 || f.Size > 8<<20 {
			return p, nil, "", 0, fmt.Errorf("ไฟล์เสียงต้องไม่เกิน 8 MB")
		}
		h, e := f.Open()
		if e != nil {
			return p, nil, "", 0, e
		}
		defer h.Close()
		raw, e := io.ReadAll(io.LimitReader(h, 8<<20))
		if e != nil {
			return p, nil, "", 0, e
		}
		ctx, cancel := context.WithTimeout(c.UserContext(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-protocol_whitelist", "pipe", "-i", "pipe:0", "-t", "121", "-vn", "-ac", "1", "-ar", "16000", "-f", "s16le", "pipe:1")
		cmd.Stdin = bytes.NewReader(raw)
		pcm, e := cmd.Output()
		if e != nil {
			return p, nil, "", 0, fmt.Errorf("อ่านเสียงไม่ได้ กรุณาอัดใหม่")
		}
		duration := float64(len(pcm)) / 32000
		if duration < .3 || duration > 120 {
			return p, nil, "", 0, fmt.Errorf("อัดเสียงระหว่าง 1–120 วินาที")
		}
		return p, gemini.WAV(pcm, 16000), "audio/wav", duration, nil
	}
	if c.BodyParser(&p) != nil || strings.TrimSpace(p.Text) == "" || len(p.Text) > 4000 {
		return p, nil, "", 0, fmt.Errorf("กรุณาพิมพ์คำตอบไม่เกิน 4,000 ตัวอักษร")
	}
	p.Kind = "text"
	return p, nil, "", 0, nil
}
func (a *App) storeAudio(ctx context.Context, tx pgx.Tx, uid, key string, b []byte, mime string, expires bool) (string, error) {
	id := uuid.NewString()
	path := filepath.Join(a.Cfg.AudioDir, id+".wav")
	if e := os.WriteFile(path, b, 0600); e != nil {
		return "", e
	}
	var owner, cache, expiry any
	if uid != "" {
		owner = uid
	}
	if key != "" {
		cache = key
	}
	if expires {
		expiry = time.Now().AddDate(0, 0, a.Cfg.RetentionDays)
	}
	_, e := tx.Exec(ctx, "INSERT INTO audio_assets(id,user_id,cache_key,path,mime,expires_at) VALUES($1,$2,$3,$4,$5,$6)", id, owner, cache, path, mime, expiry)
	if e != nil {
		os.Remove(path)
	}
	return id, e
}
func (a *App) audio(c *fiber.Ctx) error {
	if !validID(c.Params("id")) {
		return fail(c, 404, "ไม่พบเสียง")
	}
	var path, mime string
	e := a.DB.QueryRow(c.UserContext(), "SELECT path,mime FROM audio_assets WHERE id=$1 AND (user_id IS NULL OR user_id=$2) AND (expires_at IS NULL OR expires_at>now())", c.Params("id"), user(c).ID).Scan(&path, &mime)
	if e == pgx.ErrNoRows {
		return fail(c, 404, "เสียงหมดอายุหรือไม่มีสิทธิ์เข้าถึง")
	}
	if e != nil {
		return e
	}
	c.Set("Content-Type", mime)
	return c.SendFile(path)
}
func (a *App) ttsJob(c *fiber.Ctx) error {
	var p struct {
		Text  string `json:"text"`
		Voice string `json:"voice"`
	}
	if c.BodyParser(&p) != nil || len(p.Text) < 1 || len(p.Text) > 1500 {
		return fail(c, 400, "ข้อความเสียงต้องยาว 1–1,500 ตัวอักษร")
	}
	if p.Voice == "" {
		p.Voice = textValue(user(c).Profile["voice"])
	}
	if p.Voice != "Kore" && p.Voice != "Puck" && p.Voice != "Aoede" {
		p.Voice = a.Cfg.Voice
	}
	key := security.Digest(user(c).ID + "|" + a.Cfg.Version + "|" + a.Cfg.Models["tts"].ID + "|" + p.Voice + "|en|" + strings.TrimSpace(p.Text))
	var id string
	e := a.DB.QueryRow(c.UserContext(), "SELECT id::text FROM audio_assets WHERE cache_key=$1", key).Scan(&id)
	if e == nil {
		return c.JSON(fiber.Map{"audio_id": id})
	}
	if e != pgx.ErrNoRows {
		return e
	}
	payload := fiber.Map{"text": p.Text, "voice": p.Voice, "cache_key": key}
	e = a.DB.QueryRow(c.UserContext(), "INSERT INTO jobs(id,user_id,kind,request_key,payload) VALUES($1,$2,'tts',$3,$4) ON CONFLICT(user_id,request_key) DO UPDATE SET status=CASE WHEN jobs.status='failed' THEN 'queued' ELSE jobs.status END,error=NULL,attempts=CASE WHEN jobs.status='failed' THEN 0 ELSE jobs.attempts END RETURNING id::text", uuid.NewString(), user(c).ID, "tts:"+key, asJSON(payload)).Scan(&id)
	if e != nil {
		return e
	}
	return c.Status(202).JSON(fiber.Map{"job_id": id})
}
func (a *App) reviewAnswer(c *fiber.Ctx) error {
	if !validID(c.Params("id")) {
		return fail(c, 404, "ไม่พบรายการทบทวน")
	}
	p, audio, mime, duration, e := a.readInput(c)
	if e != nil {
		return fail(c, 400, e.Error())
	}
	if !validID(p.RequestID) {
		return fail(c, 400, "request_id ต้องเป็น UUID")
	}
	tx, e := a.DB.Begin(c.UserContext())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.UserContext())
	var prompt, target string
	var stage int
	var hinted bool
	e = tx.QueryRow(c.UserContext(), "SELECT prompt,target,stage,coalesce(hint_until>now(),false) FROM review_items WHERE id=$1 AND user_id=$2 FOR UPDATE", c.Params("id"), user(c).ID).Scan(&prompt, &target, &stage, &hinted)
	if e == pgx.ErrNoRows {
		return fail(c, 404, "ไม่พบรายการทบทวน")
	}
	if e != nil {
		return e
	}
	var existing []byte
	e = tx.QueryRow(c.UserContext(), "SELECT result FROM review_events WHERE user_id=$1 AND request_id=$2", user(c).ID, p.RequestID).Scan(&existing)
	if e == nil {
		c.Type("json")
		return c.Send(existing)
	}
	if e != pgx.ErrNoRows {
		return e
	}
	usage, e := a.reserve(c.UserContext(), user(c).ID, "", "tutor", 1)
	if e != nil {
		return fail(c, 402, e.Error())
	}
	r, e := a.AI.Generate(c.UserContext(), a.Cfg.Models["tutor"], learning.SystemPrompt, fmt.Sprintf("Review retrieval. Prompt: %s. Target meaning/example: %s. Accept valid alternatives. User typed: %s. Input: %s", prompt, target, p.Text, p.Kind), audio, mime, learning.FeedbackSchema, "")
	a.settle(usage, "tutor", r, e, 0)
	if e != nil {
		return fail(c, 502, e.Error())
	}
	f, parseErr := learning.ParseFeedback(r.Text, len(audio) > 0)
	if parseErr != nil {
		return fail(c, 502, "ผลประเมินไม่สมบูรณ์")
	}
	if p.Kind == "audio" && !f.AudioClear {
		return c.JSON(fiber.Map{"feedback": f, "rescheduled": false})
	}
	hint := p.HintLevel
	if hinted {
		hint = 4
	}
	if hint < 0 || hint > 4 {
		hint = 4
	}
	if p.Kind != "audio" {
		hint = max(hint, 1)
	}
	next, due := learning.Schedule(stage, f.Correct && f.GoalMet, hint, time.Now())
	_, e = tx.Exec(c.UserContext(), "UPDATE review_items SET stage=$1,due_at=$2,failures=failures+$3 WHERE id=$4", next, due, map[bool]int{true: 0, false: 1}[f.Correct], c.Params("id"))
	if e != nil {
		return e
	}
	result := fiber.Map{"feedback": f, "stage": next, "due_at": due, "rescheduled": true}
	_, e = tx.Exec(c.UserContext(), "INSERT INTO review_events(id,user_id,item_id,request_id,result) VALUES($1,$2,$3,$4,$5)", uuid.NewString(), user(c).ID, c.Params("id"), p.RequestID, asJSON(result))
	if e != nil {
		return e
	}
	if len(audio) > 0 && f.AudioClear {
		if _, e = a.storeAudio(c.UserContext(), tx, user(c).ID, "", audio, mime, true); e != nil {
			return e
		}
		if _, e = tx.Exec(c.UserContext(), "INSERT INTO speech_events(id,user_id,source,duration_seconds) VALUES($1,$2,'review',$3)", uuid.NewString(), user(c).ID, duration); e != nil {
			return e
		}
	}
	if e = tx.Commit(c.UserContext()); e != nil {
		return e
	}
	return c.JSON(result)
}
