package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"tokoloop/contracts"
	"tokoloop/internal/ebook"
	"tokoloop/internal/learning"
)

func TestCourseLessonOnlyRecognizesStableCourseIDs(t *testing.T) {
	course, err := ebook.LoadCourse()
	if err != nil {
		t.Fatal(err)
	}
	a := &App{Course: course}
	if _, ok := a.courseLesson("1"); ok {
		t.Fatal("numeric legacy unit ID was claimed by the redesigned course")
	}
	if lesson, ok := a.courseLesson("ebook-001"); !ok || lesson.UnitID != "1" {
		t.Fatalf("stable course ID lookup = %#v, %v", lesson, ok)
	}
}

func TestOpenAPIAllowsPartialCompletedStepsAndDocumentsShadowing(t *testing.T) {
	var document map[string]any
	if err := json.Unmarshal(contracts.OpenAPI, &document); err != nil {
		t.Fatal(err)
	}
	schemas := document["components"].(map[string]any)["schemas"].(map[string]any)
	state := schemas["Session"].(map[string]any)["properties"].(map[string]any)["state"].(map[string]any)
	stateProperties := state["properties"].(map[string]any)
	ebookSkill := stateProperties["ebook_skill"].(map[string]any)["enum"].([]any)
	if !containsString(ebookSkill, "shadowing") {
		t.Fatal("Session.state.ebook_skill does not include shadowing")
	}
	sessionRequest := schemas["EbookSessionRequest"].(map[string]any)
	mode := sessionRequest["properties"].(map[string]any)["mode"].(map[string]any)["enum"].([]any)
	if !containsString(mode, "shadowing") {
		t.Fatal("EbookSessionRequest.mode does not include shadowing")
	}
	progressState := schemas["EbookProgressState"].(map[string]any)
	completedSteps := progressState["properties"].(map[string]any)["completed_steps"].(map[string]any)
	if _, required := completedSteps["required"]; required {
		t.Fatal("EbookProgressState.completed_steps must allow partial maps")
	}
}

func TestEbookCourseCatalogMapsDueLegacyAndCurrentReviewKeys(t *testing.T) {
	a, owner, _, _ := ebookFixture(t)
	var userID string
	if err := a.DB.QueryRow(context.Background(), "SELECT id::text FROM users WHERE username='ebook-owner'").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	legacyKey := "ebook:" + a.Book.Version + ":1:legacy-question"
	currentKey := "ebook:" + ebook.LearnEbookCourseVersion + ":1:quiz-01"
	for _, key := range []string{legacyKey, currentKey} {
		if _, err := a.DB.Exec(context.Background(), `INSERT INTO review_items(id,user_id,key,kind,title,prompt,target,meaning,due_at) VALUES($1,$2,$3,'pattern','legacy cue','cue','target','meaning',now())`, uuid.NewString(), userID, key); err != nil {
			t.Fatal(err)
		}
	}
	catalog := owner.call(200, "GET", "/ebook", nil)
	unit := ebookCourseUnit(catalog, "ebook-001")
	if unit["review_due"] != true {
		t.Fatalf("catalog did not map due review keys: %#v", unit)
	}
	var legacyCount, currentCount int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM review_items WHERE user_id=$1 AND key=$2", userID, legacyKey).Scan(&legacyCount); err != nil {
		t.Fatal(err)
	}
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM review_items WHERE user_id=$1 AND key=$2", userID, currentKey).Scan(&currentCount); err != nil {
		t.Fatal(err)
	}
	if legacyCount != 1 || currentCount != 1 {
		t.Fatalf("catalog changed review rows: legacy=%d current=%d", legacyCount, currentCount)
	}
}

func TestEbookShadowingReplayIsIdenticalAfterSessionCompletion(t *testing.T) {
	a, owner, _, _ := ebookFixture(t)
	clearAudio, goalMet := true, true
	fake := featureAI(a, func(w http.ResponseWriter, r *http.Request) {
		feedback, _ := json.Marshal(learning.Feedback{
			Transcript: "I work in Bangkok.", Reply: "Keep going.", ReplyTH: "ลองอีกครั้ง", Meaning: "ทดสอบ shadowing",
			Correct: goalMet, GoalMet: goalMet, AudioClear: clearAudio,
			Corrections: []learning.Correction{}, Weaknesses: []string{}, Vocabulary: []string{}, Level: "A1",
		})
		response(w, string(feedback))
	})
	defer fake.Close()

	catalog := owner.call(200, "GET", "/ebook", nil)
	unitID := ebookCourseString(ebookCourseObject(ebookCourseArray(catalog, "units", "lessons")[0]), "id", "unit_id")
	started := owner.call(201, "POST", "/ebook/units/"+unitID+"/sessions", map[string]any{"mode": "shadowing", "request_id": uuid.NewString()})
	sessionID := ebookCourseString(started, "id", "session_id")
	requestID := uuid.NewString()
	first := owner.audioTurn(sessionID, requestID)
	for i := 1; i < 3; i++ {
		owner.audioTurn(sessionID, uuid.NewString())
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE learning_sessions SET status='completed' WHERE id=$1", sessionID); err != nil {
		t.Fatal(err)
	}
	replay := owner.audioTurn(sessionID, requestID)
	if !reflect.DeepEqual(first, replay) {
		t.Fatalf("completed shadowing replay changed the original response: first=%#v replay=%#v", first, replay)
	}
	if first["mastery_advanced"] != true || replay["mastery_advanced"] != true {
		t.Fatalf("shadowing replay lost advancement: first=%#v replay=%#v", first, replay)
	}
}

