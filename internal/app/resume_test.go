package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"tokoloop/internal/config"
	"tokoloop/internal/gemini"
	"tokoloop/internal/learning"
	"tokoloop/internal/security"
)

// TestLessonResume covers the learner-facing cursor contract without contacting Gemini.
func TestLessonResume(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a dedicated toko_*_test database")
	}
	if !strings.Contains(dsn, "_test") {
		t.Fatal("refusing non-test DB")
	}
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("GEMINI_API_KEY", "test-key")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.AudioDir = t.TempDir()
	a, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.DB.Close()
	if _, err = a.DB.Exec(context.Background(), "TRUNCATE users CASCADE"); err != nil {
		t.Fatal(err)
	}
	if err = a.seed(context.Background()); err != nil {
		t.Fatal(err)
	}

	feedback, _ := json.Marshal(learning.Feedback{
		Transcript:  "Hello, I'm Pim.",
		Reply:       "Nice to meet you.",
		Meaning:     "สื่อสารได้",
		Correct:     true,
		GoalMet:     true,
		Corrections: []learning.Correction{},
		Weaknesses:  []string{},
		Vocabulary:  []string{},
		Level:       "Pre-A1",
	})
	fakeGemini := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&map[string]any{})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []any{map[string]any{
				"finishReason": "STOP",
				"content":      map[string]any{"parts": []any{map[string]any{"text": string(feedback)}}},
			}},
			"usageMetadata": map[string]any{"promptTokenCount": 10, "candidatesTokenCount": 10},
		})
	}))
	defer fakeGemini.Close()
	a.AI = &gemini.Client{Key: "test-key", HTTP: fakeGemini.Client(), BaseURL: fakeGemini.URL + "/", Models: cfg.Models, DefaultTimeout: cfg.TimeoutSeconds}

	type caller struct{ token string }
	call := func(want int, method, path string, c caller, body any) any {
		t.Helper()
		var payload []byte
		if body != nil {
			payload = asJSON(body)
		}
		req := httptest.NewRequest(method, "http://localhost/ai-tutor/api/v2"+path, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		res, err := a.HTTP.Test(req, 60000)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("%s %s: got %d, want %d: %s", method, path, res.StatusCode, want, data)
		}
		var value any
		if err := json.Unmarshal(data, &value); err != nil {
			t.Fatalf("decode %s: %v", data, err)
		}
		return value
	}
	object := func(v any) map[string]any {
		t.Helper()
		m, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("want object, got %#v", v)
		}
		return m
	}
	lessonID := func(plan any) string {
		t.Helper()
		lesson, ok := object(plan)["lesson"].(map[string]any)
		if !ok {
			t.Fatalf("daily plan has no lesson: %#v", plan)
		}
		return textValue(lesson["id"])
	}
	activeID := func(plan any) string { return textValue(object(plan)["active_session_id"]) }
	curriculumCard := func(c caller, id string) map[string]any {
		t.Helper()
		cards, ok := call(200, "GET", "/curriculum", c, nil).([]any)
		if !ok {
			t.Fatal("curriculum is not an array")
		}
		for _, card := range cards {
			m := object(card)
			if textValue(m["id"]) == id {
				return m
			}
		}
		t.Fatalf("missing curriculum card %s", id)
		return nil
	}
	startLesson := func(c caller, id string, want int) string {
		t.Helper()
		return textValue(object(call(want, "POST", "/sessions", c, fiber.Map{"mode": "lesson", "lesson_id": id}))["id"])
	}
	snapshot := func(c caller, sessionID string) map[string]any {
		t.Helper()
		return object(call(200, "GET", "/sessions/"+sessionID, c, nil))
	}

	learnerID, otherID := uuid.NewString(), uuid.NewString()
	for _, account := range []struct{ id, username string }{{learnerID, "resume-learner"}, {otherID, "resume-other"}} {
		if _, err := a.DB.Exec(context.Background(), "INSERT INTO users(id,username,password_hash) VALUES($1,$2,$3)", account.id, account.username, security.Hash("resume-test-password")); err != nil {
			t.Fatal(err)
		}
	}
	login := func(username string) caller {
		t.Helper()
		return caller{token: textValue(object(call(200, "POST", "/auth/login", caller{}, fiber.Map{"username": username, "password": "resume-test-password"}))["token"])}
	}
	learner, other := login("resume-learner"), login("resume-other")

	// Empty lessons remain resumable; only an actual attempt can complete them.
	lesson001 := startLesson(learner, "lesson-001", 201)
	call(409, "POST", "/sessions/"+lesson001+"/complete", learner, fiber.Map{})
	if got := textValue(object(snapshot(learner, lesson001)["session"])["status"]); got != "active" {
		t.Fatalf("empty completion changed session status to %q", got)
	}
	if got := activeID(call(200, "GET", "/daily-plan", learner, nil)); got != lesson001 {
		t.Fatalf("empty lesson stopped being resumable: got active %q", got)
	}

	// A typed response after a hint is studied but cannot satisfy audio independence/mastery.
	call(200, "POST", "/sessions/"+lesson001+"/advance", learner, fiber.Map{})
	call(200, "POST", "/sessions/"+lesson001+"/hints", learner, fiber.Map{})
	call(200, "POST", "/sessions/"+lesson001+"/turns", learner, fiber.Map{"text": "Hello, I'm Pim.", "request_id": uuid.NewString()})
	firstCompletion := call(200, "POST", "/sessions/"+lesson001+"/complete", learner, fiber.Map{})
	if object(firstCompletion)["mastered"] != false {
		t.Fatalf("typed assisted lesson unexpectedly mastered: %#v", firstCompletion)
	}
	secondCompletion := call(200, "POST", "/sessions/"+lesson001+"/complete", learner, fiber.Map{})
	if !reflect.DeepEqual(firstCompletion, secondCompletion) {
		t.Fatalf("completion is not idempotent: first=%#v second=%#v", firstCompletion, secondCompletion)
	}
	card001 := curriculumCard(learner, "lesson-001")
	if card001["studied"] != true || card001["completed"] != false {
		t.Fatalf("lesson 001 should be studied without mastery: %#v", card001)
	}
	if got := lessonID(call(200, "GET", "/daily-plan", learner, nil)); got != "lesson-002" {
		t.Fatalf("daily plan after lesson 001 = %q, want lesson-002", got)
	}

	// Starting an active lesson again is a resume operation: no new session, state, or turns.
	lesson002 := startLesson(learner, "lesson-002", 201)
	call(200, "POST", "/sessions/"+lesson002+"/advance", learner, fiber.Map{})
	call(200, "POST", "/sessions/"+lesson002+"/turns", learner, fiber.Map{"text": "You can call me Pim.", "request_id": uuid.NewString()})
	beforeResume := snapshot(learner, lesson002)
	if got := startLesson(learner, "lesson-002", 200); got != lesson002 {
		t.Fatalf("second lesson start created a new session: got %q want %q", got, lesson002)
	}
	afterResume := snapshot(learner, lesson002)
	if !reflect.DeepEqual(beforeResume["session"], afterResume["session"]) || !reflect.DeepEqual(beforeResume["turns"], afterResume["turns"]) {
		t.Fatal("resume changed the stored session state or turns")
	}
	if plan := call(200, "GET", "/daily-plan", learner, nil); lessonID(plan) != "lesson-002" || activeID(plan) != lesson002 {
		t.Fatalf("daily plan did not return the active lesson 002: %#v", plan)
	}
	// A fresh login simulates a reload and must recover the same selected session.
	learnerReloaded := login("resume-learner")
	if plan := call(200, "GET", "/daily-plan", learnerReloaded, nil); lessonID(plan) != "lesson-002" || activeID(plan) != lesson002 {
		t.Fatalf("reload lost lesson 002 resume state: %#v", plan)
	}
	call(200, "POST", "/sessions/"+lesson002+"/complete", learnerReloaded, fiber.Map{})
	if got := lessonID(call(200, "GET", "/daily-plan", learnerReloaded, nil)); got != "lesson-003" {
		t.Fatalf("daily plan after lesson 002 = %q, want lesson-003", got)
	}

	// A newer free-talk session and an older parallel lesson cannot override the selected completion cursor.
	oldParallel := uuid.NewString()
	if _, err := a.DB.Exec(context.Background(), "INSERT INTO learning_sessions(id,user_id,lesson_id,mode,state,model_version,updated_at) VALUES($1,$2,'lesson-001','lesson',$3,'test',now()+interval '1 minute')", oldParallel, learnerID, asJSON(fiber.Map{"stage": "pattern", "step": 0})); err != nil {
		t.Fatal(err)
	}
	call(201, "POST", "/sessions", learnerReloaded, fiber.Map{"mode": "free"})
	if got := lessonID(call(200, "GET", "/daily-plan", learnerReloaded, nil)); got != "lesson-003" {
		t.Fatalf("parallel/free session replaced the cursor: daily lesson = %q", got)
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE learning_sessions SET status='completed' WHERE id=$1", oldParallel); err != nil {
		t.Fatal(err)
	}

	// A deliberate replay creates a separate active session while preserving prior studied evidence.
	replay001 := startLesson(learnerReloaded, "lesson-001", 201)
	if replay001 == lesson001 {
		t.Fatal("replaying a completed lesson reused the completed session")
	}
	if card := curriculumCard(learnerReloaded, "lesson-001"); card["studied"] != true || card["completed"] != false {
		t.Fatalf("replay reset historical studied state: %#v", card)
	}

	// A prior completed lesson with an attempt is studied even when it predates learning_cursor.
	legacySession := uuid.NewString()
	if _, err := a.DB.Exec(context.Background(), "INSERT INTO learning_sessions(id,user_id,lesson_id,mode,status,state,summary,model_version) VALUES($1,$2,'lesson-001','lesson','completed',$3,$4,'legacy')", legacySession, otherID, asJSON(fiber.Map{"stage": "conversation", "step": 0}), asJSON(fiber.Map{"mastered": false})); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(context.Background(), "INSERT INTO attempts(id,session_id,user_id,request_id,input_kind,transcript,feedback) VALUES($1,$2,$3,$4,'text','Hello',$5)", uuid.NewString(), legacySession, otherID, "legacy-history", asJSON(fiber.Map{"correct": true})); err != nil {
		t.Fatal(err)
	}
	if got := lessonID(call(200, "GET", "/daily-plan", other, nil)); got != "lesson-002" {
		t.Fatalf("legacy completed history was not inferred as studied: next=%q", got)
	}
	if card := curriculumCard(other, "lesson-001"); card["studied"] != true || card["completed"] != false {
		t.Fatalf("legacy history changed mastery or was ignored: %#v", card)
	}
	call(404, "GET", "/sessions/"+lesson001, other, nil)
	var cursorCount int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM learning_cursor WHERE user_id=$1", otherID).Scan(&cursorCount); err != nil {
		t.Fatal(err)
	}
	if cursorCount != 0 {
		t.Fatalf("another user's activity created a cursor: count=%d", cursorCount)
	}
}
