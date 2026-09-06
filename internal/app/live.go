package app

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	fw "github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	gw "github.com/gorilla/websocket"

	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"tokoloop/internal/gemini"
	"tokoloop/internal/learning"
	"tokoloop/internal/security"
)

func (a *App) liveTicket(c *fiber.Ctx) error {
	s, e := a.findSession(c)
	if e != nil {
		return e
	}
	if s.Status != "active" || user(c).MustChange {
		return fail(c, 409, "session ไม่พร้อมสำหรับ Live")
	}
	if a.Cfg.GeminiKey == "" {
		return fail(c, 503, "ยังไม่ได้ตั้งค่า Gemini")
	}
	t := security.Token()
	_, e = a.DB.Exec(c.UserContext(), "INSERT INTO live_tickets(token_hash,session_id,user_id,expires_at) VALUES($1,$2,$3,now()+interval '30 seconds')", security.Digest(t), s.ID, user(c).ID)
	if e != nil {
		return e
	}
	base := strings.Replace(strings.Replace(a.Cfg.PublicURL, "https://", "wss://", 1), "http://", "ws://", 1)
	return c.JSON(fiber.Map{"url": base + "/ai-tutor/api/v2/live?ticket=" + t, "expires_in": 30})
}
func (a *App) liveRoute(g fiber.Router) {
	g.Get("/live", func(c *fiber.Ctx) error {
		origin := c.Get("Origin")
		allowed := false
		for _, o := range a.Cfg.Origins {
			if origin == o {
				allowed = true
			}
		}
		if !allowed {
			return fail(c, 403, "origin not allowed")
		}
		if !fw.IsWebSocketUpgrade(c) {
			return fail(c, 426, "WebSocket required")
		}
		var uid, sid string
		var profile []byte
		tx, e := a.DB.Begin(c.UserContext())
		if e != nil {
			return e
		}
		defer tx.Rollback(c.UserContext())
		e = tx.QueryRow(c.UserContext(), "UPDATE live_tickets SET consumed=true WHERE token_hash=$1 AND expires_at>now() AND NOT consumed RETURNING user_id::text,session_id::text", security.Digest(c.Query("ticket"))).Scan(&uid, &sid)
		if e != nil {
			return fail(c, 401, "Live ticket expired")
		}
		e = tx.QueryRow(c.UserContext(), "SELECT profile FROM users WHERE id=$1 AND NOT must_change_password FOR UPDATE", uid).Scan(&profile)
		if e != nil {
			return fail(c, 403, "change password first")
		}
		var active int
		e = tx.QueryRow(c.UserContext(), "SELECT count(*) FROM learning_sessions WHERE user_id=$1 AND state->>'live_active'='true'", uid).Scan(&active)
		if e != nil {
			return e
		}
		if active > 0 {
			return fail(c, 409, "Live already active")
		}
		tag, e := tx.Exec(c.UserContext(), "UPDATE learning_sessions SET state=state||'{\"live_active\":true}'::jsonb,updated_at=now() WHERE id=$1 AND status='active'", sid)
		if e != nil {
			return e
		}
		if tag.RowsAffected() == 0 {
			return fail(c, 409, "session completed")
		}
		if e = tx.Commit(c.UserContext()); e != nil {
			return e
		}
		c.Locals("live_uid", uid)
		c.Locals("live_sid", sid)
		c.Locals("live_profile", profile)
		return c.Next()
	}, fw.New(a.serveLive, fw.Config{HandshakeTimeout: 10 * time.Second, ReadBufferSize: 8192, WriteBufferSize: 8192}))
}
func (a *App) serveLive(client *fw.Conn) {
	uid := client.Locals("live_uid").(string)
	sid := client.Locals("live_sid").(string)
	defer client.Close()
	defer a.invalidateUser(uid)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = a.DB.Exec(ctx, "UPDATE learning_sessions SET state=state||'{\"live_active\":false}'::jsonb,updated_at=now() WHERE id=$1", sid)
	}()
	var p map[string]any
	json.Unmarshal(client.Locals("live_profile").([]byte), &p)
	var used int
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e := a.DB.QueryRow(ctx, `SELECT coalesce(sum(live_seconds),0) FROM usage WHERE user_id=$1 AND created_at>=date_trunc('day',now() AT TIME ZONE 'Asia/Bangkok') AT TIME ZONE 'Asia/Bangkok'`, uid).Scan(&used)
	if e != nil {
		return
	}
	limit := int(number(p["live_minutes"], float64(a.Cfg.LiveMinutes)))*60 - used
	limit = min(limit, 600)
	if limit <= 0 {
		client.WriteJSON(map[string]any{"error": "ใช้เวลา Live วันนี้ครบแล้ว เลือกฝึกทีละเทิร์นต่อได้"})
		return
	}
	usageID, e := a.reserve(ctx, uid, sid, "live", float64(limit)/60*2+1)
	if e != nil {
		client.WriteJSON(map[string]any{"error": e.Error()})
		return
	}
	started := time.Now()
	r := gemini.Result{Model: a.Cfg.Models["live"].ID}
	var resultMu sync.Mutex
	var clientMu sync.Mutex
	var upstreamMu sync.Mutex
	var audioMu sync.Mutex
	var inputPCM, outputPCM []byte
	lastSpeech := atomic.Int64{}
	lastSpeech.Store(time.Now().Unix())
	var callErr error
	defer func() {
		resultMu.Lock()
		defer resultMu.Unlock()
		seconds := int(time.Since(started).Seconds())
		if r.Usage.Input == 0 && r.Usage.Output == 0 {
			callErr = fmt.Errorf("usage unavailable")
		}
		a.settle(usageID, "live", r, callErr, seconds)
	}()
	upstream, _, e := gw.DefaultDialer.DialContext(ctx, "wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent", http.Header{"x-goog-api-key": []string{a.Cfg.GeminiKey}})
	if e != nil {
		callErr = e
		client.WriteJSON(map[string]any{"error": "เชื่อมต่อ Live ไม่สำเร็จ ลองโหมดทีละเทิร์น"})
		return
	}
	defer upstream.Close()
	upstream.SetReadLimit(4 << 20)
	client.SetReadLimit(64 << 10)
	var contextBytes []byte
	_ = a.DB.QueryRow(ctx, `SELECT jsonb_build_object('scenario',(SELECT data FROM scenarios WHERE id=s.scenario_id),'lesson',(SELECT data FROM lessons WHERE id=s.lesson_id),'recent_turns',(SELECT jsonb_agg(to_jsonb(t) ORDER BY created_at) FROM(SELECT role,text,created_at FROM turns WHERE session_id=s.id ORDER BY created_at DESC LIMIT 12)t)) FROM learning_sessions s WHERE id=$1`, sid).Scan(&contextBytes)
	voice := textValue(p["voice"])
	if voice == "" {
		voice = a.Cfg.Voice
	}
	setup := map[string]any{"setup": map[string]any{"model": "models/" + a.Cfg.Models["live"].ID, "generationConfig": map[string]any{"responseModalities": []string{"AUDIO"}, "maxOutputTokens": a.Cfg.Models["live"].MaxTokens, "speechConfig": map[string]any{"voiceConfig": map[string]any{"prebuiltVoiceConfig": map[string]any{"voiceName": voice}}}}, "systemInstruction": map[string]any{"parts": []any{map[string]any{"text": learning.SystemPrompt + "\nLIVE OVERRIDE: speak naturally, no JSON. One short response then let learner speak. Save grammar coaching for post-session review unless explicitly asked. Follow scenario roles one at a time, introduce who speaks. Do not claim completed learning goals. Resume naturally using recent turns. Learner: " + string(asJSON(p)) + "\nContext: " + string(contextBytes)}}}, "inputAudioTranscription": map[string]any{}, "outputAudioTranscription": map[string]any{}, "contextWindowCompression": map[string]any{"slidingWindow": map[string]any{}}}}
	if e = upstream.WriteJSON(setup); e != nil {
		callErr = e
		return
	}
	sendClient := func(v any) error {
		clientMu.Lock()
		defer clientMu.Unlock()
		client.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return client.WriteJSON(v)
	}
	sendUp := func(v any) error {
		upstreamMu.Lock()
		defer upstreamMu.Unlock()
		upstream.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return upstream.WriteJSON(v)
	}
	done := make(chan struct{}, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer func() { done <- struct{}{} }()
		var inText, outText string
		flush := func() {
			defer a.invalidateUser(uid)
			audioMu.Lock()
			in, out := inputPCM, outputPCM
			inputPCM = nil
			outputPCM = nil
			audioMu.Unlock()
			for _, t := range []struct {
				role, text string
				pcm        []byte
				rate       int
			}{{"user", inText, in, 16000}, {"model", outText, out, 24000}} {
				if t.text == "" {
					continue
				}
				tx, e := a.DB.Begin(ctx)
				if e != nil {
					continue
				}
				var aid any
				if len(t.pcm) > 0 {
					if id, e := a.storeAudio(ctx, tx, uid, "", gemini.WAV(t.pcm, t.rate), "audio/wav", true); e == nil {
						aid = id
					}
				}
				_, e = tx.Exec(ctx, "INSERT INTO turns(id,session_id,role,text,audio_id) VALUES($1,$2,$3,$4,$5)", uuid.NewString(), sid, t.role, t.text, aid)
				if e == nil && t.role == "user" && len(t.pcm) > 0 {
					_, e = tx.Exec(ctx, "INSERT INTO speech_events(id,user_id,source,duration_seconds) VALUES($1,$2,'live',$3)", uuid.NewString(), uid, speechSeconds(t.pcm, t.rate))
				}
				if e == nil {
					tx.Commit(ctx)
				} else {
					tx.Rollback(ctx)
				}
			}
			inText = ""
			outText = ""
		}
		defer flush()
		for {
			_, b, e := upstream.ReadMessage()
			if e != nil {
				return
			}
			var event map[string]json.RawMessage
			if json.Unmarshal(b, &event) != nil {
				continue
			}
			if _, ok := event["setupComplete"]; ok {
				sendClient(map[string]any{"ready": true, "seconds_remaining": limit})
				sendUp(map[string]any{"realtimeInput": map[string]any{"text": "Start or resume this practice conversation with one short question."}})
			}
			if v, ok := event["usageMetadata"]; ok {
				var u gemini.Usage
				if json.Unmarshal(v, &u) == nil {
					resultMu.Lock()
					r.Usage.Add(u)
					cost := r.Usage.Cost(a.Cfg.Models["live"], a.Cfg.USDTHB)
					resultMu.Unlock()
					if cost >= float64(limit)/60*2 {
						sendClient(map[string]any{"error": "พัก Live เพื่อรักษาวงเงิน"})
						return
					}
				}
			}
			if v, ok := event["serverContent"]; ok {
				var sc struct {
					Input struct {
						Text string `json:"text"`
					} `json:"inputTranscription"`
					Output struct {
						Text string `json:"text"`
					} `json:"outputTranscription"`
					TurnComplete bool `json:"turnComplete"`
					Interrupted  bool `json:"interrupted"`
					Model        struct {
						Parts []struct {
							Inline struct {
								Data string `json:"data"`
								MIME string `json:"mimeType"`
							} `json:"inlineData"`
						} `json:"parts"`
					} `json:"modelTurn"`
				}
				if json.Unmarshal(v, &sc) == nil {
					inText += sc.Input.Text
					outText += sc.Output.Text
					audioMu.Lock()
					for _, part := range sc.Model.Parts {
						if pcm, e := base64.StdEncoding.DecodeString(part.Inline.Data); e == nil && len(outputPCM) < 32<<20 {
							outputPCM = append(outputPCM, pcm...)
						}
					}
					audioMu.Unlock()
					sendClient(map[string]any{"serverContent": json.RawMessage(v)})
					if sc.TurnComplete || sc.Interrupted {
						flush()
					}
				}
			}
			if _, ok := event["goAway"]; ok {
				sendClient(map[string]any{"reconnect": true})
				return
			}
			if _, ok := event["error"]; ok {
				sendClient(map[string]any{"error": "Live ขัดข้อง กรุณาเชื่อมต่อใหม่"})
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		defer func() { done <- struct{}{} }()
		totalBytes := 0
		for {
			typ, b, e := client.ReadMessage()
			if e != nil {
				return
			}
			if typ == fw.BinaryMessage {
				if len(b)%2 != 0 || len(b) > 32000 {
					return
				}
				totalBytes += len(b)
				if float64(totalBytes)/32000 > time.Since(started).Seconds()+3 {
					return
				}
				energy := float64(0)
				for i := 0; i+1 < len(b); i += 2 {
					x := float64(int16(binary.LittleEndian.Uint16(b[i:])))
					energy += x * x
				}
				if len(b) > 0 && energy/float64(len(b)/2) > 160000 {
					lastSpeech.Store(time.Now().Unix())
				}
				audioMu.Lock()
				if len(inputPCM) < 8<<20 {
					inputPCM = append(inputPCM, b...)
				}
				audioMu.Unlock()
				if e = sendUp(map[string]any{"realtimeInput": map[string]any{"audio": map[string]any{"mimeType": "audio/pcm;rate=16000", "data": base64.StdEncoding.EncodeToString(b)}}}); e != nil {
					return
				}
			} else {
				var v struct {
					Type string `json:"type"`
				}
				if json.Unmarshal(b, &v) != nil {
					return
				}
				if v.Type == "stop" {
					return
				}
				if v.Type == "ping" {
					_, _ = a.DB.Exec(ctx, "UPDATE learning_sessions SET updated_at=now() WHERE id=$1", sid)
					sendClient(map[string]any{"pong": true})
				}
			}
		}
	}()
	timer := time.NewTimer(time.Duration(limit) * time.Second)
	defer timer.Stop()
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	running := true
	for running {
		select {
		case <-done:
			running = false
		case <-timer.C:
			sendClient(map[string]any{"ended": "ครบเวลารอบ Live แล้ว"})
			running = false
		case <-tick.C:
			if time.Now().Unix()-lastSpeech.Load() > 60 {
				sendClient(map[string]any{"ended": "พัก Live หลังเงียบ 60 วินาที"})
				running = false
			}
		}
	}
	upstream.Close()
	client.Close()
	wg.Wait()
}

// Count audible 20ms frames, excluding listening/silence from speaking progress.
func speechSeconds(pcm []byte, rate int) float64 {
	size := rate / 50 * 2
	voiced := 0
	for start := 0; start+size <= len(pcm); start += size {
		var energy float64
		for i := start; i < start+size; i += 2 {
			v := float64(int16(binary.LittleEndian.Uint16(pcm[i : i+2])))
			energy += v * v
		}
		if energy/float64(size/2) > 250*250 {
			voiced++
		}
	}
	return float64(voiced) / 50
}
