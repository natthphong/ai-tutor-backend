package app

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"tokoloop/internal/ebook"
	"tokoloop/internal/learning"
)

func TestEbookCourseStartupAndCatalogContract(t *testing.T) {
	a, owner, other, _ := ebookFixture(t)
	// Production manifests use numeric legacy IDs; keep this regression aligned
	// with the existing cursor that redesigned lesson IDs must resume.
	a.Book.Units[0].ID = "1"
	if a.Course == nil {
		t.Fatal("App.Course is nil; the validated static course must load at startup")
	}

	catalog := owner.call(200, "GET", "/ebook", nil)
	units := ebookCourseArray(catalog, "units", "lessons")
	if len(units) != 145 {
		t.Fatalf("ebook catalog units = %d, want 145", len(units))
	}
	unit := ebookCourseObject(units[0])
	unitID := ebookCourseString(unit, "id", "unit_id")
	if unitID == "" || ebookCourseString(unit, "status") != "unlearned" || ebookCourseNumber(unit, "current_step") != 1 {
		t.Fatalf("new ebook unit summary = %#v", unit)
	}

	detail := owner.call(200, "GET", "/ebook/units/"+unitID, nil)
	if ebookCourseString(detail, "status") != "ready" {
		t.Fatalf("static course detail status = %q, want ready", ebookCourseString(detail, "status"))
	}
	lesson := ebookCourseObject(detail["lesson"])
	if len(ebookCourseArray(lesson, "vocabulary", "words")) != 10 {
		t.Fatalf("lesson vocabulary did not expose exactly ten terms: %#v", lesson)
	}
	for _, rawQuiz := range ebookCourseArray(lesson, "quiz", "quiz_items") {
		quizItem := ebookCourseObject(rawQuiz)
		if answers, exists := quizItem["answers"]; exists && answers != nil {
			if values, ok := answers.([]any); !ok || len(values) > 0 {
				t.Fatalf("sanitized lesson leaked quiz answers: %#v", quizItem)
			}
		}
		if explanation := ebookCourseString(quizItem, "explanation_th"); explanation != "" {
			t.Fatalf("sanitized lesson leaked quiz explanation: %#v", quizItem)
		}
	}

	owner.call(200, "PATCH", "/ebook/units/"+unitID+"/progress", map[string]any{
		"current_step":    2,
		"completed_steps": map[string]bool{"understand": true},
		"page":            1,
	})
	resumedCatalog := owner.call(200, "GET", "/ebook", nil)
	if cursorUnit := ebookCourseString(resumedCatalog["cursor"], "unit_id"); cursorUnit != unitID {
		t.Fatalf("legacy page cursor was not mapped to redesigned lesson id: got %q want %q", cursorUnit, unitID)
	}
	otherDetail := other.call(200, "GET", "/ebook/units/"+unitID, nil)
	if progress := ebookCourseObject(otherDetail["progress"]); len(progress) != 0 {
		t.Fatalf("progress crossed learner boundary: %#v", progress)
	}
	updated := owner.call(200, "GET", "/ebook/units/"+unitID, nil)
	if ebookCourseNumber(ebookCourseObject(updated["progress"]), "current_step") != 2 {
		t.Fatalf("progress did not persist current step: %#v", updated)
	}

	owner.call(200, "PATCH", "/ebook/units/"+unitID+"/progress", map[string]any{
		"current_step": 3, "completed_step": "examples",
	})
	var ownerID string
	if err := a.DB.QueryRow(context.Background(), "SELECT id::text FROM users WHERE username='ebook-owner'").Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	serverCompletion := map[string]any{
		"current_step": 5,
		"completed_steps": map[string]bool{
			"understand": true, "examples": true, "quiz": true, "shadowing": true, "speaking": true,
		},
		"learned": true, "completed_at": "2026-09-20T00:00:00Z",
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE ebook_progress SET state=$1 WHERE user_id=$2 AND unit_id=$3 AND version=$4", asJSON(serverCompletion), ownerID, "1", ebook.LearnEbookCourseVersion); err != nil {
		t.Fatal(err)
	}
	learned := ebookCourseUnit(owner.call(200, "GET", "/ebook", nil), unitID)
	if ebookCourseString(learned, "status") != "learned" || ebookCourseNumber(learned, "percent") != 100 {
		t.Fatalf("completed ebook unit summary = %#v", learned)
	}
	owner.call(200, "PATCH", "/ebook/units/"+unitID+"/progress", map[string]any{"review_requested": true})
	review := ebookCourseUnit(owner.call(200, "GET", "/ebook", nil), unitID)
	if ebookCourseString(review, "status") != "review" {
		t.Fatalf("review request did not derive review status: %#v", review)
	}
}

