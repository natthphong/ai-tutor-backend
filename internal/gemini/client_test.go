package gemini

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"tokoloop/internal/config"
)

func TestPerModalityMetering(t *testing.T) {
	var u Usage
	if e := json.Unmarshal([]byte(`{"promptTokenCount":1000,"responseTokenCount":500,"thoughtsTokenCount":30,"promptTokensDetails":[{"modality":"AUDIO","tokenCount":600}],"responseTokensDetails":[{"modality":"AUDIO","tokenCount":400}]}`), &u); e != nil {
		t.Fatal(e)
	}
	m := config.Model{Input: .75, AudioInput: 3, Output: 4.5, AudioOutput: 12}
	want := (400*.75 + 600*3 + 130*4.5 + 400*12) * 35 / 1e6
	if math.Abs(u.Cost(m, 35)-want) > .00001 {
		t.Fatalf("cost %f expected %f", u.Cost(m, 35), want)
	}
	var sum Usage
	sum.Add(u)
	sum.Add(u)
	if sum.Input != 2000 || sum.Output != 1000 || sum.AudioInput() != 1200 || sum.AudioOutput() != 800 {
		t.Fatal("live must accumulate every response", sum)
	}
}
func TestClientRejectsUnavailableAndEmpty(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
	}{{429, `{}`}, {200, `{"candidates":[]}`}, {200, `{"candidates":[{"finishReason":"MAX_TOKENS","content":{"parts":[{"text":"partial"}]}}]}`}} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("x-goog-api-key") != "test" {
				t.Error("key missing")
			}
			if r.URL.Query().Get("key") != "" {
				t.Error("key in URL")
			}
			w.WriteHeader(tc.status)
			w.Write([]byte(tc.body))
		}))
		c := Client{Key: "test", HTTP: s.Client(), BaseURL: s.URL + "/"}
		_, e := c.Generate(context.Background(), config.Model{ID: "gemini-test", MaxTokens: 30}, "system", "test", nil, "", nil, "")
		s.Close()
		if e == nil {
			t.Fatalf("accepted %s", tc.body)
		}
	}
}
func TestWAV(t *testing.T) {
	pcm := []byte{1, 2, 3, 4}
	b := WAV(pcm, 16000)
	if len(b) != 48 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		t.Fatal("invalid WAV")
	}
}
