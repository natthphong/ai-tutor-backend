package handler

import (
	"io"

	"github.com/gofiber/fiber/v2"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/api"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/config"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/internal/ai"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/internal/tutor"
	"go.uber.org/zap"
)

type TutorHandler struct {
	svc    *tutor.Service
	ingest *tutor.IngestService
	router *ai.Router
	cfg    *config.Config
	logger *zap.Logger
}

func NewTutorHandler(svc *tutor.Service, ingest *tutor.IngestService, router *ai.Router, cfg *config.Config) *TutorHandler {
	return &TutorHandler{svc: svc, ingest: ingest, router: router, cfg: cfg, logger: zap.L()}
}

func (h *TutorHandler) Register(group fiber.Router) {
	t := group.Group("/tutor")
	t.Post("/sessions/start", h.StartSession())
	t.Post("/sessions/:sessionId/next", h.GetNextStep())
	t.Post("/sessions/:sessionId/listening/answer", h.SubmitListeningAnswer())
	t.Post("/sessions/:sessionId/speaking/audio", h.SubmitSpeakingAudio())
	t.Post("/sessions/:sessionId/reading/answer", h.SubmitReadingAnswer())
	t.Get("/due", h.GetDueReviews())
	t.Post("/reviews/flashcards/:id/answer", h.ReviewFlashcard())

	group.Post("/voice/tts", h.TTS())
	group.Post("/voice/stt", h.STT())
	group.Get("/progress", h.GetProgress())
	group.Post("/admin/lessons/ingest", h.IngestLessons())
}

func (h *TutorHandler) StartSession() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req struct {
			UserID        string `json:"userId"`
			PreferredMode string `json:"preferredMode"`
			DisplayName   string `json:"displayName"`
		}
		if err := c.BodyParser(&req); err != nil {
			return api.BadRequest(c, "Invalid request body")
		}
		if req.UserID == "" {
			return api.BadRequest(c, "userId is required")
		}
		userID, err := h.svc.EnsureUser(c.Context(), req.UserID, req.DisplayName)
		if err != nil {
			return api.InternalError(c, "Failed to create user: "+err.Error())
		}
		result, err := h.svc.StartSession(c.Context(), userID, req.PreferredMode)
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		return api.OkWithMessage(c, "Tutor session started", result)
	}
}

func (h *TutorHandler) GetNextStep() fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Params("sessionId")
		var req struct {
			UserID string `json:"userId"`
		}
		c.BodyParser(&req)
		if req.UserID == "" {
			req.UserID = c.Query("userId")
		}
		userID, _ := h.svc.EnsureUser(c.Context(), req.UserID, "")
		result, err := h.svc.GetNextStep(c.Context(), sessionID, userID)
		if err != nil {
			return api.SessionNotFound(c)
		}
		return api.OkWithMessage(c, "Next tutor step", result)
	}
}

func (h *TutorHandler) SubmitListeningAnswer() fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Params("sessionId")
		var req struct {
			UserID       string `json:"userId"`
			LessonItemID string `json:"lessonItemId"`
			Answer       string `json:"answer"`
		}
		if err := c.BodyParser(&req); err != nil {
			return api.BadRequest(c, "Invalid request body")
		}
		userID, _ := h.svc.EnsureUser(c.Context(), req.UserID, "")
		result, err := h.svc.EvaluateListening(c.Context(), sessionID, userID, req.LessonItemID, req.Answer)
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		return api.OkWithMessage(c, "Listening answer evaluated", result)
	}
}

func (h *TutorHandler) SubmitSpeakingAudio() fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Params("sessionId")
		userID := c.FormValue("userId")
		if userID == "" {
			return api.BadRequest(c, "userId is required")
		}

		fh, err := c.FormFile("file")
		if err != nil {
			return api.BadRequest(c, "audio file is required")
		}

		src, err := fh.Open()
		if err != nil {
			return api.InternalError(c, "Failed to open file")
		}
		defer src.Close()

		audioBytes, err := io.ReadAll(src)
		if err != nil {
			return api.InternalError(c, "Failed to read audio")
		}

		// Run STT
		sttResp, err := h.router.Transcribe(c.Context(), ai.STTRequest{
			AudioData: audioBytes, Filename: fh.Filename, MediaType: "audio/webm",
			UserID: userID, SessionID: sessionID,
		})
		if err != nil {
			return api.InternalError(c, "STT failed: "+err.Error())
		}

		uid, _ := h.svc.EnsureUser(c.Context(), userID, "")
		result, err := h.svc.EvaluateSpeaking(c.Context(), sessionID, uid, sttResp.Text)
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		result["sttProvider"] = sttResp.Provider
		result["sttConfidence"] = sttResp.Confidence
		return api.OkWithMessage(c, "Speaking evaluated", result)
	}
}

