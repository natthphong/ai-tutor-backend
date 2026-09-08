package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"tokoloop/internal/ebook"
)

func ebookFixture(t *testing.T) (*App, featureHTTP, featureHTTP, ebook.Pack) {
	t.Helper()
	a := featureApp(t)
	dir := t.TempDir()
	a.Cfg.EbookDir = dir
	if err := os.WriteFile(filepath.Join(dir, "page-001.jpg"), []byte("fixture jpeg"), 0600); err != nil {
		t.Fatal(err)
	}
	a.Book = &ebook.Book{
		ID: "private-fixture", Title: "Private fixture", Version: "fixture-v1", PageCount: 392,
		Units: []ebook.Unit{{
			ID: "unit-001", Number: 1, Title: "Fixture grammar", LessonPage: 1, ExercisePage: 2,
			AnswerPages: []int{350}, Sections: []ebook.Section{{ID: "1.1", ItemIDs: []string{"1.1:1", "1.1:2"}}},
		}},
	}
	pack := ebook.Pack{
		ExplanationTH: "คำอธิบายจาก fixture", Pattern: "I use [grammar].", SpeakingPrompt: "Tell a short story.", SpeakingTH: "เล่าเรื่องสั้น ๆ", ListeningPrompt: "Listen and answer.", ListeningTH: "ฟังแล้วตอบ",
		Vocabulary: []ebook.Word{{Term: "fixture", Meaning: "ตัวอย่าง", Example: "A fixture example."}},
		Questions: []ebook.Question{
			{ID: "1.1:1", Kind: "choice", Prompt: "Choose the fixture answer.", InstructionTH: "เลือกคำตอบ", Options: []ebook.Option{{ID: "a", Text: "correct fixture answer"}, {ID: "b", Text: "incorrect fixture answer"}}, Answers: []string{"a"}, ExplanationTH: "เฉลย fixture"},
			{ID: "1.1:2", Kind: "choice", Prompt: "Choose the second fixture answer.", InstructionTH: "เลือกคำตอบ", Options: []ebook.Option{{ID: "a", Text: "first"}, {ID: "b", Text: "second fixture answer"}}, Answers: []string{"b"}, ExplanationTH: "เฉลยข้อสอง"},
		},
	}
	if err := pack.Validate(a.Book.Units[0]); err != nil {
		t.Fatalf("invalid test pack: %v", err)
	}
	if _, err := a.DB.Exec(context.Background(), "DELETE FROM ebook_packs WHERE unit_id=$1 AND version=$2", "unit-001", a.Book.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(context.Background(), "INSERT INTO ebook_packs(unit_id,version,status,data) VALUES($1,$2,'ready',$3)", "unit-001", a.Book.Version, asJSON(pack)); err != nil {
		t.Fatal(err)
	}
	return a, featureLogin(t, a, "ebook-owner"), featureLogin(t, a, "ebook-other"), pack
}

func ebookRaw(t *testing.T, a *App, token, method, path string, body any) (int, []byte, string) {
	t.Helper()
	var data []byte
	if body != nil {
		data = asJSON(body)
	}
	req := httptest.NewRequest(method, "http://localhost/ai-tutor/api/v2"+path, bytes.NewReader(data))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := a.HTTP.Test(req, 60000)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, raw, res.Header.Get("Content-Type")
}

func TestEbookAPIProtectsPrivatePagesAndWithholdsAnswersUntilReveal(t *testing.T) {
	a, owner, _, _ := ebookFixture(t)
	if status, _, _ := ebookRaw(t, a, "", "GET", "/ebook/pages/1", nil); status != 401 {
		t.Fatalf("anonymous page status = %d", status)
	}
	if status, _, _ := ebookRaw(t, a, "", "GET", "/ebook", nil); status != 401 {
		t.Fatalf("anonymous catalog status = %d", status)
	}
	status, page, contentType := ebookRaw(t, a, owner.token, "GET", "/ebook/pages/1", nil)
	if status != 200 || !strings.HasPrefix(contentType, "image/jpeg") || string(page) != "fixture jpeg" {
		t.Fatalf("private fixture page = %d %q %q", status, contentType, page)
	}
	if status, _, _ = ebookRaw(t, a, owner.token, "GET", "/ebook/pages/393", nil); status != 404 {
		t.Fatalf("out-of-range page status = %d", status)
	}

	status, raw, _ := ebookRaw(t, a, owner.token, "GET", "/ebook/units/unit-001", nil)
	if status != 200 {
		t.Fatalf("unit status = %d: %s", status, raw)
	}
	if strings.Contains(string(raw), "\"answers\"") || strings.Contains(string(raw), "เฉลย fixture") {
		t.Fatalf("unit response leaked hidden answer: %s", raw)
	}
	if !strings.Contains(string(raw), "Choose the fixture answer.") {
		t.Fatalf("unit response lost original question: %s", raw)
	}

	reveal := owner.call(200, "POST", "/ebook/units/unit-001/reveal", map[string]any{"ids": []string{"1.1:1"}})
	answer, ok := reveal["1.1:1"].(map[string]any)
	if !ok || textValue(answer["explanation_th"]) != "เฉลย fixture" {
		t.Fatalf("explicit reveal response = %#v", reveal)
	}
}

func TestEbookAPIProgressChecksAndOralSessionsAreIsolatedAndReplaySafe(t *testing.T) {
	a, owner, other, _ := ebookFixture(t)
	owner.call(200, "PATCH", "/ebook/units/unit-001/progress", map[string]any{"page": 2, "answers": map[string]string{"1.1:1": "b"}})
	otherDetail := other.call(200, "GET", "/ebook/units/unit-001", nil)
	otherProgress, ok := otherDetail["progress"].(map[string]any)
	if !ok || len(otherProgress) != 0 {
		t.Fatalf("other learner received progress: %#v", otherDetail)
	}
	ownerDetail := owner.call(200, "GET", "/ebook/units/unit-001", nil)
	ownerProgress := ownerDetail["progress"].(map[string]any)
	if number(ownerProgress["page"], 0) != 2 || textValue(ownerProgress["answers"].(map[string]any)["1.1:1"]) != "b" {
		t.Fatalf("saved owner draft missing: %#v", ownerProgress)
	}

	incorrectID := uuid.NewString()
	incorrect := owner.call(200, "POST", "/ebook/units/unit-001/check", map[string]any{"request_id": incorrectID, "answers": map[string]string{"1.1:1": "b"}})
	marks := incorrect["marks"].([]any)
	if len(marks) != 1 || marks[0].(map[string]any)["correct"] != false {
		t.Fatalf("incorrect mark = %#v", incorrect)
	}
	replay := owner.call(200, "POST", "/ebook/units/unit-001/check", map[string]any{"request_id": incorrectID, "answers": map[string]string{"1.1:1": "a"}})
	if string(asJSON(replay)) != string(asJSON(incorrect)) {
		t.Fatalf("check replay changed persisted result: first=%#v replay=%#v", incorrect, replay)
	}
	var reviews int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM review_items WHERE key=$1", "ebook:fixture-v1:unit-001:1.1:1").Scan(&reviews); err != nil || reviews != 1 {
		t.Fatalf("incorrect answer review = %d, %v", reviews, err)
	}

	correct := owner.call(200, "POST", "/ebook/units/unit-001/check", map[string]any{"request_id": uuid.NewString(), "answers": map[string]string{"1.1:2": "b"}})
	if correct["marks"].([]any)[0].(map[string]any)["correct"] != true {
		t.Fatalf("original answer was not accepted: %#v", correct)
	}

	requestID := uuid.NewString()
	started := owner.call(201, "POST", "/ebook/units/unit-001/sessions", map[string]any{"mode": "speak", "request_id": requestID})
	replayed := owner.call(200, "POST", "/ebook/units/unit-001/sessions", map[string]any{"mode": "speak", "request_id": requestID})
	if started["id"] != replayed["id"] {
		t.Fatalf("oral session replay changed id: %#v %#v", started, replayed)
	}
	var state []byte
	if err := a.DB.QueryRow(context.Background(), "SELECT state FROM learning_sessions WHERE id=$1", textValue(started["id"])).Scan(&state); err != nil {
		t.Fatal(err)
	}
	got := map[string]any{}
	if err := json.Unmarshal(state, &got); err != nil {
		t.Fatal(err)
	}
	if got["ebook_unit_id"] != "unit-001" || got["ebook_skill"] != "speak" || number(got["independent"], -1) != 0 {
		t.Fatalf("oral session context = %#v", got)
	}
	other.call(404, "POST", "/ebook/units/missing/sessions", map[string]any{"mode": "speak", "request_id": uuid.NewString()})
}

