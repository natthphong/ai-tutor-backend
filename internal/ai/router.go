package ai

import (
	"context"
	"fmt"

	"gitlab.com/home-server7795544/home-server/iam/iam-backend/config"
	"go.uber.org/zap"
)

// Router manages AI provider fallback chain
type Router struct {
	primaryLLM    LLMProvider
	fallbackLLMs  []LLMProvider
	primaryTTS    TTSProvider
	fallbackTTS   TTSProvider
	primarySTT    STTProvider
	fallbackSTT   STTProvider
	cfg           config.Config
	logger        *zap.Logger
}

// NewRouter creates a new AI provider router with fallback chain
func NewRouter(cfg *config.Config) *Router {
	logger := zap.L()
	r := &Router{
		cfg:    *cfg,
		logger: logger,
	}

	// Setup LLM providers
	if cfg.Gemini.APIKey != "" {
		r.primaryLLM = NewGeminiProvider(cfg.Gemini)
		logger.Info("AI Router: Gemini LLM registered as primary")
	}

	if cfg.OpenRouter.APIKey != "" {
		orProvider := NewOpenRouterProvider(cfg.OpenRouter)
		r.fallbackLLMs = append(r.fallbackLLMs, orProvider)
		logger.Info("AI Router: OpenRouter registered as fallback LLM")

		if r.primaryLLM == nil {
			r.primaryLLM = orProvider
			logger.Info("AI Router: OpenRouter promoted to primary LLM (no Gemini key)")
		}
	}

	if cfg.OpenAI.APIKey != "" {
		oaiProvider := NewOpenAIProvider(cfg.OpenAI)
		r.fallbackLLMs = append(r.fallbackLLMs, oaiProvider)
		logger.Info("AI Router: OpenAI registered as fallback LLM")

		if r.primaryLLM == nil {
			r.primaryLLM = oaiProvider
			logger.Info("AI Router: OpenAI promoted to primary LLM")
		}
	}

	// Setup TTS providers
	if cfg.Gemini.APIKey != "" {
		r.primaryTTS = NewGeminiTTS(cfg.Gemini)
		logger.Info("AI Router: Gemini TTS registered as primary")
	}
	if cfg.OpenAI.APIKey != "" {
		oaiTTS := NewOpenAITTS(cfg.OpenAI)
		r.fallbackTTS = oaiTTS
		logger.Info("AI Router: OpenAI TTS registered as fallback")

		if r.primaryTTS == nil {
			r.primaryTTS = oaiTTS
		}
	}

	// Setup STT providers
	if cfg.Gemini.APIKey != "" {
		r.primarySTT = NewGeminiSTT(cfg.Gemini)
		logger.Info("AI Router: Gemini STT registered as primary")
	}
	if cfg.OpenAI.APIKey != "" {
		oaiSTT := NewOpenAISTT(cfg.OpenAI)
		r.fallbackSTT = oaiSTT
		logger.Info("AI Router: OpenAI STT registered as fallback")

		if r.primarySTT == nil {
			r.primarySTT = oaiSTT
		}
	}

	return r
}

// Chat sends a chat request with fallback
func (r *Router) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if r.primaryLLM == nil {
		return nil, fmt.Errorf("no LLM provider configured")
	}

	// Try primary
	resp, err := r.primaryLLM.Chat(ctx, req)
	if err == nil {
		return resp, nil
	}
	r.logger.Warn("Primary LLM failed, trying fallback",
		zap.String("provider", r.primaryLLM.Name()),
		zap.Error(err))

	// Try fallbacks
	for i, fb := range r.fallbackLLMs {
		// For OpenRouter, try each fallback model
		if fb.Name() == "openrouter" && len(r.cfg.LLM.FallbackModels) > 0 {
			for _, model := range r.cfg.LLM.FallbackModels {
				req.Model = model
				resp, err = fb.Chat(ctx, req)
				if err == nil {
					return resp, nil
				}
				r.logger.Warn("OpenRouter fallback model failed",
					zap.String("model", model),
					zap.Error(err))
			}
			continue
		}

		resp, err = fb.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}
		r.logger.Warn("Fallback LLM failed",
			zap.Int("index", i),
			zap.String("provider", fb.Name()),
			zap.Error(err))
	}

	return nil, fmt.Errorf("all LLM providers failed: %w", err)
}

// ChatWithModel sends a chat request using a specific model
func (r *Router) ChatWithModel(ctx context.Context, req ChatRequest, model string) (*ChatResponse, error) {
	req.Model = model
	return r.Chat(ctx, req)
}

// DeepChat uses the deep/expensive model for complex analysis
func (r *Router) DeepChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	req.Model = r.cfg.LLM.DeepModel
	return r.Chat(ctx, req)
}

// Synthesize generates TTS audio with fallback
func (r *Router) Synthesize(ctx context.Context, req TTSRequest) ([]byte, string, error) {
	if r.primaryTTS == nil {
		return nil, "", fmt.Errorf("no TTS provider configured")
	}

	data, err := r.primaryTTS.Synthesize(ctx, req)
	if err == nil {
		return data, r.primaryTTS.Name(), nil
	}
	r.logger.Warn("Primary TTS failed, trying fallback",
		zap.String("provider", r.primaryTTS.Name()),
		zap.Error(err))

	if r.fallbackTTS != nil {
		data, err = r.fallbackTTS.Synthesize(ctx, req)
		if err == nil {
			return data, r.fallbackTTS.Name(), nil
		}
		r.logger.Error("Fallback TTS also failed",
			zap.String("provider", r.fallbackTTS.Name()),
			zap.Error(err))
	}

	return nil, "", fmt.Errorf("all TTS providers failed: %w", err)
}

// Transcribe converts audio to text with fallback
func (r *Router) Transcribe(ctx context.Context, req STTRequest) (*STTResponse, error) {
	if r.primarySTT == nil {
		return nil, fmt.Errorf("no STT provider configured")
	}

	resp, err := r.primarySTT.Transcribe(ctx, req)
	if err == nil {
		return resp, nil
	}
	r.logger.Warn("Primary STT failed, trying fallback",
		zap.String("provider", r.primarySTT.Name()),
		zap.Error(err))

	if r.fallbackSTT != nil {
		resp, err = r.fallbackSTT.Transcribe(ctx, req)
		if err == nil {
			return resp, nil
		}
		r.logger.Error("Fallback STT also failed",
			zap.String("provider", r.fallbackSTT.Name()),
			zap.Error(err))
	}

	return nil, fmt.Errorf("all STT providers failed: %w", err)
}
