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

// OpenRouterProvider implements LLMProvider using OpenRouter API
type OpenRouterProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewOpenRouterProvider(cfg config.OpenRouterConfig) *OpenRouterProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	return &OpenRouterProvider{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (o *OpenRouterProvider) Name() string { return "openrouter" }

func (o *OpenRouterProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	start := time.Now()
	model := req.Model
	if model == "" {
		model = "google/gemini-2.5-flash"
	}

	messages := make([]map[string]string, 0)
	if req.SystemPrompt != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": req.SystemPrompt,
		})
	}
	for _, msg := range req.Messages {
		messages = append(messages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	body := map[string]interface{}{
		"model":    model,
		"messages": messages,
	}
	if req.Temperature > 0 {
		body["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openrouter: marshal error: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("openrouter: request error: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openrouter: http error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openrouter: read error: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("openrouter: status %d: %s", resp.StatusCode, string(respBody))
	}

	var orResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &orResp); err != nil {
		return nil, fmt.Errorf("openrouter: unmarshal error: %w", err)
	}

	if len(orResp.Choices) == 0 {
		return nil, fmt.Errorf("openrouter: no choices")
	}

	return &ChatResponse{
		Content:      orResp.Choices[0].Message.Content,
		Provider:     "openrouter",
		Model:        model,
		InputTokens:  orResp.Usage.PromptTokens,
		OutputTokens: orResp.Usage.CompletionTokens,
		LatencyMs:    time.Since(start).Milliseconds(),
	}, nil
}

func (o *OpenRouterProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan string, error) {
	return nil, fmt.Errorf("openrouter: streaming not implemented")
}

// OpenAIProvider implements LLM, TTS, STT using OpenAI API
type OpenAIProvider struct {
	apiKey string
	client *http.Client
}

func NewOpenAIProvider(cfg config.OpenAIConfig) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey: cfg.APIKey,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (o *OpenAIProvider) Name() string { return "openai" }

func (o *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	start := time.Now()
	model := req.Model
	if model == "" {
		model = "gpt-4o-mini"
	}

	messages := make([]map[string]string, 0)
	if req.SystemPrompt != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": req.SystemPrompt,
		})
	}
	for _, msg := range req.Messages {
		messages = append(messages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	body := map[string]interface{}{
		"model":    model,
		"messages": messages,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal error: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("openai: request error: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: http error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: read error: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("openai: status %d: %s", resp.StatusCode, string(respBody))
	}

	var oaiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		return nil, fmt.Errorf("openai: unmarshal error: %w", err)
	}

	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("openai: no choices")
	}

	return &ChatResponse{
		Content:      oaiResp.Choices[0].Message.Content,
		Provider:     "openai",
		Model:        model,
		InputTokens:  oaiResp.Usage.PromptTokens,
		OutputTokens: oaiResp.Usage.CompletionTokens,
		LatencyMs:    time.Since(start).Milliseconds(),
	}, nil
}

func (o *OpenAIProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan string, error) {
	return nil, fmt.Errorf("openai: streaming not implemented")
}

// OpenAITTS implements TTSProvider
type OpenAITTS struct {
	apiKey string
	model  string
	client *http.Client
}

func NewOpenAITTS(cfg config.OpenAIConfig) *OpenAITTS {
	model := cfg.TTSModel
	if model == "" {
		model = "gpt-4o-mini-tts"
	}
	return &OpenAITTS{
		apiKey: cfg.APIKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *OpenAITTS) Name() string { return "openai" }

func (t *OpenAITTS) Synthesize(ctx context.Context, req TTSRequest) ([]byte, error) {
	body := map[string]interface{}{
		"model": t.model,
		"input": req.Text,
		"voice": "alloy",
	}
	if req.Speed > 0 {
		body["speed"] = req.Speed
	}
	if req.VoiceStyle != "" {
		body["instructions"] = req.VoiceStyle
	}

	jsonBody, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/audio/speech", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("openai tts: request error: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai tts: http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai tts: status %d: %s", resp.StatusCode, string(respBody))
	}

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai tts: read error: %w", err)
	}

	return audioData, nil
}

// OpenAISTT implements STTProvider using Whisper
type OpenAISTT struct {
	apiKey string
	model  string
	client *http.Client
}

func NewOpenAISTT(cfg config.OpenAIConfig) *OpenAISTT {
	model := cfg.STTModel
	if model == "" {
		model = "whisper-1"
	}
	return &OpenAISTT{
		apiKey: cfg.APIKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *OpenAISTT) Name() string { return "openai" }

func (s *OpenAISTT) Transcribe(ctx context.Context, req STTRequest) (*STTResponse, error) {
	start := time.Now()

	// Create multipart form
	var buf bytes.Buffer
	writer := NewMultipartWriter(&buf)
	writer.WriteField("model", s.model)
	writer.WriteField("language", "en")

	filename := req.Filename
	if filename == "" {
		filename = "audio.webm"
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("openai stt: form error: %w", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(req.AudioData)); err != nil {
		return nil, fmt.Errorf("openai stt: copy error: %w", err)
	}
	writer.Close()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/audio/transcriptions", &buf)
	if err != nil {
		return nil, fmt.Errorf("openai stt: request error: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai stt: http error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai stt: read error: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("openai stt: status %d: %s", resp.StatusCode, string(respBody))
	}

	var sttResp struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &sttResp); err != nil {
		return nil, fmt.Errorf("openai stt: unmarshal error: %w", err)
	}

	return &STTResponse{
		Text:       sttResp.Text,
		Provider:   "openai",
		Model:      s.model,
		Confidence: 0.9,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}
