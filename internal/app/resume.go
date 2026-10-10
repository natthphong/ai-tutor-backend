package app

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"tokoloop/internal/ebook"
)

// A learner may finish a lesson with help. Mastery still requires independent speech.
const studiedSQL = `(coalesce(m.completed,false) OR EXISTS(SELECT 1 FROM learning_sessions done WHERE done.user_id=$1 AND done.lesson_id=l.id AND done.mode='lesson' AND done.status='completed' AND EXISTS(SELECT 1 FROM attempts a WHERE a.session_id=done.id)))`
const lessonOrderSQL = `CASE l.data->>'level' WHEN 'Pre-A1' THEN 0 WHEN 'A1' THEN 1 WHEN 'A2' THEN 2 WHEN 'B1' THEN 3 ELSE 4 END,(l.data->>'unit')::int,l.ordinal`

func (a *App) daily(c *fiber.Ctx) error {
	u := user(c)
	var due int
	if e := a.DB.QueryRow(c.UserContext(), "SELECT count(*) FROM review_items WHERE user_id=$1 AND due_at<=now()", u.ID).Scan(&due); e != nil {
		return e
	}
	var active *string
	var lesson []byte
	// Only lesson selections drive curriculum continuation; free talk and old parallel sessions cannot hijack it.
	var selectedID, selectedLesson, status string
	e := a.DB.QueryRow(c.UserContext(), `SELECT s.id::text,s.lesson_id,s.status FROM learning_sessions s LEFT JOIN learning_cursor c ON c.session_id=s.id AND c.user_id=s.user_id WHERE s.user_id=$1 AND s.mode='lesson' AND s.lesson_id IS NOT NULL ORDER BY (c.session_id IS NOT NULL) DESC,s.updated_at DESC LIMIT 1`, u.ID).Scan(&selectedID, &selectedLesson, &status)
	if e != nil && e != pgx.ErrNoRows {
		return e
	}
	if e == nil && status == "active" {
		active = &selectedID
		if e = a.DB.QueryRow(c.UserContext(), "SELECT data FROM lessons WHERE id=$1", selectedLesson).Scan(&lesson); e != nil {
			return e
		}
	} else {
		// Prefer the next unvisited lesson after the explicit selection; wrap only if the later path is complete.
		q := `WITH path AS(SELECT l.id,l.data,row_number() OVER(ORDER BY ` + lessonOrderSQL + `) AS position,` + studiedSQL + ` AS studied FROM lessons l LEFT JOIN mastery m ON m.lesson_id=l.id AND m.user_id=$1), anchor AS(SELECT position FROM path WHERE id=$2) SELECT data FROM path WHERE NOT studied AND (CASE data->>'level' WHEN 'Pre-A1' THEN 0 WHEN 'A1' THEN 1 WHEN 'A2' THEN 2 WHEN 'B1' THEN 3 ELSE 4 END)>=CASE $3 WHEN 'A1' THEN 1 WHEN 'A2' THEN 2 WHEN 'B1' THEN 3 WHEN 'B2' THEN 4 ELSE 0 END ORDER BY CASE WHEN position>coalesce((SELECT position FROM anchor),0) THEN 0 ELSE 1 END,position LIMIT 1`
		// Explicitly selected lower-level lessons can continue at that level, regardless of placement.
		level := textValue(u.Profile["level"])
		if selectedLesson != "" {
			if e = a.DB.QueryRow(c.UserContext(), "SELECT data->>'level' FROM lessons WHERE id=$1", selectedLesson).Scan(&level); e != nil {
				return e
			}
		}
		e = a.DB.QueryRow(c.UserContext(), q, u.ID, selectedLesson, level).Scan(&lesson)
		if e != nil && e != pgx.ErrNoRows {
			return e
		}
	}
	var l any
	if len(lesson) > 0 {
		json.Unmarshal(lesson, &l)
	}
	plan := fiber.Map{"lesson": l, "due_count": due, "active_session_id": active, "minutes": number(u.Profile["daily_minutes"], 30), "blocks": []fiber.Map{{"kind": "review", "minutes": 5}, {"kind": "pattern", "minutes": 5}, {"kind": "drill", "minutes": 8}, {"kind": "conversation", "minutes": 10}, {"kind": "summary", "minutes": 2}}}
	if a.Course != nil {
		ebookPlan, err := a.ebookDailyPlan(c, due)
		if err != nil {
			return err
		}
		plan["ebook"] = ebookPlan
	}
	return c.JSON(plan)
}

// ebookDailyPlan is the Learn Ebook part of today's plan: review what is due
// first (capped), then continue the next unit on the A1-first learning path.
func (a *App) ebookDailyPlan(c *fiber.Ctx, due int) (fiber.Map, error) {
	uid := user(c).ID
	current, err := a.ebookCourseProgressRows(c.UserContext(), uid, ebook.LearnEbookCourseVersion)
	if err != nil {
		return nil, err
	}
	previous, err := a.ebookCoursePreviousStates(c.UserContext(), uid)
	if err != nil {
		return nil, err
	}
	completedAll := map[string]bool{}
	statuses := map[string]string{}
	percents := map[string]int{}
	for _, lesson := range a.Course.Lessons {
		id := ebookCourseStorageID(lesson)
		state := current[id]
		if state == nil {
			state = previous[id]
		}
		status, percent, _ := ebookCourseStatus(state, false)
		statuses[lesson.ID] = status
		percents[lesson.ID] = percent
		completedAll[lesson.ID] = percent == 100
	}
	summary := ebookCoursePathSummary(a.Course, statuses, completedAll)
	plan := fiber.Map{"total": summary["total"], "learned": summary["learned"], "levels": summary["levels"], "review_limit": reviewSessionLimit, "review_today": min(due, reviewSessionLimit)}
	if next, ok := a.Course.Lesson(ebookCourseRecommended(a.Course, completedAll)); ok {
		plan["next_unit"] = fiber.Map{"id": next.ID, "number": next.Ordinal, "title": next.Title, "title_th": next.TitleTH, "level": next.Level, "path_order": next.PathOrder, "theme_th": next.ThemeTH, "status": statuses[next.ID], "percent": percents[next.ID]}
	}
	return plan, nil
}
