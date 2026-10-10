package app

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// resetConfirmWord is what the learner must type before a reset runs.
const resetConfirmWord = "delete"

// resetLearning lets a learner start over. It only ever touches rows of the
// signed-in account.
//
// scope "library" clears My collection and the review queue: saved vocabulary,
// mistakes and every scheduled review. Lesson progress stays, and units that
// were already finished are not scheduled again.
//
// scope "all" also clears lesson progress and practice history (Learn Ebook,
// curriculum mastery, sessions, attempts, speaking minutes and streak).
//
// The account, its settings, Daily Meet entries, custom scenarios and the AI
// usage ledger are always kept: spending already happened and still counts
// against this month's budget.
func (a *App) resetLearning(c *fiber.Ctx) error {
	var body struct {
		Confirm string `json:"confirm"`
		Scope   string `json:"scope"`
	}
	if c.BodyParser(&body) != nil || (body.Scope != "library" && body.Scope != "all") {
		return fail(c, 400, "เลือกขอบเขตการรีเซ็ต: library หรือ all")
	}
	if !strings.EqualFold(strings.TrimSpace(body.Confirm), resetConfirmWord) {
		return fail(c, 400, "พิมพ์คำว่า delete เพื่อยืนยันการรีเซ็ต")
	}
	uid := user(c).ID
	ctx := c.UserContext()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Session start takes the same row lock, so a new Ebook session cannot be
	// created while this learner's rows are being cleared.
	if _, err = tx.Exec(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", uid); err != nil {
		return err
	}
	if body.Scope == "all" {
		var live int
		if err = tx.QueryRow(ctx, "SELECT count(*) FROM learning_sessions WHERE user_id=$1 AND status='active' AND state->>'live_active'='true'", uid).Scan(&live); err != nil {
			return err
		}
		if live > 0 {
			return fail(c, 409, "หยุด Live ที่กำลังใช้งานก่อนรีเซ็ต")
		}
	}

	deleted := fiber.Map{}
	remove := func(name, query string) error {
		tag, e := tx.Exec(ctx, query, uid)
		if e != nil {
			return e
		}
		deleted[name] = tag.RowsAffected()
		return nil
	}
	// Order follows the foreign keys: review_events -> review_items -> attempts.
	steps := []struct{ name, query string }{
		{"review_events", "DELETE FROM review_events WHERE user_id=$1"},
		{"review_items", "DELETE FROM review_items WHERE user_id=$1"},
		{"vocabulary", "DELETE FROM vocabulary WHERE user_id=$1"},
	}
	if body.Scope == "all" {
		steps = append(steps, []struct{ name, query string }{
			{"ebook_progress", "DELETE FROM ebook_progress WHERE user_id=$1"},
			{"ebook_events", "DELETE FROM ebook_events WHERE user_id=$1"},
			{"ebook_cursor", "DELETE FROM ebook_cursor WHERE user_id=$1"},
			{"mastery", "DELETE FROM mastery WHERE user_id=$1"},
			{"learning_cursor", "DELETE FROM learning_cursor WHERE user_id=$1"},
			{"live_tickets", "DELETE FROM live_tickets WHERE user_id=$1"},
			{"pending_summaries", "DELETE FROM jobs WHERE user_id=$1 AND kind='summary' AND status='queued'"},
			// The usage ledger is kept; it only loses its link to the session.
			{"usage_unlinked", "UPDATE usage SET session_id=NULL WHERE user_id=$1 AND session_id IS NOT NULL"},
			{"attempts", "DELETE FROM attempts WHERE user_id=$1"},
			{"turns", "DELETE FROM turns WHERE session_id IN (SELECT id FROM learning_sessions WHERE user_id=$1)"},
			{"sessions", "DELETE FROM learning_sessions WHERE user_id=$1"},
			{"speech_events", "DELETE FROM speech_events WHERE user_id=$1"},
		}...)
	} else {
		// Saved-word badges in a kept lesson point at vocabulary that is gone.
		steps = append(steps, struct{ name, query string }{"ebook_saved_word_flags", "UPDATE ebook_progress SET state=state-'vocabulary_saved',updated_at=now() WHERE user_id=$1 AND state ? 'vocabulary_saved'"})
	}
	for _, step := range steps {
		if err = remove(step.name, step.query); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"reset": true, "scope": body.Scope, "deleted": deleted})
}
