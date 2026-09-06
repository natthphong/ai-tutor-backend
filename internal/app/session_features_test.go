package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"tokoloop/internal/config"
	"tokoloop/internal/gemini"
	"tokoloop/internal/learning"
	"tokoloop/internal/security"
)

func featureApp(t *testing.T) *App {
	t.Helper()
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
	cfg.MinIO.Endpoint = ""
	a, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.DB.Close() })
	if _, err = a.DB.Exec(context.Background(), "TRUNCATE users CASCADE"); err != nil {
		t.Fatal(err)
	}
	if err = a.seed(context.Background()); err != nil {
		t.Fatal(err)
	}
	return a
}

func feedbackJSON() []byte {
	b, _ := json.Marshal(learning.Feedback{Transcript: "I work in Bangkok.", Reply: "What do you do there?", ReplyTH: "คุณทำอะไรที่นั่น", Meaning: "สื่อสารได้", Correct: true, GoalMet: true, AudioClear: true, Corrections: []learning.Correction{}, Weaknesses: []string{}, Vocabulary: []string{}, Level: "A1"})
	return b
}

func TestLessonFeatureProgressAutoFinishAndReplay(t *testing.T) {
	a := featureApp(t)
	var tutorCalls int
	var ttsCalls int
	ttsSucceeds := false
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "flash-preview-tts") {
			ttsCalls++
			if ttsSucceeds {
				_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"finishReason": "STOP", "content": map[string]any{"parts": []any{map[string]any{"inlineData": map[string]any{"mimeType": "audio/wav", "data": base64.StdEncoding.EncodeToString(gemini.WAV(make([]byte, 32000), 16000))}}}}}}, "usageMetadata": map[string]any{"promptTokenCount": 1, "candidatesTokenCount": 1}})
				return
			}
			http.Error(w, "tts unavailable", http.StatusBadGateway)
			return
		}
		tutorCalls++
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"finishReason": "STOP", "content": map[string]any{"parts": []any{map[string]any{"text": string(feedbackJSON())}}}}}, "usageMetadata": map[string]any{"promptTokenCount": 1, "candidatesTokenCount": 1}})
	}))
	defer fake.Close()
	a.AI = &gemini.Client{Key: "test-key", HTTP: fake.Client(), BaseURL: fake.URL + "/", Models: a.Cfg.Models, DefaultTimeout: a.Cfg.TimeoutSeconds}
	uid := uuid.NewString()
	if _, err := a.DB.Exec(context.Background(), "INSERT INTO users(id,username,password_hash) VALUES($1,$2,$3)", uid, "feature-learner", security.Hash("feature-password")); err != nil {
		t.Fatal(err)
	}
	call := func(want int, method, path, token string, body any) map[string]any {
		t.Helper()
		var b []byte
		if body != nil {
			b = asJSON(body)
		}
		req := httptest.NewRequest(method, "http://localhost/ai-tutor/api/v2"+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := a.HTTP.Test(req, 60000)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		out, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, res.StatusCode, want, out)
		}
		var v map[string]any
		if err := json.Unmarshal(out, &v); err != nil {
			t.Fatalf("decode %s: %v", out, err)
		}
		return v
	}
	token := textValue(call(200, "POST", "/auth/login", "", map[string]any{"username": "feature-learner", "password": "feature-password"})["token"])
	cacheGet := func(auth string) string {
		t.Helper()
		req := httptest.NewRequest("GET", "http://localhost/ai-tutor/api/v2/curriculum", nil)
		req.Header.Set("Authorization", "Bearer "+auth)
		res, err := a.HTTP.Test(req, 60000)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("curriculum cache request: %d", res.StatusCode)
		}
		return res.Header.Get("X-App-Cache")
	}
	if got := cacheGet(token); got != "MISS" {
		t.Fatalf("first per-user cache read = %q", got)
	}
	if got := cacheGet(token); got != "HIT" {
		t.Fatalf("repeat cache read = %q", got)
	}
	call(200, "PATCH", "/profile", token, map[string]any{"level": "A1"})
	if got := cacheGet(token); got != "MISS" {
		t.Fatalf("write did not invalidate user cache: %q", got)
	}
	otherID := uuid.NewString()
	if _, err := a.DB.Exec(context.Background(), "INSERT INTO users(id,username,password_hash) VALUES($1,$2,$3)", otherID, "feature-other", security.Hash("feature-password")); err != nil {
		t.Fatal(err)
	}
	other := textValue(call(200, "POST", "/auth/login", "", map[string]any{"username": "feature-other", "password": "feature-password"})["token"])
	if got := cacheGet(other); got != "MISS" {
		t.Fatalf("other user received cached response: %q", got)
	}
	sid := textValue(call(201, "POST", "/sessions", token, map[string]any{"mode": "lesson", "lesson_id": "lesson-001", "auto_audio": true})["id"])
	call(200, "POST", "/sessions/"+sid+"/advance", token, map[string]any{})
	audioTurnFor := func(sessionID, requestID string) map[string]any {
		t.Helper()
		var b bytes.Buffer
		m := multipart.NewWriter(&b)
		_ = m.WriteField("request_id", requestID)
		f, err := m.CreateFormFile("audio", "turn.wav")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write(gemini.WAV(make([]byte, 32000), 16000))
		_ = m.Close()
		req := httptest.NewRequest("POST", "http://localhost/ai-tutor/api/v2/sessions/"+sessionID+"/turns", &b)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", m.FormDataContentType())
		res, err := a.HTTP.Test(req, 60000)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		out, _ := io.ReadAll(res.Body)
		if res.StatusCode != 200 {
			t.Fatalf("audio turn: %d %s", res.StatusCode, out)
		}
		var v map[string]any
		if err := json.Unmarshal(out, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	audioTurn := func(requestID string) map[string]any { return audioTurnFor(sid, requestID) }
	firstID := uuid.NewString()
	first := audioTurn(firstID)
	beforeReplay := tutorCalls
	replay := audioTurn(firstID)
	if first["id"] != replay["id"] || tutorCalls != beforeReplay {
		t.Fatalf("turn replay was not idempotent: %#v %#v calls=%d/%d", first, replay, tutorCalls, beforeReplay)
	}
	if textValue(first["audio_error"]) == "" {
		t.Fatal("reply TTS failure was not reported")
	}
	var replyTurn string
	if err := a.DB.QueryRow(context.Background(), "SELECT reply_turn_id::text FROM attempts WHERE id=$1", textValue(first["id"])).Scan(&replyTurn); err != nil {
		t.Fatal(err)
	}
	translated := call(200, "POST", "/sessions/"+sid+"/turns/"+replyTurn+"/translate", token, map[string]any{})
	if textValue(translated["text_th"]) == "" {
		t.Fatalf("turn was not retained for translation after reply audio failure: %#v", translated)
	}
	if resumed := call(200, "POST", "/sessions", token, map[string]any{"mode": "lesson", "lesson_id": "lesson-001"}); textValue(resumed["id"]) != sid || resumed["resumed"] != true {
		t.Fatalf("active session was not preserved on resume: %#v", resumed)
	}
	for i := 0; i < 4; i++ {
		call(200, "POST", "/sessions/"+sid+"/advance", token, map[string]any{})
		if i < 3 {
			audioTurn(uuid.NewString())
		}
	}
	// The first turn plus three more drills reach conversation; every drill needed a pass and advance.
	state := call(200, "GET", "/sessions/"+sid, token, nil)["session"].(map[string]any)
	progress := state["progress"].(map[string]any)
	if number(progress["completed_drills"], 0) != 4 || number(progress["independent_conversations"], 0) != 0 {
		t.Fatalf("drill progress wrong: %#v", progress)
	}
	firstConversation := audioTurn(uuid.NewString())
	if firstConversation["session_completed"] == true || number(firstConversation["progress"].(map[string]any)["independent_conversations"], 0) != 1 {
		t.Fatalf("first conversation completed early: %#v", firstConversation)
	}
	call(200, "POST", "/sessions/"+sid+"/advance", token, map[string]any{})
	secondConversation := audioTurn(uuid.NewString())
	if secondConversation["session_completed"] != true || secondConversation["progress"].(map[string]any)["ready_to_complete"] != true {
		t.Fatalf("six required independent steps did not auto-complete: %#v", secondConversation)
	}
	var completed bool
	var independent int
	if err := a.DB.QueryRow(context.Background(), "SELECT completed,independent_successes FROM mastery WHERE user_id=$1 AND lesson_id='lesson-001'", uid).Scan(&completed, &independent); err != nil {
		t.Fatal(err)
	}
	if !completed || independent != 2 {
		t.Fatalf("mastery wrong: completed=%v independent=%d", completed, independent)
	}

	// Typed work and hinted audio remain studied work, but cannot confer speaking mastery.
	plain := textValue(call(201, "POST", "/sessions", token, map[string]any{"mode": "lesson", "lesson_id": "lesson-002"})["id"])
	if _, err := a.DB.Exec(context.Background(), "UPDATE learning_sessions SET state=$1 WHERE id=$2", asJSON(map[string]any{"stage": "conversation", "step": 0, "hint_level": 0, "independent": 0, "last_pass": false, "step_started_at": 0}), plain); err != nil {
		t.Fatal(err)
	}
	call(200, "POST", "/sessions/"+plain+"/turns", token, map[string]any{"text": "I work in Bangkok.", "request_id": uuid.NewString()})
	if _, err := a.DB.Exec(context.Background(), "UPDATE learning_sessions SET state=jsonb_set(state,'{hint_level}','1') WHERE id=$1", plain); err != nil {
		t.Fatal(err)
	}
	assisted := audioTurnFor(plain, uuid.NewString())
	if assisted["independent"] != false {
		t.Fatalf("hinted audio was incorrectly independent: %#v", assisted)
	}
	manual := call(200, "POST", "/sessions/"+plain+"/complete", token, map[string]any{})
	if manual["mastered"] != false {
		t.Fatalf("typed work awarded mastery: %#v", manual)
	}

	// Successful reply audio is saved once and returned again by a replay without another TTS call.
	ttsSucceeds = true
	successSession := textValue(call(201, "POST", "/sessions", token, map[string]any{"mode": "lesson", "lesson_id": "lesson-003", "auto_audio": true})["id"])
	if _, err := a.DB.Exec(context.Background(), "UPDATE learning_sessions SET state=$1 WHERE id=$2", asJSON(map[string]any{"stage": "conversation", "step": 0, "hint_level": 0, "independent": 0, "last_pass": false, "step_started_at": 0, "auto_audio": true}), successSession); err != nil {
		t.Fatal(err)
	}
	successRequest := uuid.NewString()
	success := call(200, "POST", "/sessions/"+successSession+"/turns", token, map[string]any{"text": "I work in Bangkok.", "request_id": successRequest})
	if textValue(success["reply_audio_id"]) == "" {
		t.Fatalf("successful reply audio missing: %#v", success)
	}
	beforeReplayTTS := ttsCalls
	successReplay := call(200, "POST", "/sessions/"+successSession+"/turns", token, map[string]any{"text": "I work in Bangkok.", "request_id": successRequest})
	if successReplay["id"] != success["id"] || successReplay["reply_audio_id"] != success["reply_audio_id"] || ttsCalls != beforeReplayTTS {
		t.Fatalf("reply audio replay was not idempotent: first=%#v replay=%#v tts=%d/%d", success, successReplay, ttsCalls, beforeReplayTTS)
	}
}

type fakeStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	deleted []string
	puts    []string
	putErr  error
}

