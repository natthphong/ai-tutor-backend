package app

import (
	"context"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"strings"
	"time"
)

func partialCaption(text string) string {
	words := strings.Fields(text)
	if len(words) == 1 {
		return "____"
	}
	for i := range words {
		if i%3 != 0 {
			words[i] = "____"
		}
	}
	return strings.Join(words, " ")
}
func (a *App) listen(c *fiber.Ctx) error {
	s, e := a.findSession(c)
	if e != nil {
		return e
	}
	if s.Mode != "listening" || s.Status != "active" {
		return fail(c, 409, "โหมดฟังไม่พร้อม")
	}
	var p struct {
		RequestID string `json:"request_id"`
	}
	if c.BodyParser(&p) != nil || !validID(p.RequestID) {
		return fail(c, 400, "ระบุรหัสคำขอ")
	}
	tx, e := a.DB.Begin(c.UserContext())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.UserContext())
	var raw []byte
	if e = tx.QueryRow(c.UserContext(), "SELECT state,status FROM learning_sessions WHERE id=$1 FOR NO KEY UPDATE", s.ID).Scan(&raw, &s.Status); e != nil {
		return e
	}
	if s.Status != "active" {
		return fail(c, 409, "session จบแล้ว")
	}
	json.Unmarshal(raw, &s.State)
	if s.State["listen_request_id"] == p.RequestID {
		return c.JSON(s.State["listen_result"])
	}
	var tid, text, thai string
	var audio *string
	if e = tx.QueryRow(c.UserContext(), "SELECT id::text,text,text_th,audio_id::text FROM turns WHERE session_id=$1 AND role='model' ORDER BY created_at DESC LIMIT 1", s.ID).Scan(&tid, &text, &thai, &audio); e != nil {
		return e
	}
	count := 0
	if s.State["listening_turn_id"] == tid {
		count = int(number(s.State["listen_count"], 0))
	}
	count = min(3, count+1)
	if audio == nil {
		voice := textValue(user(c).Profile["voice"])
		if voice == "" {
			voice = a.Cfg.Voice
		}
		ctx, cancel := context.WithTimeout(c.UserContext(), 25*time.Second)
		result, err := a.makeTTS(ctx, user(c).ID, map[string]any{"text": text, "voice": voice, "cache_key": a.ttsKey(user(c).ID, text, voice)})
		cancel()
		if err != nil {
			return fail(c, 502, "เตรียมเสียงไม่สำเร็จ ลองอีกครั้งได้โดยยังไม่นับรอบฟัง")
		}
		value := result.(map[string]any)["audio_id"].(string)
		audio = &value
		if _, e = tx.Exec(c.UserContext(), "UPDATE turns SET audio_id=$1 WHERE id=$2", value, tid); e != nil {
			return e
		}
	}
	caption := ""
	translation := ""
	hint := 0
	if count == 2 {
		caption = partialCaption(text)
		hint = 2
	}
	if count == 3 {
		caption = text
		translation = thai
		hint = 4
	}
	result := fiber.Map{"turn_id": tid, "audio_id": audio, "listen_count": count, "caption": caption, "translation": translation}
	s.State["listening_turn_id"] = tid
	s.State["listen_count"] = count
	s.State["hint_level"] = max(hint, int(number(s.State["hint_level"], 0)))
	s.State["listen_request_id"] = p.RequestID
	s.State["listen_result"] = result
	if _, e = tx.Exec(c.UserContext(), "UPDATE learning_sessions SET state=$1,updated_at=now() WHERE id=$2", asJSON(s.State), s.ID); e != nil {
		return e
	}
	if e = tx.Commit(c.UserContext()); e != nil {
		return e
	}
	return c.JSON(result)
}