func TestEbookCourseSpeakingCompletionUsesStableLessonLookup(t *testing.T) {
	a, owner, _, _ := ebookFixture(t)
	// Legacy manifests may use numeric source-book IDs. New-course sessions
	// still need to complete against the stable embedded lesson.
	a.Book.Units[0].ID = "1"
	fake := featureAI(a, func(w http.ResponseWriter, r *http.Request) {
		feedback, _ := json.Marshal(learning.Feedback{
			Transcript: "I am reviewing the dashboard.", Reply: "Good work.", ReplyTH: "ดีมาก", Meaning: "ใช้รูปแบบได้", Correct: true, GoalMet: true, AudioClear: true,
			Corrections: []learning.Correction{}, Weaknesses: []string{}, Vocabulary: []string{}, Level: "A1",
		})
		response(w, string(feedback))
	})
	defer fake.Close()

	catalog := owner.call(200, "GET", "/ebook", nil)
	unitID := ebookCourseString(ebookCourseObject(ebookCourseArray(catalog, "units", "lessons")[0]), "id", "unit_id")
	started := owner.call(201, "POST", "/ebook/units/"+unitID+"/sessions", map[string]any{"mode": "speak", "request_id": uuid.NewString()})
	sessionID := ebookCourseString(started, "id", "session_id")
	owner.audioTurn(sessionID, uuid.NewString())
	owner.audioTurn(sessionID, uuid.NewString())
	progress := ebookCourseObject(owner.call(200, "GET", "/ebook/units/"+unitID, nil)["progress"])
	completed := ebookCourseObject(progress["completed_steps"])
	if completed["speaking"] != true {
		t.Fatalf("new-course speaking completion was not persisted: %#v", progress)
	}
}

func TestEbookCourseQuizIsImmediateOwnerScopedAndIdempotent(t *testing.T) {
	a, owner, other, _ := ebookFixture(t)
	catalog := owner.call(200, "GET", "/ebook", nil)
	unitID := ebookCourseString(ebookCourseObject(ebookCourseArray(catalog, "units", "lessons")[0]), "id", "unit_id")
	detail := owner.call(200, "GET", "/ebook/units/"+unitID, nil)
	lesson := ebookCourseObject(detail["lesson"])
	quiz := ebookCourseArray(lesson, "quiz", "quiz_items")
	if len(quiz) != 5 {
		t.Fatalf("quiz items = %d, want 5", len(quiz))
	}
	quizID := ebookCourseString(ebookCourseObject(quiz[0]), "id", "quiz_id")
	if quizID == "" {
		t.Fatal("quiz item has no stable id")
	}

	requestID := uuid.NewString()
	first := owner.call(200, "POST", "/ebook/units/"+unitID+"/check", map[string]any{
		"request_id": requestID,
		"answers":    map[string]string{quizID: "deliberately submitted answer"},
	})
	replay := owner.call(200, "POST", "/ebook/units/"+unitID+"/check", map[string]any{
		"request_id": requestID,
		"answers":    map[string]string{quizID: "different replay answer"},
	})
	if !reflect.DeepEqual(first, replay) {
		t.Fatalf("quiz request replay changed the persisted result: first=%#v replay=%#v", first, replay)
	}
	var reviewCue string
	if err := a.DB.QueryRow(context.Background(), `SELECT prompt FROM review_items r JOIN users u ON u.id=r.user_id WHERE u.username='ebook-owner' AND r.key LIKE 'ebook:%' ORDER BY r.created_at DESC LIMIT 1`).Scan(&reviewCue); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reviewCue, "ประโยคใดตรงกับความหมาย") || !strings.Contains(reviewCue, "\n1. ") {
		t.Fatalf("review cue does not explain what to recall or show the choices: %q", reviewCue)
	}
	otherDetail := other.call(200, "GET", "/ebook/units/"+unitID, nil)
	if progress := ebookCourseObject(otherDetail["progress"]); len(progress) != 0 {
		t.Fatalf("quiz progress crossed learner boundary: %#v", progress)
	}
}

