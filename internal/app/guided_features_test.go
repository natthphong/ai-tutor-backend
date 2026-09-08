package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"tokoloop/internal/gemini"
	"tokoloop/internal/learning"
	"tokoloop/internal/security"
)

type featureHTTP struct {
	t     *testing.T
	a     *App
	token string
}

func (h featureHTTP) call(want int, method, path string, body any) map[string]any {
	h.t.Helper()
	var data []byte
	if body != nil {
		data = asJSON(body)
	}
	req := httptest.NewRequest(method, "http://localhost/ai-tutor/api/v2"+path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}
	res, err := h.a.HTTP.Test(req, 60000)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != want {
		h.t.Fatalf("%s %s: got %d want %d: %s", method, path, res.StatusCode, want, raw)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		h.t.Fatalf("decode %s: %v", raw, err)
	}
	return out
}

func featureLogin(t *testing.T, a *App, username string) featureHTTP {
	t.Helper()
	if _, err := a.DB.Exec(context.Background(), "INSERT INTO users(id,username,password_hash) VALUES($1,$2,$3)", uuid.NewString(), username, security.Hash("fixture-password")); err != nil {
		t.Fatal(err)
	}
	h := featureHTTP{t: t, a: a}
	h.token = textValue(h.call(200, "POST", "/auth/login", map[string]any{"username": username, "password": "fixture-password"})["token"])
	return h
}

func (h featureHTTP) audioTurn(sessionID, requestID string) map[string]any {
	h.t.Helper()
	var data bytes.Buffer
	m := multipart.NewWriter(&data)
	_ = m.WriteField("request_id", requestID)
	f, err := m.CreateFormFile("audio", "fixture.wav")
	if err != nil {
		h.t.Fatal(err)
	}
	_, _ = f.Write(gemini.WAV(make([]byte, 32000), 16000))
	_ = m.Close()
	req := httptest.NewRequest("POST", "http://localhost/ai-tutor/api/v2/sessions/"+sessionID+"/turns", &data)
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Content-Type", m.FormDataContentType())
	res, err := h.a.HTTP.Test(req, 60000)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		h.t.Fatalf("audio turn: %d %s", res.StatusCode, raw)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		h.t.Fatal(err)
	}
	return out
}

func featureAI(a *App, handler http.HandlerFunc) *httptest.Server {
	s := httptest.NewServer(handler)
	a.AI = &gemini.Client{Key: "test-key", HTTP: s.Client(), BaseURL: s.URL + "/", Models: a.Cfg.Models, DefaultTimeout: a.Cfg.TimeoutSeconds}
	return s
}

func response(w http.ResponseWriter, text string) {
	_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"finishReason": "STOP", "content": map[string]any{"parts": []any{map[string]any{"text": text}}}}}, "usageMetadata": map[string]any{"promptTokenCount": 1, "candidatesTokenCount": 1}})
}

func audioResponse(w http.ResponseWriter) {
	_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"finishReason": "STOP", "content": map[string]any{"parts": []any{map[string]any{"inlineData": map[string]any{"mimeType": "audio/wav", "data": base64.StdEncoding.EncodeToString(gemini.WAV(make([]byte, 32000), 16000))}}}}}}, "usageMetadata": map[string]any{"promptTokenCount": 1, "candidatesTokenCount": 1}})
}

