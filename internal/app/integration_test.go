package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"tokoloop/internal/config"
	"tokoloop/internal/gemini"
	"tokoloop/internal/learning"
)

func TestIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a dedicated toko_*_test database")
	}
	if !strings.Contains(dsn, "_test") {
		t.Fatal("refusing non-test DB")
	}
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("GEMINI_API_KEY", "test-key")
	cfg, e := config.Load()
	if e != nil {
		t.Fatal(e)
	}
	cfg.AudioDir = t.TempDir()
	a, e := New(context.Background(), cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer a.DB.Close()
	_, e = a.DB.Exec(context.Background(), "TRUNCATE users CASCADE")
	if e != nil {
		t.Fatal(e)
	}
	if e = a.seed(context.Background()); e != nil {
		t.Fatal(e)
	}
	var calls atomic.Int32
	var failMode atomic.Bool
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if failMode.Load() {
			w.WriteHeader(429)
			w.Write([]byte(`{"error":{"code":429}}`))
			return
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		contents := body["contents"].([]any)[0].(map[string]any)["parts"].([]any)
		isAudio := len(contents) > 1
		f := learning.Feedback{Transcript: "Hello, I'm Pim.", Reply: "Nice to meet you. Where do you work?", Meaning: "แนะนำตัวได้ชัดเจน", Correct: true, GoalMet: true, AudioClear: isAudio, Corrections: []learning.Correction{}, Weaknesses: []string{}, Vocabulary: []string{}, Level: "A1"}
		b, _ := json.Marshal(f)
		json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"finishReason": "STOP", "content": map[string]any{"parts": []any{map[string]any{"text": string(b)}}}}}, "usageMetadata": map[string]any{"promptTokenCount": 200, "candidatesTokenCount": 100}})
	}))
	defer fake.Close()
	a.AI = &gemini.Client{Key: "test-key", HTTP: fake.Client(), BaseURL: fake.URL + "/"}
	req := func(method, path, token string, p any) (int, []byte) {
		t.Helper()
		var b []byte
		if p != nil {
			b = asJSON(p)
		}
		r := httptest.NewRequest(method, "http://localhost/ai-tutor/api/v2"+path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		res, e := a.HTTP.Test(r, 60000)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		out, _ := io.ReadAll(res.Body)
		return res.StatusCode, out
	}
	decode := func(b []byte) map[string]any {
		t.Helper()
		var v map[string]any
		if e := json.Unmarshal(b, &v); e != nil {
			t.Fatalf("decode %s: %v", b, e)
		}
		return v
	}
	expect := func(want int, method, path, token string, p any) []byte {
		t.Helper()
		code, b := req(method, path, token, p)
		if code != want {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, code, want, b)
		}
		return b
	}
	admin := textValue(decode(expect(200, "POST", "/auth/login", "", map[string]any{"username": "admin", "password": "password"}))["token"])
	expect(403, "POST", "/admin/invitations", admin, map[string]any{})
	expect(200, "POST", "/auth/change-password", admin, map[string]any{"current": "password", "password": "Local-only-test-password"})
	if e = a.seed(context.Background()); e != nil {
		t.Fatal(e)
	}
	expect(401, "POST", "/auth/login", "", map[string]any{"username": "admin", "password": "password"})
	invite := textValue(decode(expect(201, "POST", "/admin/invitations", admin, map[string]any{}))["code"])
	expect(201, "POST", "/auth/register", "", map[string]any{"username": "learner", "password": "another-test-password", "invitation": invite})
	expect(400, "POST", "/auth/register", "", map[string]any{"username": "other", "password": "another-test-password", "invitation": invite})
	token := textValue(decode(expect(200, "POST", "/auth/login", "", map[string]any{"username": "learner", "password": "another-test-password"}))["token"])
	expect(403, "POST", "/admin/invitations", token, map[string]any{})
	me := decode(expect(200, "GET", "/auth/me", token, nil))
	uid := textValue(me["id"])
	expect(200, "PATCH", "/profile", token, map[string]any{"level": "B2", "role": "admin", "monthly_budget": 500})
	me = decode(expect(200, "GET", "/auth/me", token, nil))
	if me["profile"].(map[string]any)["level"] != "Pre-A1" || me["role"] != "learner" {
		t.Fatal("profile privilege escalation")
	}
	expect(400, "PATCH", "/profile", token, map[string]any{"monthly_budget": 1001})
	for _, path := range []string{"/curriculum", "/daily-plan", "/progress", "/library", "/scenarios", "/review", "/usage", "/sessions"} {
		expect(200, "GET", path, token, nil)
	}
	var lessons []any
	json.Unmarshal(expect(200, "GET", "/curriculum", token, nil), &lessons)
	if len(lessons) != 525 {
		t.Fatal("curriculum incomplete")
	}
	s := decode(expect(201, "POST", "/sessions", token, map[string]any{"mode": "lesson", "lesson_id": "lesson-001"}))
	sid := textValue(s["id"])
	expect(404, "GET", "/sessions/"+sid, admin, nil)
	expect(409, "POST", "/sessions/"+sid+"/turns", token, map[string]any{"text": "Hello", "request_id": uuid.NewString()})
	expect(200, "POST", "/sessions/"+sid+"/advance", token, map[string]any{})
	expect(409, "POST", "/sessions/"+sid+"/advance", token, map[string]any{})
	for i := 1; i <= 4; i++ {
		h := decode(expect(200, "POST", "/sessions/"+sid+"/hints", token, map[string]any{}))
		if int(number(h["level"], 0)) != i {
			t.Fatal("wrong hint level")
		}
	}
	rid := uuid.NewString()
	p := map[string]any{"text": "Hi, my name is Pim.", "request_id": rid}
	first := decode(expect(200, "POST", "/sessions/"+sid+"/turns", token, p))
	before := calls.Load()
	second := decode(expect(200, "POST", "/sessions/"+sid+"/turns", token, p))
	if first["id"] != second["id"] || before != calls.Load() {
		t.Fatal("idempotency billed twice")
	}
	var count int
	a.DB.QueryRow(context.Background(), "SELECT independent_successes FROM mastery WHERE user_id=$1 AND lesson_id='lesson-001'", uid).Scan(&count)
	if count != 0 {
		t.Fatal("typed response counted as speaking")
	}
	expect(200, "POST", "/sessions/"+sid+"/advance", token, map[string]any{})
	failMode.Store(true)
	code, _ := req("POST", "/sessions/"+sid+"/turns", token, map[string]any{"text": "Hello", "request_id": uuid.NewString()})
	if code != 502 {
		t.Fatal("provider failure was accepted", code)
	}
	a.DB.QueryRow(context.Background(), "SELECT count(*) FROM attempts WHERE session_id=$1", sid).Scan(&count)
	if count != 1 {
		t.Fatal("failed provider created attempt")
	}
	failMode.Store(false)
	expect(200, "PATCH", "/profile", token, map[string]any{"monthly_budget": 1})
	expect(402, "POST", "/sessions/"+sid+"/turns", token, map[string]any{"text": "Hello", "request_id": uuid.NewString()})
	expect(200, "PATCH", "/profile", token, map[string]any{"monthly_budget": 500})
	if _, e := exec.LookPath("ffmpeg"); e != nil {
		t.Fatal("ffmpeg required for audio integration")
	}
	// Isolate transfer mastery on two distinct conversation tasks, with a genuine decoded WAV upload.
	_, e = a.DB.Exec(context.Background(), `UPDATE learning_sessions SET state='{"stage":"conversation","step":0,"hint_level":0,"last_pass":false,"independent":0}'::jsonb WHERE id=$1`, sid)
	if e != nil {
		t.Fatal(e)
	}
	audioReq := func(requestID string) (int, map[string]any) {
		t.Helper()
		var buf bytes.Buffer
		m := multipart.NewWriter(&buf)
		m.WriteField("request_id", requestID)
		f, _ := m.CreateFormFile("audio", "voice.wav")
		f.Write(gemini.WAV(make([]byte, 32000), 16000))
		m.Close()
		r := httptest.NewRequest("POST", "http://localhost/ai-tutor/api/v2/sessions/"+sid+"/turns", &buf)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", m.FormDataContentType())
		res, e := a.HTTP.Test(r, 60000)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, decode(b)
	}
	code, answer := audioReq(uuid.NewString())
	if code != 200 {
		t.Fatal(answer)
	}
	aid := textValue(answer["audio_id"])
	expect(200, "GET", "/audio/"+aid, token, nil)
	expect(404, "GET", "/audio/"+aid, admin, nil)
	audioReq(uuid.NewString())
	var state []byte
	a.DB.QueryRow(context.Background(), "SELECT state FROM learning_sessions WHERE id=$1", sid).Scan(&state)
	if number(decode(state)["independent"], 0) != 1 {
		t.Fatal("same conversation step counted twice")
	}
	expect(200, "POST", "/sessions/"+sid+"/advance", token, map[string]any{})
	code, answer = audioReq(uuid.NewString())
	if code != 200 {
		t.Fatal(answer)
	}
	summary := decode(expect(200, "POST", "/sessions/"+sid+"/complete", token, map[string]any{}))
	if summary["mastered"] != true {
		t.Fatal("transfer was not mastered", summary)
	}
	expect(409, "POST", "/sessions/"+sid+"/turns", token, map[string]any{"text": "late", "request_id": uuid.NewString()})
	// Retention removes both metadata and file without deleting attempts.
	_, e = a.DB.Exec(context.Background(), "UPDATE audio_assets SET expires_at=now()-interval '1 second' WHERE id=$1", aid)
	if e != nil {
		t.Fatal(e)
	}
	a.cleanup(context.Background())
	expect(404, "GET", "/audio/"+aid, token, nil)
	expect(201, "POST", "/vocabulary", token, map[string]any{"term": "clarify", "meaning": "อธิบายให้ชัด", "example": "Could you clarify the deadline?"})
	var reviews []map[string]any
	json.Unmarshal(expect(200, "GET", "/review", token, nil), &reviews)
	if len(reviews) != 1 {
		t.Fatal("word missing from reviews")
	}
	reviewID := textValue(reviews[0]["id"])
	expect(404, "POST", "/review/"+reviewID+"/answer", admin, map[string]any{"text": "clarify", "request_id": uuid.NewString()})
	review := decode(expect(200, "POST", "/review/"+reviewID+"/answer", token, map[string]any{"text": "clarify", "request_id": uuid.NewString()}))
	if number(review["stage"], -1) != 0 {
		t.Fatal("typed review advanced speaking schedule")
	}
	expect(200, "POST", "/auth/logout", token, map[string]any{})
	expect(401, "GET", "/auth/me", token, nil)
	fmt.Println("integration verified: bootstrap, invitations, ownership, curriculum, hints, idempotency, provider failure, budgets, codec, transfer mastery, retention, review, logout")
}
