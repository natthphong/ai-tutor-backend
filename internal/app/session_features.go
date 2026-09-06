package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
	"tokoloop/internal/learning"
	"tokoloop/internal/security"
)

type LessonProgress struct {
	Percent                  int  `json:"percent"`
	CompletedDrills          int  `json:"completed_drills"`
	TotalDrills              int  `json:"total_drills"`
	IndependentConversations int  `json:"independent_conversations"`
	RequiredConversations    int  `json:"required_conversations"`
	Ready                    bool `json:"ready_to_complete"`
}

func lessonProgress(s Session) *LessonProgress {
	if s.Mode != "lesson" {
		return nil
	}
	p := &LessonProgress{TotalDrills: 4, RequiredConversations: 2}
	stage := textValue(s.State["stage"])
	if stage == "conversation" {
		p.CompletedDrills = 4
	} else if stage == "drill" {
		p.CompletedDrills = min(4, max(0, int(number(s.State["step"], 0))))
		if s.State["last_pass"] == true {
			p.CompletedDrills = min(4, p.CompletedDrills+1)
		}
	}
	p.CompletedDrills = min(4, max(p.CompletedDrills, int(number(s.State["progress_drills"], 0))))
	p.IndependentConversations = min(2, max(0, int(number(s.State["independent"], 0))))
	p.Ready = stage == "conversation" && p.CompletedDrills == 4 && p.IndependentConversations == 2
	p.Percent = (p.CompletedDrills + p.IndependentConversations) * 100 / 6
	return p
}
func (a *App) sessionSettings(c *fiber.Ctx) error {
	s, e := a.findSession(c)
	if e != nil {
		return e
	}
	if s.Status != "active" {
		return fail(c, 409, "session จบแล้ว")
	}
	var p struct {
		AutoAudio *bool `json:"auto_audio"`
	}
	if c.BodyParser(&p) != nil || p.AutoAudio == nil {
		return fail(c, 400, "ระบุ auto_audio")
	}
	_, e = a.DB.Exec(c.UserContext(), "UPDATE learning_sessions SET state=jsonb_set(state,'{auto_audio}',$1::jsonb),updated_at=now() WHERE id=$2 AND user_id=$3 AND status='active'", asJSON(*p.AutoAudio), s.ID, user(c).ID)
	if e != nil {
		return e
	}
	return c.JSON(fiber.Map{"auto_audio": *p.AutoAudio})
}
func (a *App) ttsKey(uid, text, voice string) string {
	return security.Digest(uid + "|tts-v3|" + string(asJSON(a.Cfg.Models["tts"])) + "|" + voice + "|en|" + strings.TrimSpace(text))
}
func (a *App) finishTurn(c *fiber.Ctx, s Session, id string, independent bool) error {
	var raw []byte
	var learnerAudio, replyAudio, replyTurn *string
	var audioError string
	e := a.DB.QueryRow(c.UserContext(), "SELECT feedback,audio_id::text,reply_audio_id::text,reply_turn_id::text,reply_audio_error FROM attempts WHERE id=$1 AND user_id=$2 AND session_id=$3", id, user(c).ID, s.ID).Scan(&raw, &learnerAudio, &replyAudio, &replyTurn, &audioError)
	if e != nil {
		return e
	}
	var f learning.Feedback
	if e = json.Unmarshal(raw, &f); e != nil {
		return e
	}
	// This runs after the learning transaction commits. Voice failure never rolls back the answer.
	if s.State["auto_audio"] == true && replyAudio == nil && audioError == "" {
		result, generationError, _ := a.replies.Do(id, func() (any, error) {
			var prior *string
			var priorError string
			if e := a.DB.QueryRow(c.UserContext(), "SELECT reply_audio_id::text,reply_audio_error FROM attempts WHERE id=$1", id).Scan(&prior, &priorError); e != nil {
				return nil, e
			}
			if prior != nil || priorError != "" {
				return prior, nil
			}
			voice := textValue(user(c).Profile["voice"])
			if voice == "" {
				voice = a.Cfg.Voice
			}
			call, cancel := context.WithTimeout(c.UserContext(), 18*time.Second)
			defer cancel()
			generated, e := a.makeTTS(call, user(c).ID, map[string]any{"text": f.Reply, "voice": voice, "cache_key": a.ttsKey(user(c).ID, f.Reply, voice)})
			if e != nil {
				_, _ = a.DB.Exec(context.Background(), "UPDATE attempts SET reply_audio_error=$1 WHERE id=$2", "สร้างเสียงไม่สำเร็จ แต่บันทึกคำตอบแล้ว กดฟังเพื่อลองสร้างเสียงอีกครั้ง", id)
				return nil, e
			}
			value := generated.(map[string]any)["audio_id"].(string)
			tx, e := a.DB.Begin(c.UserContext())
			if e != nil {
				return nil, e
			}
			defer tx.Rollback(c.UserContext())
			if _, e = tx.Exec(c.UserContext(), "UPDATE attempts SET reply_audio_id=$1 WHERE id=$2", value, id); e != nil {
				return nil, e
			}
			if replyTurn != nil {
				if _, e = tx.Exec(c.UserContext(), "UPDATE turns SET audio_id=$1 WHERE id=$2", value, *replyTurn); e != nil {
					return nil, e
				}
			}
			if e = tx.Commit(c.UserContext()); e != nil {
				return nil, e
			}
			return &value, nil
		})
		if result != nil {
			replyAudio, _ = result.(*string)
		}
		_ = a.DB.QueryRow(c.UserContext(), "SELECT reply_audio_error FROM attempts WHERE id=$1", id).Scan(&audioError)
		if generationError != nil && audioError == "" {
			audioError = "บันทึกคำตอบแล้ว แต่ยังเตรียมเสียงไม่ได้ กดฟังเพื่อลองอีกครั้ง"
		}
	}
	p := lessonProgress(s)
	if p != nil && p.Ready && s.Status == "active" {
		if e = a.completeSession(c); e != nil {
			return e
		}
		s.Status = "completed"
	}
	return c.JSON(fiber.Map{"id": id, "feedback": f, "audio_id": learnerAudio, "reply_audio_id": replyAudio, "audio_error": audioError, "independent": independent, "state": s.State, "progress": p, "session_completed": s.Status == "completed"})
}
func (a *App) translateTurn(c *fiber.Ctx) error {
	s, e := a.findSession(c)
	if e != nil {
		return e
	}
	tid := c.Params("turnID")
	if !validID(tid) {
		return fail(c, 404, "ไม่พบข้อความ")
	}
	var english, thai string
	e = a.DB.QueryRow(c.UserContext(), "SELECT text,text_th FROM turns WHERE id=$1 AND session_id=$2 AND role='model'", tid, s.ID).Scan(&english, &thai)
	if e == pgx.ErrNoRows {
		return fail(c, 404, "ไม่พบข้อความ")
	}
	if e != nil {
		return e
	}
	if thai != "" {
		return c.JSON(fiber.Map{"text_th": thai})
	}
	result, e, _ := a.replies.Do("translate:"+tid, func() (any, error) {
		var saved string
		if e := a.DB.QueryRow(c.UserContext(), "SELECT text_th FROM turns WHERE id=$1", tid).Scan(&saved); e != nil {
			return nil, e
		}
		if saved != "" {
			return saved, nil
		}
		usage, e := a.reserve(c.UserContext(), user(c).ID, s.ID, "helper", .2)
		if e != nil {
			return nil, e
		}
		r, e := a.AI.Generate(c.UserContext(), a.Cfg.Models["helper"], "Translate the provided English speaking tutor message to natural Thai. Preserve the meaning and question, not an answer. Output only the Thai translation.", english, nil, "", nil, "")
		a.settle(usage, "helper", r, e, 0)
		if e != nil {
			return nil, e
		}
		if strings.TrimSpace(r.Text) == "" {
			return nil, fmt.Errorf("empty translation")
		}
		_, e = a.DB.Exec(c.UserContext(), "UPDATE turns SET text_th=$1 WHERE id=$2", r.Text, tid)
		return r.Text, e
	})
	if e != nil {
		return fail(c, 502, "แปลไม่สำเร็จ กรุณาลองอีกครั้ง")
	}
	return c.JSON(fiber.Map{"text_th": result})
}