func TestEbookOralNeedsTwoIndependentRoundsBeforeItMarksTheSkillComplete(t *testing.T) {
	a, owner, _, _ := ebookFixture(t)
	fake := featureAI(a, func(w http.ResponseWriter, r *http.Request) { response(w, string(feedbackJSON())) })
	defer fake.Close()

	manual := owner.call(201, "POST", "/ebook/units/unit-001/sessions", map[string]any{"mode": "speak", "request_id": uuid.NewString()})
	owner.call(200, "POST", "/sessions/"+textValue(manual["id"])+"/complete", map[string]any{})
	var state []byte
	if err := a.DB.QueryRow(context.Background(), "SELECT state FROM ebook_progress WHERE user_id=(SELECT user_id FROM learning_sessions WHERE id=$1) AND unit_id='unit-001' AND version='fixture-v1'", textValue(manual["id"])).Scan(&state); err != nil && err != pgx.ErrNoRows {
		t.Fatal(err)
	}
	if len(state) > 0 && decodeFeature(t, state)["speaking_completed"] == true {
		t.Fatalf("manual incomplete session marked speaking complete: %s", state)
	}

	speak := textValue(owner.call(201, "POST", "/ebook/units/unit-001/sessions", map[string]any{"mode": "speak", "request_id": uuid.NewString()})["id"])
	first := owner.audioTurn(speak, uuid.NewString())
	if first["session_completed"] == true || number(first["progress"].(map[string]any)["percent"], 0) != 50 {
		t.Fatalf("first ebook speaking round = %#v", first)
	}
	second := owner.audioTurn(speak, uuid.NewString())
	if second["session_completed"] != true || number(second["progress"].(map[string]any)["percent"], 0) != 100 {
		t.Fatalf("second ebook speaking round = %#v", second)
	}
	if err := a.DB.QueryRow(context.Background(), "SELECT state FROM ebook_progress WHERE user_id=(SELECT user_id FROM learning_sessions WHERE id=$1) AND unit_id='unit-001' AND version='fixture-v1'", speak).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if decodeFeature(t, state)["speaking_completed"] != true {
		t.Fatalf("two independent speaking rounds did not persist completion: %s", state)
	}

	listen := textValue(owner.call(201, "POST", "/ebook/units/unit-001/sessions", map[string]any{"mode": "listening", "request_id": uuid.NewString()})["id"])
	if one := owner.audioTurn(listen, uuid.NewString()); one["session_completed"] == true {
		t.Fatalf("first ebook listening round completed early: %#v", one)
	}
	if two := owner.audioTurn(listen, uuid.NewString()); two["session_completed"] != true {
		t.Fatalf("second ebook listening round did not complete: %#v", two)
	}
	if err := a.DB.QueryRow(context.Background(), "SELECT state FROM ebook_progress WHERE user_id=(SELECT user_id FROM learning_sessions WHERE id=$1) AND unit_id='unit-001' AND version='fixture-v1'", listen).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if decodeFeature(t, state)["listening_completed"] != true {
		t.Fatalf("two listening rounds did not persist completion: %s", state)
	}
}

