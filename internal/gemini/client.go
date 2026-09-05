package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"tokoloop/internal/config"
)

type Client struct {
	Key            string
	HTTP           *http.Client
	BaseURL        string
	Models         map[string]config.Model
	DefaultTimeout int
}
type Usage struct {
	Input         int      `json:"promptTokenCount"`
	Output        int      `json:"candidatesTokenCount"`
	Thinking      int      `json:"thoughtsTokenCount"`
	InputDetails  []Detail `json:"promptTokensDetails"`
	OutputDetails []Detail `json:"candidatesTokensDetails"`
}
type Detail struct {
	Modality string `json:"modality"`
	Count    int    `json:"tokenCount"`
}

// Live reports per-response usage using responseTokenCount rather than candidatesTokenCount.
func (u *Usage) UnmarshalJSON(b []byte) error {
	type plain Usage
	var v struct {
		plain
		Response        int      `json:"responseTokenCount"`
		ResponseDetails []Detail `json:"responseTokensDetails"`
	}
	if e := json.Unmarshal(b, &v); e != nil {
		return e
	}
	*u = Usage(v.plain)
	if u.Output == 0 {
		u.Output = v.Response
	}
	if len(u.OutputDetails) == 0 {
		u.OutputDetails = v.ResponseDetails
	}
	return nil
}
func (u *Usage) Add(v Usage) {
	u.Input += v.Input
	u.Output += v.Output
	u.Thinking += v.Thinking
	u.InputDetails = append(u.InputDetails, v.InputDetails...)
	u.OutputDetails = append(u.OutputDetails, v.OutputDetails...)
}
func (u Usage) AudioInput() int {
	total := 0
	for _, v := range u.InputDetails {
		if v.Modality == "AUDIO" {
			total += v.Count
		}
	}
	return total
}
func (u Usage) AudioOutput() int {
	total := 0
	for _, v := range u.OutputDetails {
		if v.Modality == "AUDIO" {
			total += v.Count
		}
	}
	return total
}
func (u Usage) Cost(m config.Model, fx float64) float64 {
	ai, ao := u.AudioInput(), u.AudioOutput()
	return (float64(max(0, u.Input-ai))*m.Input + float64(ai)*m.AudioInput + float64(max(0, u.Output-ao)+u.Thinking)*m.Output + float64(ao)*m.AudioOutput) * fx / 1e6
}

type Result struct {
	Text           string
	Audio          []byte
	MIME           string
	Usage          Usage
	Latency        int
	Model          string
	Rejected       bool
	EstimatedUsage Usage
}

func New(c config.Config) *Client {
	return &Client{Models: c.Models, Key: c.GeminiKey, DefaultTimeout: c.TimeoutSeconds, HTTP: &http.Client{}, BaseURL: "https://generativelanguage.googleapis.com/v1beta/models/"}
}
func (c *Client) Generate(ctx context.Context, m config.Model, system, prompt string, audio []byte, mime string, schema any, voice string) (Result, error) {
	r := Result{Model: m.ID}
	timeout := m.TimeoutSeconds
	if timeout < 1 {
		timeout = c.DefaultTimeout
	}
	if timeout < 1 {
		timeout = 45
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	if voice != "" {
		r.EstimatedUsage = Usage{Input: max(1, len(prompt)/3), OutputDetails: []Detail{{Modality: "AUDIO", Count: min(m.MaxTokens, max(50, len(strings.Fields(prompt))*13))}}}
		r.EstimatedUsage.Output = r.EstimatedUsage.AudioOutput()
	}

	if c.Key == "" {
		return r, fmt.Errorf("ยังไม่ได้ตั้งค่า Gemini API key")
	}
	parts := []any{map[string]any{"text": prompt}}
	if len(audio) > 0 {
		parts = append(parts, map[string]any{"inlineData": map[string]any{"mimeType": mime, "data": base64.StdEncoding.EncodeToString(audio)}})
	}
	gen := map[string]any{"maxOutputTokens": m.MaxTokens, "temperature": m.Temperature}
	if schema != nil {
		gen["responseMimeType"] = "application/json"
		gen["responseJsonSchema"] = schema
	}
	if strings.Contains(m.ID, "2.5") && !strings.Contains(m.ID, "tts") {
		gen["thinkingConfig"] = map[string]any{"thinkingBudget": m.ThinkingBudget}
	}
	if m.ThinkingLevel != "" && strings.Contains(m.ID, "gemini-3") {
		gen["thinkingConfig"] = map[string]any{"thinkingLevel": m.ThinkingLevel}
	}
	body := map[string]any{"contents": []any{map[string]any{"role": "user", "parts": parts}}, "generationConfig": gen}
	if voice != "" {
		gen["responseModalities"] = []string{"AUDIO"}
		gen["speechConfig"] = map[string]any{"voiceConfig": map[string]any{"prebuiltVoiceConfig": map[string]any{"voiceName": voice}}}
	} else {
		body["systemInstruction"] = map[string]any{"parts": []any{map[string]any{"text": system}}}
	}
	b, _ := json.Marshal(body)
	start := time.Now()
	req, e := http.NewRequestWithContext(ctx, "POST", c.BaseURL+url.PathEscape(m.ID)+":generateContent", bytes.NewReader(b))
	if e != nil {
		return r, e
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.Key)
	resp, e := c.HTTP.Do(req)
	r.Latency = int(time.Since(start).Milliseconds())
	if e != nil {
		return r, fmt.Errorf("Gemini connection interrupted; please retry")
	}
	defer resp.Body.Close()
	data, e := io.ReadAll(io.LimitReader(resp.Body, 24<<20))
	if e != nil {
		return r, e
	}
	if resp.StatusCode != 200 {
		r.Rejected = resp.StatusCode == 400 || resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 || resp.StatusCode == 429
		if (resp.StatusCode == 404 || resp.StatusCode == 400) && m.Fallback != "" {
			for _, fallback := range c.Models {
				if fallback.ID == m.Fallback {
					fallback.Fallback = ""
					return c.Generate(ctx, fallback, system, prompt, audio, mime, schema, voice)
				}
			}
		}
		return r, fmt.Errorf("Gemini unavailable (HTTP %d); no learning result saved", resp.StatusCode)
	}
	var raw struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text   string `json:"text"`
					Inline struct {
						Data string `json:"data"`
						MIME string `json:"mimeType"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		Usage Usage `json:"usageMetadata"`
	}
	if e = json.Unmarshal(data, &raw); e != nil {
		return r, fmt.Errorf("invalid Gemini response")
	}
	r.Usage = raw.Usage
	if len(raw.Candidates) == 0 {
		return r, fmt.Errorf("Gemini returned no response")
	}
	if raw.Candidates[0].FinishReason != "STOP" && raw.Candidates[0].FinishReason != "" {
		return r, fmt.Errorf("Gemini response incomplete; please retry")
	}
	for _, p := range raw.Candidates[0].Content.Parts {
		r.Text += p.Text
		if p.Inline.Data != "" {
			r.Audio, e = base64.StdEncoding.DecodeString(p.Inline.Data)
			r.MIME = p.Inline.MIME
			if e != nil {
				return r, e
			}
		}
	}
	if voice != "" && len(r.Audio) == 0 {
		return r, fmt.Errorf("Gemini returned no audio")
	}
	return r, nil
}
func WAV(pcm []byte, rate int) []byte {
	b := make([]byte, 44+len(pcm))
	copy(b, "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(36+len(pcm)))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], uint32(rate))
	binary.LittleEndian.PutUint32(b[28:], uint32(rate*2))
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(len(pcm)))
	copy(b[44:], pcm)
	return b
}
