package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
	"tokoloop/internal/content"
)

func (a *App) jsonRows(c *fiber.Ctx, q string, args ...any) error {
	var b []byte
	if e := a.DB.QueryRow(c.UserContext(), q, args...).Scan(&b); e != nil {
		return e
	}
	c.Type("json")
	return c.Send(b)
}
func (a *App) curriculum(c *fiber.Ctx) error {
	return a.jsonRows(c, `SELECT coalesce(jsonb_agg(l.data || jsonb_build_object('completed',coalesce(m.completed,false),'studied',`+studiedSQL+`,'active_session_id',(SELECT s.id FROM learning_sessions s WHERE s.user_id=$1 AND s.lesson_id=l.id AND s.mode='lesson' AND s.status='active' ORDER BY s.updated_at DESC LIMIT 1),'independent_successes',coalesce(m.independent_successes,0)) ORDER BY `+lessonOrderSQL+`),'[]'::jsonb) FROM lessons l LEFT JOIN mastery m ON m.lesson_id=l.id AND m.user_id=$1`, user(c).ID)
}
func (a *App) lesson(c *fiber.Ctx) error {
	b, e := a.lessonData(c.UserContext(), c.Params("id"))
	if e == pgx.ErrNoRows {
		return fail(c, 404, "ไม่พบบทเรียน")
	}
	if e != nil {
		return e
	}
	c.Type("json")
	return c.Send(b)
}
func (a *App) progress(c *fiber.Ctx) error {
	if e := a.refreshReviewCues(c.UserContext(), user(c).ID); e != nil {
		return e
	}
	return a.jsonRows(c, `WITH speech AS (
 SELECT created_at,duration_seconds FROM attempts WHERE user_id=$1 AND input_kind='audio' AND (feedback->>'audio_clear')::boolean
 UNION ALL SELECT created_at,duration_seconds FROM speech_events WHERE user_id=$1
), daily AS (
 SELECT (created_at AT TIME ZONE 'Asia/Bangkok')::date AS day,round(sum(duration_seconds)::numeric/60,1) AS minutes,count(*) AS attempts FROM speech GROUP BY 1
), ranked AS (SELECT day,row_number() OVER(ORDER BY day DESC) AS rn FROM daily), streak AS (
 SELECT count(*) AS n FROM ranked WHERE day=((SELECT max(day) FROM daily)-(rn-1)::int) AND (SELECT max(day) FROM daily)>=(now() AT TIME ZONE 'Asia/Bangkok')::date-1
) SELECT jsonb_build_object(
 'total_lessons',(SELECT count(*) FROM lessons),'speaking_minutes',coalesce((SELECT round(sum(duration_seconds)::numeric/60,1) FROM speech),0),
 'attempts',(SELECT count(*) FROM attempts WHERE user_id=$1)+(SELECT count(*) FROM review_events WHERE user_id=$1),
 'completed_lessons',(SELECT count(*) FROM lessons l LEFT JOIN mastery m ON m.lesson_id=l.id AND m.user_id=$1 WHERE `+studiedSQL+`),'mastered_lessons',(SELECT count(*) FROM mastery WHERE user_id=$1 AND completed),
 'independent_successes',coalesce((SELECT sum(independent_successes) FROM mastery WHERE user_id=$1),0),
 'hint_dependency',coalesce((SELECT round(100.0*count(*) FILTER(WHERE hint_level>0)/nullif(count(*),0)) FROM attempts WHERE user_id=$1),0),
 'streak',(SELECT n FROM streak),'active_vocabulary',(SELECT count(*) FROM vocabulary WHERE user_id=$1 AND uses>=2),
 'daily',coalesce((SELECT jsonb_agg(to_jsonb(daily) ORDER BY day) FROM daily WHERE day >= (now() AT TIME ZONE 'Asia/Bangkok')::date-6),'[]'::jsonb),
 'weaknesses',coalesce((SELECT jsonb_agg(to_jsonb(w)) FROM(SELECT title,prompt,failures,due_at FROM review_items WHERE user_id=$1 ORDER BY failures DESC LIMIT 8) w),'[]'::jsonb))`, user(c).ID)
}
func (a *App) library(c *fiber.Ctx) error {
	if e := a.refreshReviewCues(c.UserContext(), user(c).ID); e != nil {
		return e
	}
	return a.jsonRows(c, `SELECT jsonb_build_object('vocabulary',coalesce((SELECT jsonb_agg(to_jsonb(v) - 'user_id' ORDER BY v.created_at DESC) FROM vocabulary v WHERE user_id=$1),'[]'::jsonb),'patterns',coalesce((SELECT jsonb_agg(l.data || jsonb_build_object('completed',m.completed,'uses',m.independent_successes)) FROM mastery m JOIN lessons l ON l.id=m.lesson_id WHERE m.user_id=$1),'[]'::jsonb),'mistakes',coalesce((SELECT jsonb_agg(to_jsonb(r)-'user_id' ORDER BY due_at) FROM review_items r WHERE user_id=$1),'[]'::jsonb))`, user(c).ID)
}
func (a *App) saveWord(c *fiber.Ctx) error {
	var p content.Word
	if c.BodyParser(&p) != nil || len(p.Term) < 1 || len(p.Term) > 120 || len(p.Meaning) > 1000 || len(p.Example) > 1000 {
		return fail(c, 400, "คำศัพท์ไม่ถูกต้อง")
	}
	id := uuid.NewString()
	_, e := a.DB.Exec(c.UserContext(), "INSERT INTO vocabulary(id,user_id,term,meaning,example) VALUES($1,$2,$3,$4,$5) ON CONFLICT(user_id,term) DO UPDATE SET meaning=excluded.meaning,example=excluded.example", id, user(c).ID, strings.TrimSpace(p.Term), p.Meaning, p.Example)
	if e != nil {
		return e
	}
	_, e = a.DB.Exec(c.UserContext(), "INSERT INTO review_items(id,user_id,key,kind,prompt,target,meaning) VALUES($1,$2,$3,'vocabulary',$4,$5,$4) ON CONFLICT(user_id,key) DO NOTHING", uuid.NewString(), user(c).ID, "word:"+strings.ToLower(p.Term), p.Meaning, p.Term)
	if e != nil {
		return e
	}
	return c.Status(201).JSON(fiber.Map{"ok": true})
}
func (a *App) scenarios(c *fiber.Ctx) error {
	return a.jsonRows(c, "SELECT coalesce(jsonb_agg(data || jsonb_build_object('custom',user_id IS NOT NULL) ORDER BY created_at,id),'[]'::jsonb) FROM scenarios WHERE user_id IS NULL OR user_id=$1", user(c).ID)
}
func (a *App) createScenario(c *fiber.Ctx) error {
	var p struct {
		Prompt    string `json:"prompt"`
		LessonID  string `json:"lesson_id"`
		RequestID string `json:"request_id"`
	}
	if c.BodyParser(&p) != nil || len(p.Prompt) < 5 || len(p.Prompt) > 2000 || !validID(p.RequestID) {
		return fail(c, 400, "ระบุสถานการณ์ที่อยากฝึกและรหัสคำขอ")
	}
	id := uuid.NewString()
	payload := fiber.Map{"prompt": p.Prompt, "level": user(c).Profile["level"], "lesson_id": p.LessonID}
	var result string
	e := a.DB.QueryRow(c.UserContext(), "INSERT INTO jobs(id,user_id,kind,request_key,payload) VALUES($1,$2,'scenario',$3,$4) ON CONFLICT(user_id,request_key) DO UPDATE SET request_key=excluded.request_key RETURNING id::text", id, user(c).ID, "scenario:"+p.RequestID, asJSON(payload)).Scan(&result)
	if e != nil {
		return e
	}
	return c.Status(202).JSON(fiber.Map{"job_id": result})
}
func (a *App) editScenario(c *fiber.Ctx) error {
	var s content.Scenario
	if c.BodyParser(&s) != nil || len(s.Title) < 3 || len(s.Title) > 120 || len(s.Brief) > 3000 || len(s.Goal) > 1000 || len(s.Roles) < 1 || len(s.Roles) > 5 {
		return fail(c, 400, "ข้อมูลสถานการณ์ไม่ถูกต้อง")
	}
	s.ID = c.Params("id")
	tag, e := a.DB.Exec(c.UserContext(), "UPDATE scenarios SET data=$1 WHERE id=$2 AND user_id=$3", asJSON(s), s.ID, user(c).ID)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		return fail(c, 404, "ไม่พบสถานการณ์ของคุณ")
	}
	return c.JSON(s)
}
func (a *App) reviews(c *fiber.Ctx) error {
	if e := a.refreshReviewCues(c.UserContext(), user(c).ID); e != nil {
		return e
	}
	return a.jsonRows(c, "SELECT coalesce(jsonb_agg(to_jsonb(r)-'user_id' ORDER BY due_at),'[]'::jsonb) FROM review_items r WHERE user_id=$1 AND due_at<=now()", user(c).ID)
}
func (a *App) job(c *fiber.Ctx) error {
	if !validID(c.Params("id")) {
		return fail(c, 404, "ไม่พบงาน")
	}
	var b []byte
	e := a.DB.QueryRow(c.UserContext(), "SELECT jsonb_build_object('id',id,'kind',kind,'status',status,'result',result,'error',error) FROM jobs WHERE id=$1 AND user_id=$2", c.Params("id"), user(c).ID).Scan(&b)
	if e == pgx.ErrNoRows {
		return fail(c, 404, "ไม่พบงาน")
	}
	if e != nil {
		return e
	}
	c.Type("json")
	return c.Send(b)
}
func (a *App) lessonData(ctx context.Context, id string) ([]byte, error) {
	key := "lesson:" + a.Cfg.Version + ":" + id
	if a.Cache != nil {
		if b, ok, e := a.Cache.Get(ctx, key); e == nil && ok {
			return b, nil
		}
	}
	var b []byte
	if e := a.DB.QueryRow(ctx, "SELECT data FROM lessons WHERE id=$1", id).Scan(&b); e != nil {
		return nil, e
	}
	if a.Cache != nil {
		_ = a.Cache.Set(ctx, key, b, time.Duration(a.Cfg.CacheTTLSeconds)*time.Second)
	}
	return b, nil
}
func (a *App) contextLesson(c *fiber.Ctx, id *string) (content.Lesson, error) {
	var l content.Lesson
	if id == nil {
		return l, nil
	}
	b, e := a.lessonData(c.UserContext(), *id)
	if e != nil {
		return l, e
	}
	e = json.Unmarshal(b, &l)
	return l, e
}
func firstPrompt(mode string, l content.Lesson, s content.Scenario) string {
	if mode == "placement" {
		return "Hello! Please introduce yourself. Tell me your name and what you do."
	}
	if s.ID != "" {
		return s.Opening
	}
	if l.ID != "" {
		return fmt.Sprintf("Let’s practice: %s", l.Example)
	}
	return "Hi! What would you like to talk about today?"
}