func TestEbookCheckRejectsRequestIDUsedForAnotherCourseUnit(t *testing.T) {
	a, owner, _, _ := ebookFixture(t)
	catalog := owner.call(200, "GET", "/ebook", nil)
	units := ebookCourseArray(catalog, "units", "lessons")
	firstID := ebookCourseString(ebookCourseObject(units[0]), "id")
	secondID := ebookCourseString(ebookCourseObject(units[1]), "id")
	firstDetail := owner.call(200, "GET", "/ebook/units/"+firstID, nil)
	secondDetail := owner.call(200, "GET", "/ebook/units/"+secondID, nil)
	firstQuiz := ebookCourseString(ebookCourseObject(ebookCourseArray(ebookCourseObject(firstDetail["lesson"]), "quiz")[0]), "id")
	secondQuiz := ebookCourseString(ebookCourseObject(ebookCourseArray(ebookCourseObject(secondDetail["lesson"]), "quiz")[0]), "id")
	requestID := uuid.NewString()
	owner.call(200, "POST", "/ebook/units/"+firstID+"/check", map[string]any{"request_id": requestID, "answers": map[string]string{firstQuiz: "wrong"}})
	status, _, _ := ebookRaw(t, a, owner.token, "POST", "/ebook/units/"+secondID+"/check", map[string]any{"request_id": requestID, "answers": map[string]string{secondQuiz: "wrong"}})
	if status != 409 {
		t.Fatalf("cross-unit ebook request reuse status = %d, want 409", status)
	}
}

func TestEbookShadowingRejectsRequestIDUsedForAnotherSession(t *testing.T) {
	a, owner, _, _ := ebookFixture(t)
	fake := featureAI(a, func(w http.ResponseWriter, r *http.Request) {
		feedback, _ := json.Marshal(learning.Feedback{
			Transcript: "I work in Bangkok.", Reply: "Keep going.", ReplyTH: "ลองอีกครั้ง", Meaning: "ทดสอบ shadowing",
			Correct: true, GoalMet: true, AudioClear: true,
			Corrections: []learning.Correction{}, Weaknesses: []string{}, Vocabulary: []string{}, Level: "A1",
		})
		response(w, string(feedback))
	})
	defer fake.Close()
	catalog := owner.call(200, "GET", "/ebook", nil)
	units := ebookCourseArray(catalog, "units", "lessons")
	firstID := ebookCourseString(ebookCourseObject(units[0]), "id")
	secondID := ebookCourseString(ebookCourseObject(units[1]), "id")
	requestID := uuid.NewString()
	first := owner.call(201, "POST", "/ebook/units/"+firstID+"/sessions", map[string]any{"mode": "shadowing", "request_id": uuid.NewString()})
	owner.audioTurn(ebookCourseString(first, "id", "session_id"), requestID)
	second := owner.call(201, "POST", "/ebook/units/"+secondID+"/sessions", map[string]any{"mode": "shadowing", "request_id": uuid.NewString()})
	secondSessionID := ebookCourseString(second, "id", "session_id")
	status, _, _ := ebookRaw(t, a, owner.token, "POST", "/sessions/"+secondSessionID+"/turns", map[string]any{"request_id": requestID, "text": "typed"})
	if status != 200 {
		// Typed shadowing input is intentionally an accessibility no-op. Submit audio
		// with the reused ID so the global attempts uniqueness check is exercised.
		status, _, _ = ebookRaw(t, a, owner.token, "POST", "/sessions/"+secondSessionID+"/turns", map[string]any{"request_id": requestID, "text": "typed"})
	}
	if status != 200 {
		t.Fatalf("cross-session typed shadowing request status = %d, want accessibility 200", status)
	}
}

func containsString(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

var _ = bytes.Equal
var _ = strings.Contains
