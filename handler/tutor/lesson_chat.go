package handler

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/api"
	authPkg "gitlab.com/home-server7795544/home-server/iam/iam-backend/handler/auth"
)

// resolveUserID accepts an authenticated JWT (preferred) OR a userId provided
// in the query/body. We deliberately parse the JWT here too so the endpoint
// works whether or not the route group applies middleware – the existing
// frontend axios client always sends an Authorization header.
func (h *TutorHandler) resolveUserID(c *fiber.Ctx, fallback string) (string, error) {
	if v, ok := c.Locals("userId").(string); ok && v != "" {
		return v, nil
	}
	if claims := authPkg.ParseAuthHeader(c); claims != nil && claims.UserID != "" {
		// Stash so downstream calls don't re-parse.
		c.Locals("userId", claims.UserID)
		c.Locals("lineUserId", claims.LineUserID)
		return claims.UserID, nil
	}
	if fallback == "" {
		fallback = c.Query("userId")
	}
	if fallback == "" {
		return "", fiber.NewError(fiber.StatusBadRequest, "userId required")
	}
	return h.svc.EnsureUser(c.Context(), fallback, "")
}

// GetLessonChat returns the full chat transcript for (userId, lessonId).
// lessonId here is the lesson_units.id (integer "unit id").
func (h *TutorHandler) GetLessonChat() fiber.Handler {
	return func(c *fiber.Ctx) error {
		lessonID, err := strconv.Atoi(c.Params("lessonId"))
		if err != nil || lessonID <= 0 {
			return api.BadRequest(c, "invalid lessonId")
		}
		userID, err := h.resolveUserID(c, "")
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		messages, err := h.svc.GetUnitHistory(c.Context(), userID, lessonID)
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		return api.OkWithMessage(c, "Lesson chat", fiber.Map{
			"lessonId": lessonID,
			"messages": messages,
		})
	}
}

// AppendLessonChat persists a message into tutor_messages. The frontend uses
// this for messages it generated locally (e.g. system hints) that aren't part
// of a /turn round-trip; the tutor's own replies are already persisted by the
// HandleTurn flow.
func (h *TutorHandler) AppendLessonChat() fiber.Handler {
	return func(c *fiber.Ctx) error {
		lessonID, err := strconv.Atoi(c.Params("lessonId"))
		if err != nil || lessonID <= 0 {
			return api.BadRequest(c, "invalid lessonId")
		}
		var req struct {
			UserID    string                 `json:"userId"`
			Role      string                 `json:"role"`
			Content   string                 `json:"content"`
			ContentTh string                 `json:"contentTh"`
			Type      string                 `json:"type"`
			Metadata  map[string]interface{} `json:"metadata"`
			SessionID string                 `json:"sessionId"`
		}
		if err := c.BodyParser(&req); err != nil {
			return api.BadRequest(c, "invalid body")
		}
		userID, err := h.resolveUserID(c, req.UserID)
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		role := strings.ToLower(strings.TrimSpace(req.Role))
		if role != "user" && role != "assistant" && role != "system" {
			role = "user"
		}
		mType := strings.TrimSpace(req.Type)
		if mType == "" {
			mType = "text"
		}
		h.svc.InsertLessonMessage(c.Context(), req.SessionID, userID, lessonID, role, req.Content, req.ContentTh, mType, req.Metadata)
		return api.OkWithMessage(c, "Message stored", fiber.Map{"ok": true})
	}
}

// GetLessonProgress returns the user_unit_progress row for (user, unit).
func (h *TutorHandler) GetLessonProgress() fiber.Handler {
	return func(c *fiber.Ctx) error {
		lessonID, err := strconv.Atoi(c.Params("lessonId"))
		if err != nil || lessonID <= 0 {
			return api.BadRequest(c, "invalid lessonId")
		}
		userID, err := h.resolveUserID(c, "")
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		prog, err := h.svc.GetLessonProgress(c.Context(), userID, lessonID)
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		return api.OkWithMessage(c, "Lesson progress", prog)
	}
}

// UpdateLessonProgress allows clients to persist incremental progress
// (current_step, scores) for resume after refresh.
func (h *TutorHandler) UpdateLessonProgress() fiber.Handler {
	return func(c *fiber.Ctx) error {
		lessonID, err := strconv.Atoi(c.Params("lessonId"))
		if err != nil || lessonID <= 0 {
			return api.BadRequest(c, "invalid lessonId")
		}
		var req struct {
			UserID      string  `json:"userId"`
			CurrentStep string  `json:"currentStep"`
			Status      string  `json:"status"`
			Listening   float64 `json:"listeningScore"`
			Speaking    float64 `json:"speakingScore"`
			Reading     float64 `json:"readingScore"`
		}
		if err := c.BodyParser(&req); err != nil {
			return api.BadRequest(c, "invalid body")
		}
		userID, err := h.resolveUserID(c, req.UserID)
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		prog, err := h.svc.UpsertLessonProgress(c.Context(), userID, lessonID,
			req.CurrentStep, req.Status, req.Listening, req.Speaking, req.Reading)
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		return api.OkWithMessage(c, "Progress saved", prog)
	}
}
