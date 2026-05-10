package ai

import (
	"context"
)

// LLMProvider is the interface for language model providers
type LLMProvider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest) (<-chan string, error)
	Name() string
}

// TTSProvider is the interface for text-to-speech providers
type TTSProvider interface {
	Synthesize(ctx context.Context, req TTSRequest) ([]byte, error)
	Name() string
}

// STTProvider is the interface for speech-to-text providers
type STTProvider interface {
	Transcribe(ctx context.Context, req STTRequest) (*STTResponse, error)
	Name() string
}

// ChatRequest represents a request to an LLM
type ChatRequest struct {
	SystemPrompt string         `json:"systemPrompt"`
	Messages     []ChatMessage  `json:"messages"`
	Model        string         `json:"model,omitempty"`
	Temperature  float64        `json:"temperature,omitempty"`
	MaxTokens    int            `json:"maxTokens,omitempty"`
	UseCase      string         `json:"useCase,omitempty"`
	UserID       string         `json:"userId,omitempty"`
	SessionID    string         `json:"sessionId,omitempty"`
}

// ChatMessage represents a single message in a conversation
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse represents a response from an LLM
type ChatResponse struct {
	Content      string `json:"content"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
	LatencyMs    int64  `json:"latencyMs"`
}

// TTSRequest represents a text-to-speech request
type TTSRequest struct {
	Text       string `json:"text"`
	VoiceStyle string `json:"voiceStyle,omitempty"`
	Speed      float32 `json:"speed,omitempty"`
	UserID     string `json:"userId,omitempty"`
	SessionID  string `json:"sessionId,omitempty"`
}

// STTRequest represents a speech-to-text request
type STTRequest struct {
	AudioData []byte `json:"audioData"`
	Filename  string `json:"filename"`
	MediaType string `json:"mediaType"`
	UserID    string `json:"userId,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
}

// STTResponse represents a speech-to-text response
type STTResponse struct {
	Text       string  `json:"text"`
	Provider   string  `json:"provider"`
	Model      string  `json:"model"`
	Confidence float64 `json:"confidence"`
	DurationMs int64   `json:"durationMs"`
}
