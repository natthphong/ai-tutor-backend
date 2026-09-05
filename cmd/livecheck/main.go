// Opt-in black-box Live QA with app-generated synthetic audio, never a microphone.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type apiClient struct {
	base, token string
	h           *http.Client
}

func (a *apiClient) call(method, path string, value any) (int, map[string]any) {
	var body io.Reader
	if value != nil {
		b, _ := json.Marshal(value)
		if raw, ok := value.(json.RawMessage); ok {
			b = raw
		}
		body = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, a.base+path, body)
	req.Header.Set("User-Agent", "TokoLoop-QA/2")
	if value != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	res, err := a.h.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return res.StatusCode, out
}

func (a *apiClient) must(method, path string, value any) map[string]any {
	status, out := a.call(method, path, value)
	if status >= 300 {
		panic(fmt.Sprintf("%s %s HTTP %d", method, path, status))
	}
	return out
}

func dial(ctx context.Context, url, origin string) (*websocket.Conn, *http.Response, error) {
	return websocket.DefaultDialer.DialContext(ctx, url, http.Header{"Origin": []string{origin}, "User-Agent": []string{"TokoLoop-QA/2"}})
}

func waitInactive(api *apiClient, sid string) {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		s := api.must("GET", "/sessions/"+sid, nil)
		state := s["session"].(map[string]any)["state"].(map[string]any)
		if state["live_active"] == false {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	panic("live_active remained true after stop")
}

func writePCM(ws *websocket.Conn, pcm []byte) error {
	for start := 0; start < len(pcm); start += 640 {
		end := min(start+640, len(pcm))
		if err := ws.WriteMessage(websocket.BinaryMessage, pcm[start:end]); err != nil {
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

func main() {
	base := os.Getenv("TEST_API_URL")
	if base == "" {
		base = "http://localhost:8080/ai-tutor/api/v2"
	}
	origin := os.Getenv("TEST_ORIGIN")
	if origin == "" {
		origin = "http://localhost:3000"
	}
	wavPath := os.Getenv("QA_TTS_WAV")
	if wavPath == "" {
		wavPath = "/tmp/toko-qa-tts.wav"
	}
	pcm, err := exec.Command("ffmpeg", "-v", "error", "-i", wavPath, "-vn", "-ac", "1", "-ar", "16000", "-f", "s16le", "pipe:1").Output()
	if err != nil || len(pcm) == 0 {
		panic("cannot decode app-generated TTS fixture")
	}
	pcm = append(pcm, make([]byte, 32000)...)

	credentials, err := os.ReadFile(".toko-qa.json")
	if err != nil {
		panic(err)
	}
	api := &apiClient{base: strings.TrimRight(base, "/"), h: &http.Client{Timeout: 55 * time.Second}}
	api.token = api.must("POST", "/auth/login", json.RawMessage(credentials))["token"].(string)
	before := api.must("GET", "/usage", nil)
	beforeCalls := int(before["calls"].(float64))
	beforeSeconds := int(before["live_seconds"].(float64))

	result := map[string]any{
		"target": base, "synthetic_only": true, "fixture": wavPath,
		"wrong_origin_blocked": false, "ticket_one_use": false, "ready": false,
		"input_transcription": false, "audio_received": false, "output_transcription": false,
		"stop_clears_active": false, "reconnect_ready": false,
		"reconnect_clears_active": false, "ledger_stable_after_stop": false,
	}

	first := api.must("POST", "/sessions", map[string]any{"mode": "live"})
	sid := first["id"].(string)
	result["session_id"] = sid
	url := api.must("POST", "/sessions/"+sid+"/live-ticket", map[string]any{})["url"].(string)

	wrongCtx, wrongCancel := context.WithTimeout(context.Background(), 8*time.Second)
	wrong, response, wrongErr := dial(wrongCtx, url, "http://wrong-origin.invalid")
	wrongCancel()
	if wrong != nil {
		wrong.Close()
	}
	if wrongErr == nil || response == nil || response.StatusCode != http.StatusForbidden {
		panic("wrong WebSocket origin was not rejected with 403")
	}
	result["wrong_origin_blocked"] = true

	ctx, cancel := context.WithTimeout(context.Background(), 28*time.Second)
	ws, response, err := dial(ctx, url, origin)
	if err != nil {
		cancel()
		if response != nil {
			panic(fmt.Sprintf("Live upgrade HTTP %d", response.StatusCode))
		}
		panic(err)
	}
	ws.SetReadDeadline(time.Now().Add(27 * time.Second))
	sentInput, turnComplete := false, false
	for {
		_, b, readErr := ws.ReadMessage()
		if readErr != nil {
			ws.Close()
			cancel()
			panic(readErr)
		}
		var event map[string]json.RawMessage
		_ = json.Unmarshal(b, &event)
		if raw, ok := event["error"]; ok {
			ws.Close()
			cancel()
			panic("Live error: " + string(raw))
		}
		if _, ok := event["ready"]; ok {
			result["ready"] = true
			if !sentInput {
				sentInput = true
				if err = writePCM(ws, pcm); err != nil {
					panic(err)
				}
			}
		}
		if raw, ok := event["serverContent"]; ok {
			var content map[string]json.RawMessage
			_ = json.Unmarshal(raw, &content)
			if v, ok := content["inputTranscription"]; ok && len(v) > 2 {
				result["input_transcription"] = true
			}
			if _, ok := content["modelTurn"]; ok {
				result["audio_received"] = true
			}
			if v, ok := content["outputTranscription"]; ok && len(v) > 2 {
				result["output_transcription"] = true
			}
			var marker struct {
				TurnComplete bool `json:"turnComplete"`
			}
			_ = json.Unmarshal(raw, &marker)
			turnComplete = turnComplete || marker.TurnComplete
		}
		if sentInput && turnComplete && result["input_transcription"] == true && result["audio_received"] == true && result["output_transcription"] == true {
			break
		}
	}
	_ = ws.WriteJSON(map[string]string{"type": "stop"})
	ws.Close()
	cancel()
	waitInactive(api, sid)
	result["stop_clears_active"] = true

	reuseCtx, reuseCancel := context.WithTimeout(context.Background(), 8*time.Second)
	reused, response, reuseErr := dial(reuseCtx, url, origin)
	reuseCancel()
	if reused != nil {
		reused.Close()
	}
	if reuseErr == nil || response == nil || response.StatusCode != http.StatusUnauthorized {
		panic("consumed Live ticket was accepted")
	}
	result["ticket_one_use"] = true

	secondURL := api.must("POST", "/sessions/"+sid+"/live-ticket", map[string]any{})["url"].(string)
	reCtx, reCancel := context.WithTimeout(context.Background(), 12*time.Second)
	reWS, response, err := dial(reCtx, secondURL, origin)
	if err != nil {
		reCancel()
		if response != nil {
			panic(fmt.Sprintf("reconnect HTTP %d", response.StatusCode))
		}
		panic(err)
	}
	reWS.SetReadDeadline(time.Now().Add(10 * time.Second))
	for {
		_, b, readErr := reWS.ReadMessage()
		if readErr != nil {
			panic(readErr)
		}
		var event map[string]json.RawMessage
		_ = json.Unmarshal(b, &event)
		if _, ok := event["ready"]; ok {
			result["reconnect_ready"] = true
			break
		}
		if raw, ok := event["error"]; ok {
			panic("reconnect Live error: " + string(raw))
		}
	}
	_ = reWS.WriteJSON(map[string]string{"type": "stop"})
	reWS.Close()
	reCancel()
	waitInactive(api, sid)
	result["reconnect_clears_active"] = true

	after := api.must("GET", "/usage", nil)
	time.Sleep(3 * time.Second)
	stable := api.must("GET", "/usage", nil)
	result["calls_delta"] = int(stable["calls"].(float64)) - beforeCalls
	result["live_seconds_delta"] = int(stable["live_seconds"].(float64)) - beforeSeconds
	result["ledger_stable_after_stop"] = after["calls"] == stable["calls"] && after["live_seconds"] == stable["live_seconds"] && after["spent"] == stable["spent"]
	if result["calls_delta"].(int) != 2 || result["live_seconds_delta"].(int) > 30 || result["ledger_stable_after_stop"] != true {
		panic("Live ledger was not bounded/stable")
	}
	for key, value := range result {
		if flag, ok := value.(bool); ok && !flag {
			panic("failed check: " + key)
		}
	}
	result["status"] = "passed"
	_ = os.MkdirAll("reports", 0755)
	b, _ := json.MarshalIndent(result, "", "  ")
	if err = os.WriteFile("reports/live-smoke.json", append(b, '\n'), 0644); err != nil {
		panic(err)
	}
	fmt.Println("Live QA passed: origin, one-use ticket, synthetic input/output, stop, reconnect, ledger")
}