func TestGuidedLessonNeedsTwoIndependentRounds(t *testing.T) {
	a := featureApp(t)
	fake := featureAI(a, func(w http.ResponseWriter, r *http.Request) { response(w, string(feedbackJSON())) })
	defer fake.Close()
	h := featureLogin(t, a, "guided-learner")
	sid := textValue(h.call(201, "POST", "/sessions", map[string]any{"mode": "lesson", "lesson_id": "lesson-001"})["id"])
	var state []byte
	if err := a.DB.QueryRow(context.Background(), "SELECT state FROM learning_sessions WHERE id=$1", sid).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if textValue(decodeFeature(t, state)["lesson_flow"]) != "guided-v2" {
		t.Fatalf("new lesson did not use guided flow: %s", state)
	}
	advanced := h.call(200, "POST", "/sessions/"+sid+"/advance", map[string]any{})
	if advanced["stage"] != "conversation" {
		t.Fatalf("guided lesson did not skip drills: %#v", advanced)
	}
	first := h.audioTurn(sid, uuid.NewString())
	if first["session_completed"] == true || number(first["progress"].(map[string]any)["percent"], 0) != 50 {
		t.Fatalf("first guided round status: %#v", first)
	}
	h.call(200, "POST", "/sessions/"+sid+"/advance", map[string]any{})
	second := h.audioTurn(sid, uuid.NewString())
	if second["session_completed"] != true || number(second["progress"].(map[string]any)["percent"], 0) != 100 {
		t.Fatalf("two guided rounds did not complete: %#v", second)
	}
}

func TestHintReplayUnicodeAndFailureDoNotAdvance(t *testing.T) {
	a := featureApp(t)
	var calls int
	fail := false
	fake := featureAI(a, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if fail {
			http.Error(w, "helper unavailable", http.StatusBadGateway)
			return
		}
		response(w, "คำช่วยภาษาไทย")
	})
	defer fake.Close()
	h := featureLogin(t, a, "hint-learner")
	sid := textValue(h.call(201, "POST", "/sessions", map[string]any{"mode": "free"})["id"])
	id := uuid.NewString()
	first := h.call(200, "POST", "/sessions/"+sid+"/hints", map[string]any{"request_id": id, "idea": strings.Repeat("ก", 500)})
	if number(first["level"], 0) != 1 || calls != 1 {
		t.Fatalf("unicode hint was not accepted once: %#v calls=%d", first, calls)
	}
	replay := h.call(200, "POST", "/sessions/"+sid+"/hints", map[string]any{"request_id": id, "idea": strings.Repeat("ก", 500)})
	if replay["level"] != first["level"] || calls != 1 {
		t.Fatalf("hint replay called helper again: %#v calls=%d", replay, calls)
	}
	fail = true
	req := httptest.NewRequest("POST", "http://localhost/ai-tutor/api/v2/sessions/"+sid+"/hints", bytes.NewReader(asJSON(map[string]any{"request_id": uuid.NewString(), "idea": "another idea"})))
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := a.HTTP.Test(req, 60000)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 502 {
		t.Fatalf("failed hint = %d", res.StatusCode)
	}
	var state []byte
	if err := a.DB.QueryRow(context.Background(), "SELECT state FROM learning_sessions WHERE id=$1", sid).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if number(decodeFeature(t, state)["hint_level"], 0) != 1 {
		t.Fatalf("failed hint advanced level: %s", state)
	}
}

