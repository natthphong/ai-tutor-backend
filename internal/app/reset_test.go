package app

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"tokoloop/internal/ebook"
)

func resetUserID(t *testing.T, a *App, username string) string {
	t.Helper()
	var id string
	if err := a.DB.QueryRow(context.Background(), "SELECT id::text FROM users WHERE username=$1", username).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func resetCount(t *testing.T, a *App, table, uid string) int {
	t.Helper()
	query := "SELECT count(*) FROM " + table + " WHERE user_id=$1"
	if table == "turns" {
		query = "SELECT count(*) FROM turns WHERE session_id IN (SELECT id FROM learning_sessions WHERE user_id=$1)"
	}
	var n int
	if err := a.DB.QueryRow(context.Background(), query, uid).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// resetSeed gives a learner a finished Ebook unit (sessions, attempts, turns,
// scheduled reviews, library words), one saved word, and one usage row.
func resetSeed(t *testing.T, a *App, client featureHTTP, uid string) ebook.Lesson {
	t.Helper()
	lesson := a.Course.Path()[0]
	ebookCourseFinishUnit(t, a, client, lesson)
	client.call(201, "POST", "/vocabulary", map[string]any{"term": "reset-word", "meaning": "คำทดสอบ", "example": "A reset word."})
	client.call(200, "PATCH", "/ebook/units/"+lesson.ID+"/progress", map[string]any{"vocabulary_saved": map[string]bool{"reset-word": true}})
	for _, table := range []string{"review_items", "vocabulary", "ebook_progress", "learning_sessions", "attempts", "turns", "usage"} {
		if resetCount(t, a, table, uid) == 0 {
			t.Fatalf("seed left %s empty; the reset test would prove nothing", table)
		}
	}
	return lesson
}

func TestAccountResetNeedsTheTypedWordAndAScope(t *testing.T) {
	a, owner, _, _ := ebookFixture(t)
	defer ebookCoursePassingAI(a)()
	uid := resetUserID(t, a, "ebook-owner")
	resetSeed(t, a, owner, uid)
	before := resetCount(t, a, "review_items", uid)
	for _, body := range []map[string]any{
		{"scope": "all"},
		{"scope": "all", "confirm": "yes"},
		{"scope": "all", "confirm": "delete everything"},
		{"confirm": "delete"},
		{"scope": "everything", "confirm": "delete"},
	} {
		if status, _, _ := ebookRaw(t, a, owner.token, "POST", "/account/reset", body); status != 400 {
			t.Fatalf("reset with %v returned %d, want 400", body, status)
		}
	}
	// The route is rate limited; a sixth request inside a minute is refused
	// even when it is well formed.
	if status, _, _ := ebookRaw(t, a, owner.token, "POST", "/account/reset", map[string]any{"scope": "all", "confirm": "delete"}); status != 429 {
		t.Fatalf("sixth reset request in a minute returned %d, want 429", status)
	}
	if status, _, _ := ebookRaw(t, a, "", "POST", "/account/reset", map[string]any{"scope": "all", "confirm": "delete"}); status != 401 {
		t.Fatalf("reset without a login returned %d, want 401", status)
	}
	if after := resetCount(t, a, "review_items", uid); after != before || resetCount(t, a, "vocabulary", uid) == 0 {
		t.Fatalf("a rejected reset still removed data: review_items %d -> %d", before, after)
	}
}

func TestAccountResetLibraryClearsCollectionAndReviewsButKeepsLessons(t *testing.T) {
	a, owner, other, _ := ebookFixture(t)
	defer ebookCoursePassingAI(a)()
	uid, otherID := resetUserID(t, a, "ebook-owner"), resetUserID(t, a, "ebook-other")
	lesson := resetSeed(t, a, owner, uid)
	other.call(201, "POST", "/vocabulary", map[string]any{"term": "other-word", "meaning": "ของอีกคน", "example": "Another learner."})
	sessions, attempts, usage := resetCount(t, a, "learning_sessions", uid), resetCount(t, a, "attempts", uid), resetCount(t, a, "usage", uid)

	// The word is typed by hand, so case and spaces around it do not matter.
	result := owner.call(200, "POST", "/account/reset", map[string]any{"scope": "library", "confirm": "  DELETE "})
	if result["reset"] != true || ebookCourseString(result, "scope") != "library" {
		t.Fatalf("reset result = %#v", result)
	}
	for _, table := range []string{"review_items", "review_events", "vocabulary"} {
		if n := resetCount(t, a, table, uid); n != 0 {
			t.Fatalf("library reset left %d rows in %s", n, table)
		}
	}
	if resetCount(t, a, "learning_sessions", uid) != sessions || resetCount(t, a, "attempts", uid) != attempts || resetCount(t, a, "usage", uid) != usage {
		t.Fatal("library reset removed practice history or the usage ledger")
	}
	unit := ebookCourseUnit(owner.call(200, "GET", "/ebook", nil), lesson.ID)
	if ebookCourseString(unit, "status") != "learned" || ebookCourseNumber(unit, "percent") != 100 {
		t.Fatalf("library reset changed lesson progress: %#v", unit)
	}
	progress := ebookCourseObject(owner.call(200, "GET", "/ebook/units/"+lesson.ID, nil)["progress"])
	if _, kept := progress["vocabulary_saved"]; kept {
		t.Fatalf("saved-word badges still point at deleted vocabulary: %#v", progress["vocabulary_saved"])
	}
	// A finished unit is not scheduled again just because it is opened.
	owner.call(200, "PATCH", "/ebook/units/"+lesson.ID+"/progress", map[string]any{"current_step": 5})
	if n := resetCount(t, a, "review_items", uid); n != 0 {
		t.Fatalf("opening a finished unit after the reset scheduled %d reviews again", n)
	}
	if due := owner.call(200, "GET", "/daily-plan", nil)["due_count"]; int(due.(float64)) != 0 {
		t.Fatalf("due count after the reset = %v, want 0", due)
	}
	library := owner.call(200, "GET", "/library", nil)
	if len(ebookCourseArray(library, "vocabulary")) != 0 || len(ebookCourseArray(library, "mistakes")) != 0 {
		t.Fatalf("My collection is not empty after the reset: %#v", library)
	}
	if resetCount(t, a, "vocabulary", otherID) != 1 {
		t.Fatal("the reset touched another learner's vocabulary")
	}
}

func TestAccountResetAllStartsOverAndKeepsAccountAndUsage(t *testing.T) {
	a, owner, other, _ := ebookFixture(t)
	defer ebookCoursePassingAI(a)()
	uid, otherID := resetUserID(t, a, "ebook-owner"), resetUserID(t, a, "ebook-other")
	lesson := resetSeed(t, a, owner, uid)
	otherLesson := a.Course.Path()[1]
	other.call(200, "PATCH", "/ebook/units/"+otherLesson.ID+"/progress", map[string]any{"current_step": 2, "completed_steps": map[string]bool{"understand": true}})
	other.call(201, "POST", "/vocabulary", map[string]any{"term": "other-word", "meaning": "ของอีกคน", "example": "Another learner."})
	other.call(201, "POST", "/ebook/units/"+otherLesson.ID+"/sessions", map[string]any{"mode": "speak", "request_id": uuid.NewString()})
	usage := resetCount(t, a, "usage", uid)
	var spent float64
	if err := a.DB.QueryRow(context.Background(), "SELECT coalesce(sum(cost_thb+reserved_thb),0) FROM usage WHERE user_id=$1", uid).Scan(&spent); err != nil {
		t.Fatal(err)
	}

	owner.call(200, "POST", "/account/reset", map[string]any{"scope": "all", "confirm": "delete"})
	for _, table := range []string{"review_items", "review_events", "vocabulary", "ebook_progress", "ebook_events", "ebook_cursor", "mastery", "learning_cursor", "learning_sessions", "attempts", "turns", "speech_events", "live_tickets"} {
		if n := resetCount(t, a, table, uid); n != 0 {
			t.Fatalf("full reset left %d rows in %s", n, table)
		}
	}
	// Money already spent still counts against this month's budget.
	var spentAfter float64
	var linked int
	if err := a.DB.QueryRow(context.Background(), "SELECT coalesce(sum(cost_thb+reserved_thb),0),count(*) FILTER(WHERE session_id IS NOT NULL) FROM usage WHERE user_id=$1", uid).Scan(&spentAfter, &linked); err != nil {
		t.Fatal(err)
	}
	if resetCount(t, a, "usage", uid) != usage || spentAfter != spent || linked != 0 {
		t.Fatalf("usage ledger changed: rows %d -> %d, spend %v -> %v, still linked %d", usage, resetCount(t, a, "usage", uid), spent, spentAfter, linked)
	}
	if me := owner.call(200, "GET", "/auth/me", nil); ebookCourseString(me, "username") != "ebook-owner" {
		t.Fatalf("the account did not survive the reset: %#v", me)
	}
	catalog := owner.call(200, "GET", "/ebook", nil)
	if unit := ebookCourseUnit(catalog, lesson.ID); ebookCourseString(unit, "status") != "unlearned" || ebookCourseNumber(unit, "percent") != 0 {
		t.Fatalf("unit after a full reset = %#v, want unlearned at 0%%", unit)
	}
	if ebookCourseString(catalog, "recommended_unit_id") != lesson.ID {
		t.Fatalf("recommended unit after a full reset = %q, want the first unit %q", ebookCourseString(catalog, "recommended_unit_id"), lesson.ID)
	}
	plan := owner.call(200, "GET", "/daily-plan", nil)
	if int(plan["due_count"].(float64)) != 0 || ebookCourseNumber(plan["ebook"], "learned") != 0 {
		t.Fatalf("daily plan after a full reset = %#v", plan)
	}
	// Learning again works and schedules reviews again.
	ebookCourseFinishUnit(t, a, owner, lesson)
	if n := resetCount(t, a, "review_items", uid); n != len(lesson.Review) {
		t.Fatalf("finishing the unit again scheduled %d reviews, want %d", n, len(lesson.Review))
	}

	if resetCount(t, a, "vocabulary", otherID) != 1 || resetCount(t, a, "ebook_progress", otherID) != 1 || resetCount(t, a, "learning_sessions", otherID) != 1 {
		t.Fatal("the reset touched another learner's data")
	}
	if unit := ebookCourseUnit(other.call(200, "GET", "/ebook", nil), otherLesson.ID); ebookCourseString(unit, "status") != "learning" {
		t.Fatalf("another learner's unit changed: %#v", unit)
	}
}
