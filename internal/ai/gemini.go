package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"gitlab.com/home-server7795544/home-server/iam/iam-backend/config"
)

// GeminiProvider implements LLM, TTS, and STT using Google Gemini API
type GeminiProvider struct {
	apiKey string
	client *http.Client
}

func NewGeminiProvider(cfg config.GeminiConfig) *GeminiProvider {
	return &GeminiProvider{
		apiKey: cfg.APIKey,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (g *GeminiProvider) Name() string { return "gemini" }

// Chat sends a chat request to Gemini
func (g *GeminiProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	start := time.Now()

	model := req.Model
	if model == "" {
		model = "gemini-2.5-flash"
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, g.apiKey)

	// Build contents
	var contents []map[string]interface{}

	// Add system instruction if present
	systemInstruction := map[string]interface{}{}
	if req.SystemPrompt != "" {
		systemInstruction = map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": req.SystemPrompt},
			},
		}
	}

	// Add messages
	for _, msg := range req.Messages {
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]interface{}{
			"role": role,
			"parts": []map[string]interface{}{
				{"text": msg.Content},
			},
		})
	}

	body := map[string]interface{}{
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature": 0.7,
		},
	}

	if len(systemInstruction) > 0 {
		body["systemInstruction"] = systemInstruction
	}

	if req.Temperature > 0 {
		body["generationConfig"].(map[string]interface{})["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		body["generationConfig"].(map[string]interface{})["maxOutputTokens"] = req.MaxTokens
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal error: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("gemini: request error: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: http error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini: read error: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gemini: status %d: %s", resp.StatusCode, string(respBody))
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("gemini: unmarshal error: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini: no content in response")
	}

	content := ""
	for _, part := range geminiResp.Candidates[0].Content.Parts {
		content += part.Text
	}

	return &ChatResponse{
		Content:      content,
		Provider:     "gemini",
		Model:        model,
		InputTokens:  geminiResp.UsageMetadata.PromptTokenCount,
		OutputTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
		LatencyMs:    time.Since(start).Milliseconds(),
	}, nil
}

// ChatStream is not yet implemented for Gemini
func (g *GeminiProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan string, error) {
	return nil, fmt.Errorf("gemini: streaming not implemented")
}

// GeminiTTS implements TTSProvider using Gemini
type GeminiTTS struct {
	apiKey string
	client *http.Client
}

func NewGeminiTTS(cfg config.GeminiConfig) *GeminiTTS {
	return &GeminiTTS{
		apiKey: cfg.APIKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *GeminiTTS) Name() string { return "gemini" }

func (t *GeminiTTS) Synthesize(ctx context.Context, req TTSRequest) ([]byte, error) {
	model := "gemini-2.5-flash"
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, t.apiKey)

	voiceInstruction := "Speak clearly and naturally as a friendly English teacher."
	if req.VoiceStyle != "" {
		voiceInstruction = req.VoiceStyle
	}

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{"text": fmt.Sprintf("Please read the following text aloud clearly and naturally: %s", req.Text)},
				},
			},
		},
		"systemInstruction": map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": voiceInstruction},
			},
		},
		"generationConfig": map[string]interface{}{
			"responseModalities": []string{"AUDIO"},
			"speechConfig": map[string]interface{}{
				"voiceConfig": map[string]interface{}{
					"prebuiltVoiceConfig": map[string]interface{}{
						"voiceName": "Kore",
					},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("gemini tts: marshal error: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("gemini tts: request error: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini tts: http error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini tts: read error: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gemini tts: status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse the audio data from Gemini response
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					InlineData struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("gemini tts: unmarshal error: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini tts: no audio in response")
	}

	// Decode base64 audio
	b64Data := geminiResp.Candidates[0].Content.Parts[0].InlineData.Data
	audioData, err := decodeBase64(b64Data)
	if err != nil {
		return nil, fmt.Errorf("gemini tts: decode error: %w", err)
	}

	return audioData, nil
}

// GeminiSTT implements STTProvider using Gemini
type GeminiSTT struct {
	apiKey string
	client *http.Client
}

func NewGeminiSTT(cfg config.GeminiConfig) *GeminiSTT {
	return &GeminiSTT{
		apiKey: cfg.APIKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *GeminiSTT) Name() string { return "gemini" }

func (s *GeminiSTT) Transcribe(ctx context.Context, req STTRequest) (*STTResponse, error) {
	start := time.Now()
	model := "gemini-2.5-flash"
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, s.apiKey)

	// Encode audio to base64
	audioB64 := encodeBase64(req.AudioData)

	mimeType := req.MediaType
	if mimeType == "" {
		mimeType = "audio/webm"
	}

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{
						"inlineData": map[string]interface{}{
							"mimeType": mimeType,
							"data":     audioB64,
						},
					},
					{"text": "Please transcribe this audio exactly. Return only the transcribed text, nothing else."},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature": 0.1,
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("gemini stt: marshal error: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("gemini stt: request error: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini stt: http error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini stt: read error: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gemini stt: status %d: %s", resp.StatusCode, string(respBody))
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("gemini stt: unmarshal error: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("gemini stt: no transcription")
	}

	text := ""
	for _, part := range geminiResp.Candidates[0].Content.Parts {
		text += part.Text
	}

	return &STTResponse{
		Text:       text,
		Provider:   "gemini",
		Model:      model,
		Confidence: 0.85,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}