func TestEbookCourseQuizAcceptsPublicOptionIDAndFlexibleWriting(t *testing.T) {
	a, owner, _, _ := ebookFixture(t)
	fake := featureAI(a, func(w http.ResponseWriter, r *http.Request) {
		response(w, `{"marks":[{"id":"quiz-03","correct":true,"reason_th":"สื่อความหมายถูกและใช้โครงสร้างเป้าหมายแล้ว","answer":"I am reviewing the client dashboard now."}]}`)
	})
	defer fake.Close()

	unitID := ebookCourseString(ebookCourseObject(ebookCourseArray(owner.call(200, "GET", "/ebook", nil), "units", "lessons")[0]), "id", "unit_id")
	detail := owner.call(200, "GET", "/ebook/units/"+unitID, nil)
	questions := ebookCourseArray(detail["pack"], "questions")
	choice := ebookCourseObject(questions[0])
	choiceID := ebookCourseString(choice, "id")
	lesson, ok := a.Course.Lesson(unitID)
	if !ok {
		t.Fatalf("course lesson %q is missing", unitID)
	}
	correctText := ""
	for _, quiz := range lesson.Quiz {
		if quiz.ID == choiceID && len(quiz.Answers) > 0 {
			correctText = quiz.Answers[0]
			break
		}
	}
	optionID := ""
	for _, rawOption := range ebookCourseArray(choice, "options") {
		option := ebookCourseObject(rawOption)
		if ebookCourseString(option, "text") == correctText {
			optionID = ebookCourseString(option, "id")
			break
		}
	}
	if optionID == "" {
		t.Fatalf("public choice omitted the authored answer %q: %#v", correctText, choice)
	}
	choiceResult := owner.call(200, "POST", "/ebook/units/"+unitID+"/check", map[string]any{
		"request_id": uuid.NewString(),
		"answers":    map[string]string{choiceID: optionID},
	})
	choiceMarks := ebookCourseArray(choiceResult, "marks")
	if len(choiceMarks) != 1 || ebookCourseObject(choiceMarks[0])["correct"] != true {
		t.Fatalf("public option id was not accepted as the matching choice: %#v", choiceResult)
	}

	writeResult := owner.call(200, "POST", "/ebook/units/"+unitID+"/check", map[string]any{
		"request_id": uuid.NewString(),
		"answers":    map[string]string{"quiz-03": "I am checking the dashboard right now."},
	})
	writeMarks := ebookCourseArray(writeResult, "marks")
	if len(writeMarks) != 1 || ebookCourseObject(writeMarks[0])["correct"] != true {
		t.Fatalf("flexible grammatical writing was not accepted: %#v", writeResult)
	}
}

func TestEbookCourseRevealPersistsHelpUseForOwnerOnly(t *testing.T) {
	_, owner, other, _ := ebookFixture(t)
	unitID := ebookCourseString(ebookCourseObject(ebookCourseArray(owner.call(200, "GET", "/ebook", nil), "units", "lessons")[0]), "id", "unit_id")
	owner.call(200, "POST", "/ebook/units/"+unitID+"/reveal", map[string]any{"ids": []string{"quiz-03"}})

	ownerProgress := ebookCourseObject(owner.call(200, "GET", "/ebook/units/"+unitID, nil)["progress"])
	revealed := ebookCourseObject(ownerProgress["revealed"])
	if revealed["quiz-03"] != true {
		t.Fatalf("revealed answer was not recorded in progress: %#v", ownerProgress)
	}
	if status := ebookCourseString(ebookCourseUnit(owner.call(200, "GET", "/ebook", nil), unitID), "status"); status != "learning" {
		t.Fatalf("revealed help did not move the unit into learning state: %q", status)
	}
	otherProgress := ebookCourseObject(other.call(200, "GET", "/ebook/units/"+unitID, nil)["progress"])
	if len(ebookCourseObject(otherProgress["revealed"])) != 0 {
		t.Fatalf("reveal state crossed learner boundary: %#v", otherProgress)
	}
}