func (s *fakeStore) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	b, e := io.ReadAll(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.puts = append(s.puts, key)
	if s.putErr != nil {
		return s.putErr
	}
	if e == nil {
		s.objects[key] = b
	}
	return e
}
func (s *fakeStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	s.mu.Lock()
	b, ok := s.objects[key]
	s.mu.Unlock()
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (s *fakeStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	s.deleted = append(s.deleted, key)
	return nil
}

func TestMediaStoreRestoresLocalCopyAndExpiresRemoteObject(t *testing.T) {
	a := featureApp(t)
	store := &fakeStore{objects: map[string][]byte{}}
	a.Objects = store
	uid, restoreID, expiredID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err := a.DB.Exec(context.Background(), "INSERT INTO users(id,username,password_hash) VALUES($1,$2,$3)", uid, "media-learner", security.Hash("media-password")); err != nil {
		t.Fatal(err)
	}
	restorePath := a.Cfg.AudioDir + "/" + restoreID + ".wav"
	restoreKey := a.objectKey(restoreID, true)
	store.objects[restoreKey] = []byte("restored")
	if _, err := a.DB.Exec(context.Background(), "INSERT INTO audio_assets(id,user_id,path,mime,object_key,uploaded_at) VALUES($1,$2,$3,'audio/wav',$4,now())", restoreID, uid, restorePath, restoreKey); err != nil {
		t.Fatal(err)
	}
	if err := a.ensureLocalAudio(context.Background(), restoreID, restorePath, restoreKey); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(restorePath); err != nil || string(b) != "restored" {
		t.Fatalf("remote restore failed: %q %v", b, err)
	}
	expiredPath := a.Cfg.AudioDir + "/" + expiredID + ".wav"
	expiredKey := a.objectKey(expiredID, true)
	if err := os.WriteFile(expiredPath, []byte("expired"), 0600); err != nil {
		t.Fatal(err)
	}
	store.objects[expiredKey] = []byte("expired")
	if _, err := a.DB.Exec(context.Background(), "INSERT INTO audio_assets(id,user_id,path,mime,object_key,expires_at) VALUES($1,$2,$3,'audio/wav',$4,now()-interval '1 second')", expiredID, uid, expiredPath, expiredKey); err != nil {
		t.Fatal(err)
	}
	a.expireAudio(context.Background())
	var n int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM audio_assets WHERE id=$1", expiredID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("expired metadata was retained")
	}
	if _, err := os.Stat(expiredPath); !os.IsNotExist(err) {
		t.Fatalf("expired local file remained: %v", err)
	}
	store.mu.Lock()
	deleted := append([]string(nil), store.deleted...)
	store.mu.Unlock()
	if len(deleted) != 1 || deleted[0] != expiredKey {
		t.Fatalf("expired object was not deleted: %#v", deleted)
	}

	// A failed upload records its retry without marking the asset uploaded; a later retry uploads once.
	pendingID := uuid.NewString()
	pendingPath := a.Cfg.AudioDir + "/" + pendingID + ".wav"
	pendingKey := a.objectKey(pendingID, false)
	if err := os.WriteFile(pendingPath, []byte("pending"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(context.Background(), "INSERT INTO audio_assets(id,user_id,path,mime,object_key) VALUES($1,$2,$3,'audio/wav',$4)", pendingID, uid, pendingPath, pendingKey); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.putErr = errors.New("temporary object-store failure")
	store.mu.Unlock()
	a.syncMedia(context.Background())
	var uploaded bool
	if err := a.DB.QueryRow(context.Background(), "SELECT uploaded_at IS NOT NULL FROM audio_assets WHERE id=$1", pendingID).Scan(&uploaded); err != nil || uploaded {
		t.Fatalf("failed upload marked durable: uploaded=%v err=%v", uploaded, err)
	}
	store.mu.Lock()
	store.putErr = nil
	store.mu.Unlock()
	if _, err := a.DB.Exec(context.Background(), "UPDATE audio_assets SET upload_retry_at=now() WHERE id=$1", pendingID); err != nil {
		t.Fatal(err)
	}
	a.syncMedia(context.Background())
	if err := a.DB.QueryRow(context.Background(), "SELECT uploaded_at IS NOT NULL FROM audio_assets WHERE id=$1", pendingID).Scan(&uploaded); err != nil || !uploaded {
		t.Fatalf("retry did not upload asset: uploaded=%v err=%v", uploaded, err)
	}
	expiredPending := uuid.NewString()
	expiredPendingPath := a.Cfg.AudioDir + "/" + expiredPending + ".wav"
	expiredPendingKey := a.objectKey(expiredPending, true)
	if err := os.WriteFile(expiredPendingPath, []byte("expired pending"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(context.Background(), "INSERT INTO audio_assets(id,user_id,path,mime,object_key,expires_at) VALUES($1,$2,$3,'audio/wav',$4,now()-interval '1 second')", expiredPending, uid, expiredPendingPath, expiredPendingKey); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	putsBeforeExpiredSync := len(store.puts)
	store.mu.Unlock()
	a.syncMedia(context.Background())
	store.mu.Lock()
	putsAfterExpiredSync := len(store.puts)
	store.mu.Unlock()
	if putsAfterExpiredSync != putsBeforeExpiredSync {
		t.Fatal("expired pending audio was uploaded")
	}
}
