package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"tokoloop/internal/gemini"

	"github.com/google/uuid"
)

func TestDailySplitSentencesKeepsPunctuationAndOrder(t *testing.T) {
	got := dailySplitSentences("I fixed the retry bug.  We are still waiting for QA!\nWill they confirm tomorrow? \"Maybe.\" I hope so")
	want := []string{"I fixed the retry bug.", "We are still waiting for QA!", "Will they confirm tomorrow?", "\"Maybe.\"", "I hope so"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("split = %q, want %q", got, want)
	}
	if len(dailySplitSentences(strings.Repeat("Go. ", 80))) != dailyMaxSentences {
		t.Fatal("sentence list is not capped")
	}
}

func TestDailyResplitKeepsAnalysisOfUnchangedSentences(t *testing.T) {
	old := []DailySentence{{EN: "I fixed the bug.", TH: "ฉันแก้บั๊กแล้ว", Reading: "ไอ ฟิกซ์ เดอะ บั๊ก", Grammar: []DailyGrammar{{ID: "ebook-005"}}}, {EN: "QA will test it.", TH: "QA จะทดสอบ", Reading: "คิว-เอ วิล เทสต์ อิท"}}
	same, analyzed := dailyResplit(old, "I fixed the bug. QA will test it.")
	if !analyzed || !dailySameSentences(old, same) || same[0].Reading == "" {
		t.Fatalf("unchanged text lost its analysis: %#v analyzed=%v", same, analyzed)
	}
	edited, analyzed := dailyResplit(old, "I fixed the bug. QA tests it tomorrow.")
	if analyzed || len(edited) != 2 || edited[0].TH == "" || edited[1].TH != "" || edited[1].Grammar == nil {
		t.Fatalf("edited text = %#v analyzed=%v", edited, analyzed)
	}
}

func (h featureHTTP) dailySpeak(want int, dailyID, requestID, target, level string) map[string]any {
	h.t.Helper()
	var data bytes.Buffer
	m := multipart.NewWriter(&data)
	_ = m.WriteField("request_id", requestID)
	_ = m.WriteField("target", target)
	_ = m.WriteField("level", level)
	f, err := m.CreateFormFile("audio", "fixture.wav")
	if err != nil {
		h.t.Fatal(err)
	}
	_, _ = f.Write(gemini.WAV(make([]byte, 32000), 16000))
	_ = m.Close()
	req := httptest.NewRequest("POST", "http://localhost/ai-tutor/api/v2/daily-meets/"+dailyID+"/speak", &data)
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Content-Type", m.FormDataContentType())
	res, err := h.a.HTTP.Test(req, 60000)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != want {
		h.t.Fatalf("daily speak: got %d want %d: %s", res.StatusCode, want, raw)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		h.t.Fatal(err)
	}
	return out
}