func TestEbookCourseMigratesLegacyProgressWithoutChangingLegacyRow(t *testing.T) {
	a, owner, _, _ := ebookFixture(t)
	a.Book.Units[0].ID = "1"
	var userID string
	if err := a.DB.QueryRow(context.Background(), "SELECT id::text FROM users WHERE username='ebook-owner'").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	legacy := map[string]any{
		"page": 1, "answers": map[string]string{"legacy-question": "kept answer"},
		"exercises_completed": true, "listening_completed": true, "speaking_completed": true,
	}
	if _, err := a.DB.Exec(context.Background(), "INSERT INTO ebook_progress(user_id,unit_id,version,state) VALUES($1,'1',$2,$3)", userID, a.Book.Version, asJSON(legacy)); err != nil {
		t.Fatal(err)
	}

	unitID := "ebook-001"
	detail := owner.call(200, "GET", "/ebook/units/"+unitID, nil)
	progress := ebookCourseObject(detail["progress"])
	completed := ebookCourseObject(progress["completed_steps"])
	for _, step := range ebookCourseSteps {
		if completed[step] != true {
			t.Fatalf("legacy completion did not map %s into redesigned progress: %#v", step, progress)
		}
	}
	if progress["learned"] != true || ebookCourseObject(progress["answers"])["legacy-question"] != "kept answer" {
		t.Fatalf("legacy learned state or draft was not preserved: %#v", progress)
	}
	catalogProgress := ebookCourseObject(ebookCourseObject(owner.call(200, "GET", "/ebook", nil)["progress"])[unitID])
	if catalogProgress["learned"] != true || ebookCourseObject(catalogProgress["answers"])["legacy-question"] != "kept answer" {
		t.Fatalf("catalog did not expose migrated progress under redesigned lesson id: %#v", catalogProgress)
	}

	owner.call(200, "PATCH", "/ebook/units/"+unitID+"/progress", map[string]any{"current_step": 5})
	var migratedRaw, legacyRaw []byte
	if err := a.DB.QueryRow(context.Background(), "SELECT state FROM ebook_progress WHERE user_id=$1 AND unit_id='1' AND version=$2", userID, ebook.LearnEbookCourseVersion).Scan(&migratedRaw); err != nil {
		t.Fatal(err)
	}
	if err := a.DB.QueryRow(context.Background(), "SELECT state FROM ebook_progress WHERE user_id=$1 AND unit_id='1' AND version=$2", userID, a.Book.Version).Scan(&legacyRaw); err != nil {
		t.Fatal(err)
	}
	migrated := ebookCourseState(migratedRaw)
	if migrated["learned"] != true || ebookCourseObject(migrated["answers"])["legacy-question"] != "kept answer" {
		t.Fatalf("new version dropped legacy state during additive migration: %#v", migrated)
	}
	legacyAfter := ebookCourseState(legacyRaw)
	if _, changed := legacyAfter["completed_steps"]; changed {
		t.Fatalf("legacy row was rewritten instead of preserved: %#v", legacyAfter)
	}
}

func TestEbookCourseProgressCannotSpoofAssessedSteps(t *testing.T) {
	_, owner, _, _ := ebookFixture(t)
	unitID := ebookCourseString(ebookCourseObject(ebookCourseArray(owner.call(200, "GET", "/ebook", nil), "units", "lessons")[0]), "id", "unit_id")
	owner.call(200, "PATCH", "/ebook/units/"+unitID+"/progress", map[string]any{
		"current_step": 5,
		"completed_steps": map[string]bool{
			"understand": true, "examples": true, "quiz": true, "shadowing": true, "speaking": true,
		},
		"quiz_scores":  map[string]bool{"quiz-01": true, "quiz-02": true, "quiz-03": true, "quiz-04": true, "quiz-05": true},
		"learned":      true,
		"completed_at": "2099-01-01T00:00:00Z",
	})
	progress := ebookCourseObject(owner.call(200, "GET", "/ebook/units/"+unitID, nil)["progress"])
	completed := ebookCourseObject(progress["completed_steps"])
	if completed["understand"] != true || completed["examples"] != true {
		t.Fatalf("self-paced steps were not saved: %#v", progress)
	}
	for _, step := range []string{"quiz", "shadowing", "speaking"} {
		if completed[step] == true {
			t.Fatalf("client spoofed server-assessed step %q: %#v", step, progress)
		}
	}
	if progress["learned"] == true || ebookCourseString(progress, "completed_at") != "" || len(ebookCourseObject(progress["quiz_scores"])) != 0 {
		t.Fatalf("client spoofed learned state, completion time, or quiz scores: %#v", progress)
	}
}