func (h *TutorHandler) SubmitReadingAnswer() fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Params("sessionId")
		var req struct {
			UserID       string `json:"userId"`
			LessonItemID string `json:"lessonItemId"`
			Translation  string `json:"translation"`
		}
		if err := c.BodyParser(&req); err != nil {
			return api.BadRequest(c, "Invalid request body")
		}
		userID, _ := h.svc.EnsureUser(c.Context(), req.UserID, "")
		result, err := h.svc.EvaluateReading(c.Context(), sessionID, userID, req.LessonItemID, req.Translation)
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		return api.OkWithMessage(c, "Reading translation evaluated", result)
	}
}

func (h *TutorHandler) GetDueReviews() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Query("userId")
		if userID == "" {
			return api.BadRequest(c, "userId is required")
		}
		uid, _ := h.svc.EnsureUser(c.Context(), userID, "")
		dueItems, err := h.svc.GetDueItems(c.Context(), uid)
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		return api.OkWithMessage(c, "Due review items", dueItems)
	}
}

func (h *TutorHandler) ReviewFlashcard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		flashcardID := c.Params("id")
		var req struct {
			UserID string  `json:"userId"`
			Answer string  `json:"answer"`
			Score  float64 `json:"score"`
		}
		if err := c.BodyParser(&req); err != nil {
			return api.BadRequest(c, "Invalid request body")
		}
		uid, _ := h.svc.EnsureUser(c.Context(), req.UserID, "")
		result, err := h.svc.ReviewFlashcard(c.Context(), uid, flashcardID, req.Score)
		if err != nil {
			return api.NotFound(c, err.Error())
		}
		return api.OkWithMessage(c, "Flashcard reviewed", result)
	}
}

func (h *TutorHandler) TTS() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req struct {
			Text       string  `json:"text"`
			VoiceStyle string  `json:"voiceStyle"`
			Speed      float32 `json:"speed"`
			UserID     string  `json:"userId"`
			SessionID  string  `json:"sessionId"`
		}
		if err := c.BodyParser(&req); err != nil {
			return api.BadRequest(c, "Invalid request body")
		}
		if req.Text == "" {
			return api.BadRequest(c, "text is required")
		}
		audioData, provider, err := h.router.Synthesize(c.Context(), ai.TTSRequest{
			Text: req.Text, VoiceStyle: req.VoiceStyle, Speed: req.Speed,
		})
		if err != nil {
			return api.InternalError(c, "TTS failed: "+err.Error())
		}
		c.Set("Content-Type", "audio/mpeg")
		c.Set("X-TTS-Provider", provider)
		return c.Send(audioData)
	}
}

func (h *TutorHandler) STT() fiber.Handler {
	return func(c *fiber.Ctx) error {
		fh, err := c.FormFile("file")
		if err != nil {
			return api.BadRequest(c, "audio file required")
		}
		src, err := fh.Open()
		if err != nil {
			return api.InternalError(c, "open file failed")
		}
		defer src.Close()
		audioBytes, _ := io.ReadAll(src)
		resp, err := h.router.Transcribe(c.Context(), ai.STTRequest{AudioData: audioBytes, Filename: fh.Filename, MediaType: "audio/webm"})
		if err != nil {
			return api.InternalError(c, "STT failed: "+err.Error())
		}
		return api.OkWithMessage(c, "STT completed", resp)
	}
}

func (h *TutorHandler) GetProgress() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Query("userId")
		if userID == "" {
			return api.BadRequest(c, "userId required")
		}
		uid, _ := h.svc.EnsureUser(c.Context(), userID, "")
		result, err := h.svc.GetProgress(c.Context(), uid)
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		return api.OkWithMessage(c, "Progress data", result)
	}
}

func (h *TutorHandler) IngestLessons() fiber.Handler {
	return func(c *fiber.Ctx) error {
		lecturePath := h.cfg.Tutor.LecturePath
		if lecturePath == "" {
			lecturePath = "../lecture"
		}
		count, err := h.ingest.IngestLessons(c.Context(), lecturePath)
		if err != nil {
			return api.InternalError(c, "Ingest failed: "+err.Error())
		}
		return api.OkWithMessage(c, "Lessons ingested", fiber.Map{"count": count})
	}
}