func TestDailyMeetSentencesGrammarAndSpeakingDrill(t *testing.T) {
	a := featureApp(t)
	lesson := a.Course.Path()[0]
	sentences := []any{
		map[string]any{"en": "I fixed the retry bug.", "th": "ฉันแก้บั๊ก retry แล้ว", "reading_th": "ไอ ฟิกซ์ เดอะ รี-ไทร บั๊ก", "units": []string{lesson.ID, "not-a-unit", lesson.ID}},
		map[string]any{"en": "QA will confirm tomorrow.", "th": "QA จะยืนยันพรุ่งนี้", "reading_th": "คิว-เอ วิล คัน-เฟิร์ม ทู-มอร์-โรว์", "units": []string{}},
	}
	daily, _ := json.Marshal(map[string]any{"title": "Stand-up", "english": "Today I fixed the retry bug, and QA will confirm tomorrow.", "thai": "ฉันแก้บั๊ก retry แล้ว QA จะยืนยันพรุ่งนี้", "question": "What did you fix?", "question_th": "คุณแก้อะไร", "phrases": []any{}, "sentences": sentences})
	analysis, _ := json.Marshal(map[string]any{"sentences": []any{
		map[string]any{"en": "I fixed the retry bug.", "th": "ฉันแก้บั๊ก retry แล้ว", "reading_th": "ไอ ฟิกซ์ เดอะ รี-ไทร บั๊ก", "units": []string{lesson.ID}},
		map[string]any{"en": "QA tests it tomorrow.", "th": "QA ทดสอบพรุ่งนี้", "reading_th": "คิว-เอ เทสต์ส อิท ทู-มอร์-โรว์", "units": []string{}},
	}})
	verdict := map[string]any{"transcript": "I fixed the retry bug.", "audio_clear": true, "matched": true, "missing_words": []string{}, "tip_th": "ดีมาก", "pronunciation_th": ""}
	var speakCalls atomic.Int32
	var matched atomic.Bool
	matched.Store(true)
	fake := featureAI(a, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		text := string(asJSON(body))
		switch {
		case strings.Contains(text, "Notes:"):
			if !strings.Contains(text, lesson.ID) {
				t.Errorf("daily meet prompt has no grammar catalog")
			}
			response(w, string(daily))
		case strings.Contains(text, "English text:"):
			response(w, string(analysis))
		default:
			speakCalls.Add(1)
			out := map[string]any{}
			for key, value := range verdict {
				out[key] = value
			}
			out["matched"] = matched.Load()
			response(w, string(asJSON(out)))
		}
	})
	defer fake.Close()
	h := featureLogin(t, a, "drill-owner")
	other := featureLogin(t, a, "drill-other")

	job := h.call(202, "POST", "/daily-meets", map[string]any{"day": "2026-10-11", "title": "Stand-up", "source": "แก้ retry bug แล้ว รอ QA confirm พรุ่งนี้", "request_id": uuid.NewString()})
	a.runJob(context.Background())
	var dailyID string
	var raw []byte
	if err := a.DB.QueryRow(context.Background(), "SELECT id::text,data FROM daily_meets WHERE user_id=(SELECT user_id FROM jobs WHERE id=$1)", textValue(job["job_id"])).Scan(&dailyID, &raw); err != nil {
		t.Fatal(err)
	}
	var created DailyMeet
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatal(err)
	}
	if created.English != "I fixed the retry bug. QA will confirm tomorrow." {
		t.Fatalf("entry text %q is not the sentences joined", created.English)
	}
	if !created.Analyzed || len(created.Sentences) != 2 || created.Sentences[0].Reading == "" {
		t.Fatalf("created sentences = %#v analyzed=%v", created.Sentences, created.Analyzed)
	}
	if grammar := created.Sentences[0].Grammar; len(grammar) != 1 || grammar[0].ID != lesson.ID || grammar[0].Title != lesson.Title || grammar[0].Level != lesson.Level {
		t.Fatalf("grammar units = %#v, want only the known unit once with its title", grammar)
	}
	if created.Sentences[1].Grammar == nil {
		t.Fatal("a sentence without units must store an empty list, not null")
	}

	// A wrong attempt records nothing; a clear matching one records the level.
	matched.Store(false)
	if miss := h.dailySpeak(200, dailyID, uuid.NewString(), "0", "1"); miss["passed"] != false || len(decodeDrillLevels(t, miss)) != 0 {
		t.Fatalf("unmatched attempt = %#v", miss)
	}
	matched.Store(true)
	requestID := uuid.NewString()
	first := h.dailySpeak(200, dailyID, requestID, "0", "2")
	if first["passed"] != true || decodeDrillLevels(t, first)["0"] != 2 {
		t.Fatalf("matched attempt = %#v", first)
	}
	calls := speakCalls.Load()
	if replay := h.dailySpeak(200, dailyID, requestID, "0", "2"); replay["transcript"] != first["transcript"] || speakCalls.Load() != calls {
		t.Fatalf("replay called the model again or changed: %#v", replay)
	}
	// An easier level later never lowers the recorded one.
	if lower := h.dailySpeak(200, dailyID, uuid.NewString(), "0", "1"); decodeDrillLevels(t, lower)["0"] != 2 {
		t.Fatalf("level went down: %#v", lower)
	}
	whole := h.dailySpeak(200, dailyID, uuid.NewString(), "all", "3")
	if drill, _ := whole["drill"].(map[string]any); drill["full_passed"] != true {
		t.Fatalf("whole entry = %#v", whole)
	}
	h.dailySpeak(400, dailyID, uuid.NewString(), "9", "1")
	h.dailySpeak(400, dailyID, uuid.NewString(), "0", "4")
	other.dailySpeak(404, dailyID, uuid.NewString(), "0", "1")
	other.call(404, "POST", "/daily-meets/"+dailyID+"/analyze", map[string]any{"request_id": uuid.NewString()})

	// Saving the same text keeps progress; editing it restarts the drill and
	// marks the entry for analysis again.
	kept := h.call(200, "PATCH", "/daily-meets/"+dailyID, map[string]any{"title": "Stand-up", "english": created.English, "thai": created.Thai})
	if kept["analyzed"] != true || decodeDrillLevels(t, kept)["0"] != 2 {
		t.Fatalf("unchanged save lost progress: %#v", kept)
	}
	edited := h.call(200, "PATCH", "/daily-meets/"+dailyID, map[string]any{"title": "Stand-up", "english": "I fixed the retry bug. QA tests it tomorrow.", "thai": created.Thai})
	if edited["analyzed"] != false || len(decodeDrillLevels(t, edited)) != 0 {
		t.Fatalf("edited entry = %#v", edited)
	}
	analyzeID := uuid.NewString()
	queued := h.call(202, "POST", "/daily-meets/"+dailyID+"/analyze", map[string]any{"request_id": analyzeID})
	if again := h.call(202, "POST", "/daily-meets/"+dailyID+"/analyze", map[string]any{"request_id": analyzeID}); again["job_id"] != queued["job_id"] {
		t.Fatalf("analyze replay made a second job: %#v", again)
	}
	a.runJob(context.Background())
	if err := a.DB.QueryRow(context.Background(), "SELECT data FROM daily_meets WHERE id=$1", dailyID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var analyzed DailyMeet
	if err := json.Unmarshal(raw, &analyzed); err != nil {
		t.Fatal(err)
	}
	if !analyzed.Analyzed || len(analyzed.Sentences) != 2 || analyzed.Sentences[1].Reading == "" {
		t.Fatalf("analysis job result = %#v", analyzed)
	}
}

func decodeDrillLevels(t *testing.T, body map[string]any) map[string]float64 {
	t.Helper()
	out := map[string]float64{}
	drill, _ := body["drill"].(map[string]any)
	levels, _ := drill["levels"].(map[string]any)
	for key, value := range levels {
		out[key] = number(value, 0)
	}
	return out
}