func TestEbookShadowingUsesLessonLinesAndAudioOnlyMastery(t *testing.T) {
	a, owner, _, _ := ebookFixture(t)
	clearAudio, goalMet := false, false
	fake := featureAI(a, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "tts") {
			audioResponse(w)
			return
		}
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
	shadow := owner.call(201, "POST", "/ebook/units/"+unitID+"/sessions", map[string]any{"mode": "shadowing", "request_id": uuid.NewString()})
	sessionID := ebookCourseString(shadow, "id", "session_id")
	if sessionID == "" {
		t.Fatal("shadowing session did not return an id")
	}
	legacy := owner.call(200, "POST", "/ebook/units/"+unitID+"/sessions", map[string]any{"mode": "listening", "request_id": uuid.NewString()})
	if ebookCourseString(legacy, "id", "session_id") != sessionID {
		t.Fatalf("listening compatibility alias did not resume shadowing session: shadow=%#v alias=%#v", shadow, legacy)
	}

	listened := owner.call(200, "POST", "/sessions/"+sessionID+"/listen", map[string]any{"request_id": uuid.NewString()})
	if ebookCourseString(listened, "target", "sentence", "text", "caption") == "" || ebookCourseString(listened, "thai", "meaning", "translation") == "" {
		t.Fatalf("shadowing listen response hid the lesson target: %#v", listened)
	}
	before := ebookCourseShadowIndex(owner.call(200, "GET", "/sessions/"+sessionID, nil))
	owner.call(200, "POST", "/sessions/"+sessionID+"/turns", map[string]any{"request_id": uuid.NewString(), "text": "typed accessibility answer"})
	afterTyped := ebookCourseShadowIndex(owner.call(200, "GET", "/sessions/"+sessionID, nil))
	if afterTyped != before {
		t.Fatalf("typed input advanced shadowing mastery: before=%d after=%d", before, afterTyped)
	}

	unclear := owner.audioTurn(sessionID, uuid.NewString())
	if unclear["session_completed"] == true || ebookCourseShadowIndex(owner.call(200, "GET", "/sessions/"+sessionID, nil)) != before {
		t.Fatalf("unclear audio advanced shadowing: %#v", unclear)
	}
	clearAudio, goalMet = true, true
	requestID := uuid.NewString()
	passed := owner.audioTurn(sessionID, requestID)
	advanced := ebookCourseShadowIndex(owner.call(200, "GET", "/sessions/"+sessionID, nil))
	if advanced != before+1 {
		t.Fatalf("clear goal-matching audio did not advance exactly one line: response=%#v index=%d", passed, advanced)
	}
	owner.audioTurn(sessionID, requestID)
	if replayed := ebookCourseShadowIndex(owner.call(200, "GET", "/sessions/"+sessionID, nil)); replayed != advanced {
		t.Fatalf("shadowing audio replay advanced twice: %d -> %d", advanced, replayed)
	}
	for i := advanced; i < 3; i++ {
		owner.audioTurn(sessionID, uuid.NewString())
	}
	final := owner.call(200, "GET", "/ebook/units/"+unitID, nil)
	completed := ebookCourseObject(ebookCourseObject(final["progress"])["completed_steps"])
	if completed["shadowing"] != true {
		t.Fatalf("three clear shadowing lines did not complete the step: %#v", final)
	}
}

func ebookCourseArray(value any, keys ...string) []any {
	object := ebookCourseObject(value)
	for _, key := range keys {
		if values, ok := object[key].([]any); ok {
			return values
		}
	}
	return nil
}

func ebookCourseObject(value any) map[string]any {
	object, _ := value.(map[string]any)
	return object
}

func ebookCourseString(value any, keys ...string) string {
	object := ebookCourseObject(value)
	for _, key := range keys {
		if text, ok := object[key].(string); ok && strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

func ebookCourseNumber(value any, key string) int {
	object := ebookCourseObject(value)
	switch number := object[key].(type) {
	case float64:
		return int(number)
	case int:
		return number
	}
	return 0
}

func ebookCourseUnit(catalog map[string]any, id string) map[string]any {
	for _, raw := range ebookCourseArray(catalog, "units", "lessons") {
		unit := ebookCourseObject(raw)
		if ebookCourseString(unit, "id", "unit_id") == id {
			return unit
		}
	}
	return nil
}

func ebookCourseShadowIndex(session map[string]any) int {
	if value := ebookCourseNumber(session, "shadow_index"); value > 0 {
		return value
	}
	state := ebookCourseObject(session["state"])
	if len(state) == 0 {
		state = ebookCourseObject(ebookCourseObject(session["session"])["state"])
	}
	return ebookCourseNumber(state, "shadow_index")
}
