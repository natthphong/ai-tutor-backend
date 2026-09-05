package app

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"tokoloop/internal/learning"
)

// A post-scene retry keeps the original completed scene immutable.
func (a *App) retrySession(c *fiber.Ctx) error {
	s, e := a.findSession(c)
	if e != nil {
		return e
	}
	if s.Status != "completed" {
		return fail(c, 409, "จบฉากก่อนเริ่มฝึกประโยคจาก feedback")
	}
	var summary struct {
		Feedback learning.Feedback `json:"feedback"`
	}
	if json.Unmarshal(asJSON(s.Summary), &summary) != nil || summary.Feedback.RetrySentence == "" {
		return fail(c, 409, "ยังไม่มีประโยคให้ฝึกซ้ำ")
	}
	id := uuid.NewString()
	tx, e := a.DB.Begin(c.UserContext())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.UserContext())
	state := fiber.Map{"stage": "conversation", "step": 0, "hint_level": 4, "last_pass": false, "independent": 0, "retry_target": summary.Feedback.RetrySentence, "source_session_id": s.ID}
	if _, e = tx.Exec(c.UserContext(), "INSERT INTO learning_sessions(id,user_id,mode,state,model_version) VALUES($1,$2,'retry',$3,$4)", id, user(c).ID, asJSON(state), a.Cfg.Version); e != nil {
		return e
	}
	if _, e = tx.Exec(c.UserContext(), "INSERT INTO turns(id,session_id,role,text) VALUES($1,$2,'model',$3)", uuid.NewString(), id, "Let’s try that sentence again: "+summary.Feedback.RetrySentence); e != nil {
		return e
	}
	if e = tx.Commit(c.UserContext()); e != nil {
		return e
	}
	return c.Status(201).JSON(fiber.Map{"id": id})
}
func (a *App) reviewHint(c *fiber.Ctx) error {
	if !validID(c.Params("id")) {
		return fail(c, 404, "ไม่พบรายการ")
	}
	var target string
	e := a.DB.QueryRow(c.UserContext(), "UPDATE review_items SET hint_until=now()+interval '1 day' WHERE id=$1 AND user_id=$2 RETURNING target", c.Params("id"), user(c).ID).Scan(&target)
	if e != nil {
		return fail(c, 404, "ไม่พบรายการ")
	}
	return c.JSON(fiber.Map{"level": 4, "text": target})
}