func ebookPackBuildFixture(t *testing.T) (*App, string, ebook.Unit, ebook.Pack) {
	t.Helper()
	a := featureApp(t)
	dir := t.TempDir()
	a.Cfg.EbookDir = dir
	unit := ebook.Unit{ID: "repair-unit", Number: 1, Title: "Repair fixture", LessonPage: 1, ExercisePage: 1, Sections: []ebook.Section{{ID: "1.1", ItemIDs: []string{"1.1:1", "1.1:2"}}}}
	a.Book = &ebook.Book{ID: "repair-book", Title: "Repair fixture", Version: "repair-v1", PageCount: 1, Units: []ebook.Unit{unit}}
	if err := os.WriteFile(filepath.Join(dir, "page-001.jpg"), []byte("fixture image"), 0600); err != nil {
		t.Fatal(err)
	}
	full := ebook.Pack{
		ExplanationTH: "คำอธิบาย", Pattern: "I use [grammar].", SpeakingPrompt: "Speak.", SpeakingTH: "พูด", ListeningPrompt: "Listen.", ListeningTH: "ฟัง",
		Vocabulary: []ebook.Word{{Term: "fixture", Meaning: "ตัวอย่าง", Example: "A fixture."}},
		Questions: []ebook.Question{
			{ID: "1.1:1", Kind: "choice", Prompt: "First original item", InstructionTH: "เลือก", Options: []ebook.Option{{ID: "a", Text: "one"}, {ID: "b", Text: "two"}}, Answers: []string{"a"}},
			{ID: "1.1:2", Kind: "choice", Prompt: "Worked original item", InstructionTH: "เลือก", Options: []ebook.Option{{ID: "a", Text: "one"}, {ID: "b", Text: "two"}}, Answers: []string{"b"}, Example: true},
		},
	}
	if err := full.Validate(unit); err != nil {
		t.Fatal(err)
	}
	featureLogin(t, a, "ebook-repair")
	var uid string
	if err := a.DB.QueryRow(context.Background(), "SELECT id::text FROM users WHERE username='ebook-repair'").Scan(&uid); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(context.Background(), "DELETE FROM ebook_packs WHERE unit_id=$1 AND version=$2", unit.ID, a.Book.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(context.Background(), "INSERT INTO ebook_packs(unit_id,version,status) VALUES($1,$2,'queued')", unit.ID, a.Book.Version); err != nil {
		t.Fatal(err)
	}
	return a, uid, unit, full
}

func TestEbookPackRepairsSkippedWorkedExampleInOriginalOrder(t *testing.T) {
	a, uid, unit, full := ebookPackBuildFixture(t)
	initial := full
	initial.Questions = initial.Questions[:1]
	calls := 0
	fake := featureAI(a, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			response(w, string(asJSON(initial)))
			return
		}
		response(w, string(asJSON(map[string]ebook.Question{unit.Sections[0].ItemIDs[1]: full.Questions[1]})))
	})
	defer fake.Close()
	result, err := a.makeEbookPack(context.Background(), uid, map[string]any{"unit_id": unit.ID, "version": a.Book.Version})
	if err != nil {
		t.Fatalf("repair failed: %v", err)
	}
	if calls != 2 {
		t.Fatalf("provider calls = %d, want initial + one repair", calls)
	}
	public := result.(ebook.Pack)
	if len(public.Questions) != 2 || public.Questions[0].ID != "1.1:1" || public.Questions[1].ID != "1.1:2" || len(public.Questions[0].Answers) != 0 || len(public.Questions[1].Answers) != 1 {
		t.Fatalf("repaired public pack = %#v", public.Questions)
	}
	var status string
	var saved []byte
	if err := a.DB.QueryRow(context.Background(), "SELECT status,data FROM ebook_packs WHERE unit_id=$1 AND version=$2", unit.ID, a.Book.Version).Scan(&status, &saved); err != nil {
		t.Fatal(err)
	}
	var stored ebook.Pack
	if err := json.Unmarshal(saved, &stored); err != nil {
		t.Fatal(err)
	}
	if status != "ready" || stored.Validate(unit) != nil {
		t.Fatalf("repaired stored pack status=%q data=%s", status, saved)
	}
}

func TestEbookPackFailsWhenRepairStillOmitsARequiredItem(t *testing.T) {
	a, uid, unit, full := ebookPackBuildFixture(t)
	initial := full
	initial.Questions = initial.Questions[:1]
	calls := 0
	fake := featureAI(a, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			response(w, string(asJSON(initial)))
			return
		}
		response(w, "{}")
	})
	defer fake.Close()
	if _, err := a.makeEbookPack(context.Background(), uid, map[string]any{"unit_id": unit.ID, "version": a.Book.Version}); err == nil {
		t.Fatal("missing repair item was accepted")
	}
	var status string
	if err := a.DB.QueryRow(context.Background(), "SELECT status FROM ebook_packs WHERE unit_id=$1 AND version=$2", unit.ID, a.Book.Version).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Fatalf("incomplete repair status = %q", status)
	}
}
