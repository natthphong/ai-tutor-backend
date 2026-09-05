package app

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
	"tokoloop/internal/security"

	"tokoloop/internal/content"
	"tokoloop/internal/learning"
)

type Session struct {
	ID         string         `json:"id"`
	LessonID   *string        `json:"lesson_id"`
	ScenarioID *string        `json:"scenario_id"`
	Mode       string         `json:"mode"`
	Status     string         `json:"status"`
	State      map[string]any `json:"state"`
	Summary    any            `json:"summary"`
}

func (a *App) findSession(c *fiber.Ctx) (Session, error) {
	var s Session
	if !validID(c.Params("id")) {
		return s, fiber.NewError(404, "ไม่พบ session")
	}
	var b []byte
	e := a.DB.QueryRow(c.UserContext(), "SELECT id::text,lesson_id,scenario_id,mode,status,state,summary FROM learning_sessions WHERE id=$1 AND user_id=$2", c.Params("id"), user(c).ID).Scan(&s.ID, &s.LessonID, &s.ScenarioID, &s.Mode, &s.Status, &b, &s.Summary)
	if e == pgx.ErrNoRows {
		return s, fiber.NewError(404, "ไม่พบ session")
	}
	if e != nil {
		return s, e
	}
	e = json.Unmarshal(b, &s.State)
	return s, e
}
func (a *App) createSession(c *fiber.Ctx) error {
	var p struct {
		LessonID   *string `json:"lesson_id"`
		ScenarioID *string `json:"scenario_id"`
		Mode       string  `json:"mode"`
	}
	if c.BodyParser(&p) != nil {
		return fail(c, 400, "ข้อมูลไม่ถูกต้อง")
	}
	if p.Mode != "lesson" && p.Mode != "free" && p.Mode != "scenario" && p.Mode != "live" && p.Mode != "placement" {
		return fail(c, 400, "โหมดไม่ถูกต้อง")
	}
	l, e := a.contextLesson(c, p.LessonID)
	if e != nil {
		return fail(c, 404, "ไม่พบบทเรียน")
	}
	if p.Mode == "lesson" && l.ID == "" {
		return fail(c, 400, "เลือกบทเรียนก่อน")
	}
	var s content.Scenario
	if p.ScenarioID != nil {
		var b []byte
		e = a.DB.QueryRow(c.UserContext(), "SELECT data FROM scenarios WHERE id=$1 AND (user_id IS NULL OR user_id=$2)", *p.ScenarioID, user(c).ID).Scan(&b)
		if e != nil {
			return fail(c, 404, "ไม่พบสถานการณ์")
		}
		json.Unmarshal(b, &s)
	}
	id := uuid.NewString()
	stage := "conversation"
	if p.Mode == "lesson" {
		stage = "pattern"
	}
	state := fiber.Map{"stage": stage, "step": 0, "hint_level": 0, "last_pass": false, "independent": 0, "live_active": false, "step_started_at": time.Now().UnixMilli()}
	tx, e := a.DB.Begin(c.UserContext())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.UserContext())
	_, e = tx.Exec(c.UserContext(), "INSERT INTO learning_sessions(id,user_id,lesson_id,scenario_id,mode,state,model_version) VALUES($1,$2,$3,$4,$5,$6,$7)", id, user(c).ID, p.LessonID, p.ScenarioID, p.Mode, asJSON(state), a.Cfg.Version)
	if e != nil {
		return e
	}
	_, e = tx.Exec(c.UserContext(), "INSERT INTO turns(id,session_id,role,text) VALUES($1,$2,'model',$3)", uuid.NewString(), id, firstPrompt(p.Mode, l, s))
	if e != nil {
		return e
	}
	if e = tx.Commit(c.UserContext()); e != nil {
		return e
	}
	return c.Status(201).JSON(fiber.Map{"id": id})
}
func (a *App) sessions(c *fiber.Ctx) error {
	return a.jsonRows(c, "SELECT coalesce(jsonb_agg(to_jsonb(s) ORDER BY updated_at DESC),'[]'::jsonb) FROM(SELECT id,lesson_id,scenario_id,mode,status,state,summary,created_at,updated_at FROM learning_sessions WHERE user_id=$1 ORDER BY updated_at DESC LIMIT 50) s", user(c).ID)
}
func (a *App) getSession(c *fiber.Ctx) error {
	s, e := a.findSession(c)
	if e != nil {
		return e
	}
	l, e := a.contextLesson(c, s.LessonID)
	if e != nil {
		return e
	}
	var turns, attempts []byte
	if e = a.DB.QueryRow(c.UserContext(), "SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY created_at),'[]'::jsonb) FROM(SELECT id,role,text,audio_id,created_at FROM turns WHERE session_id=$1) t", s.ID).Scan(&turns); e != nil {
		return e
	}
	if e = a.DB.QueryRow(c.UserContext(), "SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY created_at),'[]'::jsonb) FROM(SELECT id,transcript,feedback,hint_level,input_kind,duration_seconds,audio_id,retry_of,created_at FROM attempts WHERE session_id=$1) t", s.ID).Scan(&attempts); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"session": s, "lesson": l, "turns": json.RawMessage(turns), "attempts": json.RawMessage(attempts)})
}
func (a *App) hint(c *fiber.Ctx) error {
	s, e := a.findSession(c)
	if e != nil {
		return e
	}
	if s.Status != "active" {
		return fail(c, 409, "session จบแล้ว")
	}
	l, e := a.contextLesson(c, s.LessonID)
	if e != nil {
		return e
	}
	var request struct {
		Idea string `json:"idea"`
	}
	_ = c.BodyParser(&request)
	if len(request.Idea) > 500 {
		return fail(c, 400, "ไอเดียยาวเกินไป")
	}
	var level int
	e = a.DB.QueryRow(c.UserContext(), `UPDATE learning_sessions SET state=jsonb_set(state,'{hint_level}',to_jsonb(least(4,coalesce((state->>'hint_level')::int,0)+1))),updated_at=now() WHERE id=$1 AND status='active' RETURNING (state->>'hint_level')::int`, s.ID).Scan(&level)
	if e != nil {
		return e
	}
	if l.ID != "" && strings.TrimSpace(request.Idea) == "" {
		return c.JSON(fiber.Map{"level": level, "text": learning.Hint(l.Pattern, l.Example, l.Meaning, level)})
	}
	var prompt string
	if e = a.DB.QueryRow(c.UserContext(), "SELECT text FROM turns WHERE session_id=$1 AND role='model' ORDER BY created_at DESC LIMIT 1", s.ID).Scan(&prompt); e != nil {
		return e
	}
	key := security.Digest(user(c).ID + a.Cfg.Version + a.Cfg.Models["helper"].ID + fmt.Sprint(level) + request.Idea + prompt + l.Pattern)
	var cached []byte
	if err := a.DB.QueryRow(c.UserContext(), "SELECT data FROM hint_cache WHERE key=$1 AND expires_at>now()", key).Scan(&cached); err == nil {
		c.Type("json")
		return c.Send(cached)
	}
	usage, e := a.reserve(c.UserContext(), user(c).ID, s.ID, "helper", .5)
	if e != nil {
		return fail(c, 402, e.Error())
	}
	r, e := a.AI.Generate(c.UserContext(), a.Cfg.Models["helper"], "Help a Thai learner respond. Hint level 1=idea in Thai,2=keywords,3=sentence pattern with blanks,4=one full example with Thai meaning. Never reveal a full sentence before level 4. Respond in at most 70 words.", fmt.Sprintf("Question: %s\nHint level: %d\nLearner idea in Thai: %s\nPattern: %s", prompt, level, request.Idea, l.Pattern), nil, "", nil, "")
	a.settle(usage, "helper", r, e, 0)
	if e != nil {
		return fail(c, 502, e.Error())
	}
	result := fiber.Map{"level": level, "text": r.Text}
	_, _ = a.DB.Exec(c.UserContext(), "INSERT INTO hint_cache(key,data,expires_at) VALUES($1,$2,now()+interval '30 days') ON CONFLICT(key) DO UPDATE SET data=excluded.data,expires_at=excluded.expires_at", key, asJSON(result))
	return c.JSON(result)
}
func (a *App) advance(c *fiber.Ctx) error {
	s, e := a.findSession(c)
	if e != nil {
		return e
	}
	if s.Status != "active" {
		return fail(c, 409, "session จบแล้ว")
	}
	tx, e := a.DB.Begin(c.UserContext())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.UserContext())
	var b []byte
	if e = tx.QueryRow(c.UserContext(), "SELECT state,status FROM learning_sessions WHERE id=$1 FOR NO KEY UPDATE", s.ID).Scan(&b, &s.Status); e != nil {
		return e
	}
	json.Unmarshal(b, &s.State)
	if s.Status != "active" {
		return fail(c, 409, "session จบแล้ว")
	}
	stage := textValue(s.State["stage"])
	step := int(number(s.State["step"], 0))
	if stage == "pattern" {
		s.State["stage"] = "drill"
		s.State["step"] = 0
	} else {
		if s.State["last_pass"] != true {
			return fail(c, 409, "ลองพูดให้ผ่านเป้าหมายก่อนเปลี่ยนข้อ")
		}
		step++
		if stage == "drill" && step >= 4 {
			s.State["stage"] = "conversation"
			step = 0
		}
		s.State["step"] = step
	}
	s.State["hint_level"] = 0
	s.State["last_pass"] = false
	s.State["step_started_at"] = time.Now().UnixMilli()
	_, e = tx.Exec(c.UserContext(), "UPDATE learning_sessions SET state=$1,updated_at=now() WHERE id=$2", asJSON(s.State), s.ID)
	if e != nil {
		return e
	}
	if e = tx.Commit(c.UserContext()); e != nil {
		return e
	}
	return c.JSON(s.State)
}
func (a *App) submitTurn(c *fiber.Ctx) error {
	s, e := a.findSession(c)
	if e != nil {
		return e
	}
	if s.Status != "active" {
		return fail(c, 409, "session จบแล้ว")
	}
	if s.State["live_active"] == true {
		return fail(c, 409, "พัก Live ก่อนส่งคำตอบ")
	}
	if textValue(s.State["stage"]) == "pattern" {
		return fail(c, 409, "เริ่มขั้นฝึกก่อนส่งคำตอบ")
	}
	p, audio, mime, duration, e := a.readInput(c)
	if e != nil {
		return fail(c, 400, e.Error())
	}
	if !validID(p.RequestID) {
		return fail(c, 400, "request_id ต้องเป็น UUID")
	}
	u := user(c)
	tx, e := a.DB.Begin(c.UserContext())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.UserContext())
	var state []byte
	if e = tx.QueryRow(c.UserContext(), "SELECT state,status FROM learning_sessions WHERE id=$1 FOR NO KEY UPDATE", s.ID).Scan(&state, &s.Status); e != nil {
		return e
	}
	json.Unmarshal(state, &s.State)
	if s.Status != "active" {
		return fail(c, 409, "session จบแล้ว")
	}
	if s.State["live_active"] == true {
		return fail(c, 409, "พัก Live ก่อนส่งคำตอบ")
	}
	var existing []byte
	e = tx.QueryRow(c.UserContext(), "SELECT jsonb_build_object('id',id,'feedback',feedback,'audio_id',audio_id) FROM attempts WHERE user_id=$1 AND request_id=$2", u.ID, p.RequestID).Scan(&existing)
	if e == nil {
		c.Type("json")
		return c.Send(existing)
	}
	if e != pgx.ErrNoRows {
		return e
	}
	var lastID *string
	var retryFeedback []byte
	e = tx.QueryRow(c.UserContext(), "SELECT id::text,feedback FROM attempts WHERE session_id=$1 ORDER BY created_at DESC LIMIT 1", s.ID).Scan(&lastID, &retryFeedback)
	if e != nil && e != pgx.ErrNoRows {
		return e
	}
	retry := p.RetryOf != ""
	if retry && (!validID(p.RetryOf) || lastID == nil || *lastID != p.RetryOf) {
		return fail(c, 400, "retry ต้องอ้างอิงคำตอบล่าสุดใน session")
	}
	l, e := a.contextLesson(c, s.LessonID)
	if e != nil {
		return e
	}
	var history []byte
	e = tx.QueryRow(c.UserContext(), "SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY created_at),'[]'::jsonb) FROM(SELECT role,left(text,1000) AS text,created_at FROM turns WHERE session_id=$1 ORDER BY created_at DESC LIMIT 8)t", s.ID).Scan(&history)
	if e != nil {
		return e
	}
	var weak []byte
	if e = tx.QueryRow(c.UserContext(), "SELECT coalesce(jsonb_agg(prompt),'[]'::jsonb) FROM(SELECT prompt FROM review_items WHERE user_id=$1 ORDER BY failures DESC LIMIT 5)w", u.ID).Scan(&weak); e != nil {
		return e
	}
	task := l.ConversationPrompt
	stage := textValue(s.State["stage"])
	step := int(number(s.State["step"], 0))
	if stage == "drill" && step < len(l.Drills) {
		task = string(asJSON(l.Drills[step]))
	}
	if retry {
		task = "Retry this corrected sentence, not a new question: " + string(retryFeedback)
	}
	if s.Mode == "retry" {
		task = "Repeat the corrected sentence from the completed scene: " + textValue(s.State["retry_target"])
	}
	if s.Mode == "placement" {
		task = "Placement interview. Progress through introduce yourself, describe yesterday, explain a problem, compare solutions, defend a trade-off. Ask the next question at increasing difficulty. Return estimated level based only on demonstrated ability."
	}
	var scenarioContext []byte
	if s.ScenarioID != nil {
		if e = tx.QueryRow(c.UserContext(), "SELECT data FROM scenarios WHERE id=$1", *s.ScenarioID).Scan(&scenarioContext); e != nil {
			return e
		}
		task += "\nScenario and goal: " + string(scenarioContext)
	}
	var performance []byte
	if e = tx.QueryRow(c.UserContext(), `SELECT jsonb_build_object('recent_attempts',count(*),'hint_rate',coalesce(avg(hint_level),0),'response_ms',coalesce(avg(response_ms),0),'independent_ratio',coalesce(avg(CASE WHEN input_kind='audio' AND (feedback->>'audio_clear')::boolean AND (feedback->>'correct')::boolean AND (feedback->>'goal_met')::boolean AND hint_level=0 AND retry_of IS NULL THEN 1.0 ELSE 0 END),0)) FROM(SELECT * FROM attempts WHERE user_id=$1 ORDER BY created_at DESC LIMIT 8)p`, u.ID).Scan(&performance); e != nil {
		return e
	}
	prompt := fmt.Sprintf("Learner profile: %s\nWeaknesses: %s\nLesson: %s\nTask: %s\nStage: %s\nHistory: %s\nInput type: %s\nLearner text (empty for audio): %s\n", asJSON(u.Profile), weak, asJSON(map[string]any{"level": l.Level, "objective": l.Objective, "pattern": l.Pattern, "example": l.Example, "acceptance": l.Acceptance}), task, stage, history, p.Kind, p.Text)
	prompt += "\nRecent performance: " + string(performance) + "\nPersonalize: only after at least 4 recent attempts, if independent_ratio>=0.75 and hint_rate<1 ask for one additional reason or follow-up detail; otherwise keep one short concrete question and avoid auto-revealing answers. Do not claim a new CEFR level from one answer."
	if l.Assessment {
		prompt += "\nUnit checkpoint: check transfer to a fresh context, ask for a follow-up and one earlier reusable pattern; copied examples are not independent transfer."
	}
	usage, e := a.reserve(c.UserContext(), u.ID, s.ID, "tutor", 2)
	if e != nil {
		return fail(c, 402, e.Error())
	}
	r, e := a.AI.Generate(c.UserContext(), a.Cfg.Models["tutor"], learning.SystemPrompt, prompt, audio, mime, learning.FeedbackSchema, "")
	a.settle(usage, "tutor", r, e, 0)
	if e != nil {
		return fail(c, 502, e.Error())
	}
	f, parseErr := learning.ParseFeedback(r.Text, len(audio) > 0)
	if parseErr != nil {
		return fail(c, 502, "ผลประเมินไม่สมบูรณ์ กรุณาลองใหม่")
	}
	if p.Kind == "text" {
		f.Transcript = p.Text
	}
	id := uuid.NewString()
	hint := int(number(s.State["hint_level"], 0))
	var aid any
	if len(audio) > 0 {
		audioID, e := a.storeAudio(c.UserContext(), tx, u.ID, "", audio, mime, true)
		if e != nil {
			return e
		}
		aid = audioID
	}
	var retryID any
	if retry {
		retryID = p.RetryOf
	}
	_, e = tx.Exec(c.UserContext(), "INSERT INTO attempts(id,session_id,user_id,request_id,input_kind,transcript,feedback,hint_level,duration_seconds,retry_of,audio_id,response_ms) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)", id, s.ID, u.ID, p.RequestID, p.Kind, f.Transcript, asJSON(f), hint, duration, retryID, aid, min(600000, max(0, time.Now().UnixMilli()-int64(number(s.State["step_started_at"], float64(time.Now().UnixMilli()))))))
	if e != nil {
		return e
	}
	for _, t := range []struct {
		role, text string
		audio      any
	}{{"user", f.Transcript, aid}, {"model", f.Reply, nil}} {
		if _, e = tx.Exec(c.UserContext(), "INSERT INTO turns(id,session_id,role,text,audio_id) VALUES($1,$2,$3,$4,$5)", uuid.NewString(), s.ID, t.role, t.text, t.audio); e != nil {
			return e
		}
	}
	independent := learning.Independent(f, p.Kind, hint, retry)
	if independent && stage == "conversation" {
		seen, _ := s.State["independent_steps"].(map[string]any)
		if seen == nil {
			seen = map[string]any{}
		}
		key := fmt.Sprint(step)
		if l.ID != "" && seen[key] == true {
			independent = false
		}
		seen[key] = true
		s.State["independent_steps"] = seen
	}
	s.State["last_pass"] = f.Correct && f.GoalMet && (p.Kind == "text" || f.AudioClear)
	if independent && stage == "conversation" {
		s.State["independent"] = number(s.State["independent"], 0) + 1
	}
	s.State["last_attempt"] = id
	s.State["estimated_level"] = f.Level
	if l.ID != "" {
		inc, assisted := 0, 0
		if independent && stage == "conversation" {
			inc = 1
		} else if f.Correct && p.Kind == "audio" && f.AudioClear {
			assisted = 1
		}
		_, e = tx.Exec(c.UserContext(), "INSERT INTO mastery(user_id,lesson_id,independent_successes,assisted_successes) VALUES($1,$2,$3,$4) ON CONFLICT(user_id,lesson_id) DO UPDATE SET independent_successes=mastery.independent_successes+$3,assisted_successes=mastery.assisted_successes+$4,updated_at=now()", u.ID, l.ID, inc, assisted)
		if e != nil {
			return e
		}
	}
	if p.Kind == "text" || f.AudioClear {
		for _, w := range f.Weaknesses {
			if len(w) > 160 {
				continue
			}
			target := f.RetrySentence
			if target == "" {
				target = l.Example
			}
			if target == "" {
				continue
			}
			_, e = tx.Exec(c.UserContext(), "INSERT INTO review_items(id,user_id,key,kind,prompt,target,meaning,failures,source_attempt) VALUES($1,$2,$3,'mistake',$4,$5,$6,1,$7) ON CONFLICT(user_id,key) DO UPDATE SET failures=review_items.failures+1,due_at=least(review_items.due_at,now()),target=excluded.target,source_attempt=excluded.source_attempt", uuid.NewString(), u.ID, "mistake:"+strings.ToLower(w), w, target, f.Meaning, id)
			if e != nil {
				return e
			}
		}
	}
	if independent {
		_, e = tx.Exec(c.UserContext(), `UPDATE vocabulary SET uses=uses+1 WHERE user_id=$1 AND strpos(' '||trim(regexp_replace(lower($2),'[^a-z0-9]+',' ','g'))||' ', ' '||trim(regexp_replace(lower(term),'[^a-z0-9]+',' ','g'))||' ')>0`, u.ID, f.Transcript)
		if e != nil {
			return e
		}
	}

	if _, e = tx.Exec(c.UserContext(), "UPDATE learning_sessions SET state=$1,updated_at=now() WHERE id=$2", asJSON(s.State), s.ID); e != nil {
		return e
	}
	if e = tx.Commit(c.UserContext()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"id": id, "feedback": f, "audio_id": aid, "independent": independent, "state": s.State})
}
func (a *App) completeSession(c *fiber.Ctx) error {
	s, e := a.findSession(c)
	if e != nil {
		return e
	}
	if s.State["live_active"] == true {
		return fail(c, 409, "หยุด Live ก่อนจบ session")
	}
	if s.Status == "completed" {
		return c.JSON(s.Summary)
	}
	var n int
	if e = a.DB.QueryRow(c.UserContext(), "SELECT count(*) FROM attempts WHERE session_id=$1", s.ID).Scan(&n); e != nil {
		return e
	}
	if s.Mode == "placement" && n < 5 {
		return fail(c, 409, "ตอบคำถามประเมินอย่างน้อย 5 ข้อ")
	}
	tx, e := a.DB.Begin(c.UserContext())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.UserContext())
	var state []byte
	if e = tx.QueryRow(c.UserContext(), "SELECT state,status,summary FROM learning_sessions WHERE id=$1 FOR NO KEY UPDATE", s.ID).Scan(&state, &s.Status, &s.Summary); e != nil {
		return e
	}
	json.Unmarshal(state, &s.State)
	if s.Status == "completed" {
		return c.JSON(s.Summary)
	}
	if s.State["live_active"] == true {
		return fail(c, 409, "หยุด Live ก่อนจบ session")
	}
	mastered := s.Mode == "lesson" && s.LessonID != nil && number(s.State["independent"], 0) >= 2
	if mastered {
		if _, e = tx.Exec(c.UserContext(), "UPDATE mastery SET completed=true,updated_at=now() WHERE user_id=$1 AND lesson_id=$2", user(c).ID, *s.LessonID); e != nil {
			return e
		}
	}
	if s.LessonID != nil {
		l, err := a.contextLesson(c, s.LessonID)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(c.UserContext(), "INSERT INTO review_items(id,user_id,key,kind,prompt,target,meaning,due_at) VALUES($1,$2,$3,'pattern',$4,$5,$4,now()+interval '1 day') ON CONFLICT(user_id,key) DO NOTHING", uuid.NewString(), user(c).ID, "pattern:"+l.ID, l.Meaning, l.Example); err != nil {
			return err
		}
		for _, w := range l.Vocabulary {
			if _, err = tx.Exec(c.UserContext(), "INSERT INTO vocabulary(id,user_id,term,meaning,example) VALUES($1,$2,$3,$4,$5) ON CONFLICT(user_id,term) DO NOTHING", uuid.NewString(), user(c).ID, w.Term, w.Meaning, w.Example); err != nil {
				return err
			}
		}
	}

	summary := fiber.Map{"attempts": n, "independent": number(s.State["independent"], 0), "mastered": mastered, "message": "ฝึกเสร็จแล้ว กลับมาทบทวนประโยคที่ยังติดขัดได้เสมอ"}
	if s.Mode == "placement" {
		var audios int
		tx.QueryRow(c.UserContext(), "SELECT count(*) FROM attempts WHERE session_id=$1 AND input_kind='audio' AND (feedback->>'audio_clear')::boolean", s.ID).Scan(&audios)
		if audios < 5 {
			return fail(c, 409, "placement ต้องตอบด้วยเสียงที่ชัดอย่างน้อย 5 ข้อ")
		}
		level := textValue(s.State["estimated_level"])
		if level != "A1" && level != "A2" && level != "B1" && level != "B2" {
			level = "Pre-A1"
		}
		if _, e = tx.Exec(c.UserContext(), "UPDATE users SET profile=profile || $1::jsonb WHERE id=$2", asJSON(fiber.Map{"level": level, "onboarded": true}), user(c).ID); e != nil {
			return e
		}
		summary["level"] = level
	}
	if _, e = tx.Exec(c.UserContext(), "UPDATE learning_sessions SET status='completed',summary=$1,updated_at=now() WHERE id=$2", asJSON(summary), s.ID); e != nil {
		return e
	}
	if s.Mode == "live" || s.ScenarioID != nil {
		_, e = tx.Exec(c.UserContext(), "INSERT INTO jobs(id,user_id,kind,request_key,payload) VALUES($1,$2,'summary',$3,$4) ON CONFLICT(user_id,request_key) DO NOTHING", uuid.NewString(), user(c).ID, "summary:"+s.ID, asJSON(fiber.Map{"session_id": s.ID}))
		if e != nil {
			return e
		}
	}
	if e = tx.Commit(c.UserContext()); e != nil {
		return e
	}
	return c.JSON(summary)
}