func TestListeningAndDailyMeetFeatureContracts(t *testing.T) {
	a := featureApp(t)
	feedback, _ := json.Marshal(learning.Feedback{Transcript: "I like music.", Reply: "What kind of music do you like?", ReplyTH: "คุณชอบเพลงแบบไหน", Meaning: "เข้าใจคำถาม", Correct: false, GoalMet: true, AudioClear: true, Corrections: []learning.Correction{}, Weaknesses: []string{}, Vocabulary: []string{}, Level: "A1"})
	daily, _ := json.Marshal(map[string]any{"title": "Stand-up", "english": "I reviewed the release.", "thai": "ฉันทบทวนรีลีส", "question": "What did you review?", "question_th": "คุณทบทวนอะไร", "phrases": []any{map[string]any{"en": "review a release", "th": "ทบทวนรีลีส", "note": "งาน"}}})
	fake := featureAI(a, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "tts") {
			audioResponse(w)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		text := string(feedback)
		if parts, ok := body["contents"].([]any); ok && len(parts) > 0 && strings.Contains(string(asJSON(parts)), "Notes:") {
			text = string(daily)
		}
		response(w, text)
	})
	defer fake.Close()
	h := featureLogin(t, a, "listen-owner")
	other := featureLogin(t, a, "meet-other")
	listenID := textValue(h.call(201, "POST", "/sessions", map[string]any{"mode": "listening"})["id"])
	listen := func(want int, requestID string) map[string]any {
		return h.call(want, "POST", "/sessions/"+listenID+"/listen", map[string]any{"request_id": requestID})
	}
	firstID := uuid.NewString()
	one := listen(200, firstID)
	if number(one["listen_count"], 0) != 1 || one["caption"] != "" {
		t.Fatalf("first listen wrong: %#v", one)
	}
	if replay := listen(200, firstID); replay["listen_count"] != one["listen_count"] {
		t.Fatalf("listen replay incremented: %#v", replay)
	}
	two := listen(200, uuid.NewString())
	if number(two["listen_count"], 0) != 2 || textValue(two["caption"]) == "" {
		t.Fatalf("second listen wrong: %#v", two)
	}
	three := listen(200, uuid.NewString())
	if number(three["listen_count"], 0) != 3 || textValue(three["translation"]) == "" {
		t.Fatalf("third listen wrong: %#v", three)
	}
	h.call(200, "POST", "/sessions/"+listenID+"/turns", map[string]any{"request_id": uuid.NewString(), "text": "I like music."})
	var listenState []byte
	if err := a.DB.QueryRow(context.Background(), "SELECT state FROM learning_sessions WHERE id=$1", listenID).Scan(&listenState); err != nil {
		t.Fatal(err)
	}
	state := decodeFeature(t, listenState)
	if number(state["listen_count"], -1) != 0 || state["listening_understood"] != true {
		t.Fatalf("learner reply did not reset/listen-goal state: %#v", state)
	}

	requestID := uuid.NewString()
	job := h.call(202, "POST", "/daily-meets", map[string]any{"day": "2026-09-06", "title": "Stand-up", "source": "ตรวจ release และจดงานต่อ", "request_id": requestID})
	if replay := h.call(202, "POST", "/daily-meets", map[string]any{"day": "2026-09-06", "title": "Stand-up", "source": "ตรวจ release และจดงานต่อ", "request_id": requestID}); replay["job_id"] != job["job_id"] {
		t.Fatalf("daily meet job replay changed id: %#v", replay)
	}
	a.runJob(context.Background())
	req := httptest.NewRequest("GET", "http://localhost/ai-tutor/api/v2/daily-meets", nil)
	req.Header.Set("Authorization", "Bearer "+h.token)
	res, err := a.HTTP.Test(req, 60000)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("daily meet list status %d: %s", res.StatusCode, raw)
	}
	var meets []DailyMeet
	if err := json.Unmarshal(raw, &meets); err != nil || len(meets) != 1 {
		t.Fatalf("daily meet list: %#v err=%v", meets, err)
	}
	var dailyID string
	if err := a.DB.QueryRow(context.Background(), "SELECT id::text FROM daily_meets WHERE user_id=(SELECT user_id FROM jobs WHERE id=$1)", textValue(job["job_id"])).Scan(&dailyID); err != nil {
		t.Fatal(err)
	}
	updated := h.call(200, "PATCH", "/daily-meets/"+dailyID, map[string]any{"title": "Updated", "english": "I reviewed the release.", "thai": "ฉันทบทวนรีลีส"})
	if updated["title"] != "Updated" {
		t.Fatalf("daily edit not retained: %#v", updated)
	}
	other.call(404, "PATCH", "/daily-meets/"+dailyID, map[string]any{"title": "No", "english": "No", "thai": "ไม่"})
	other.call(404, "POST", "/daily-meets/"+dailyID+"/sessions", map[string]any{"mode": "free", "request_id": uuid.NewString()})
	session := h.call(201, "POST", "/daily-meets/"+dailyID+"/sessions", map[string]any{"mode": "listening", "request_id": uuid.NewString()})
	var sessionState []byte
	if err := a.DB.QueryRow(context.Background(), "SELECT state FROM learning_sessions WHERE id=$1", textValue(session["id"])).Scan(&sessionState); err != nil {
		t.Fatal(err)
	}
	if decodeFeature(t, sessionState)["daily_meet_id"] != dailyID {
		t.Fatalf("daily session missing context: %s", sessionState)
	}
}

func decodeFeature(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
