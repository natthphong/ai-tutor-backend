package handler

import (
	"io"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"gitlab.com/home-server7795544/home-server/iam/iam-backend/api"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/internal/tutor"
)

// ShadowingHandler exposes /shadowing/* endpoints used by the parroto-style UI.
type ShadowingHandler struct {
	svc *tutor.ShadowingService
	app *TutorHandler
}

func NewShadowingHandler(svc *tutor.ShadowingService, app *TutorHandler) *ShadowingHandler {
	return &ShadowingHandler{svc: svc, app: app}
}

func (h *ShadowingHandler) Register(group fiber.Router) {
	g := group.Group("/shadowing")
	g.Post("/clips", h.CreateClip())
	g.Get("/clips", h.ListClips())
	g.Get("/clips/:clipId", h.GetClip())
	g.Patch("/clips/:clipId/progress", h.SaveProgress())
	g.Post("/clips/:clipId/segments/:segmentId/recordings", h.UploadRecording())
	g.Get("/clips/:clipId/segments/:segmentId/recordings", h.ListRecordings())
	g.Post("/clips/:clipId/notes", h.UpsertNote())
	g.Get("/clips/:clipId/notes", h.ListNotes())
	g.Get("/clips/:clipId/segments/:segmentId/translate", h.TranslateSegment())
	g.Post("/clips/:clipId/recordings/:recordingId/score", h.ScoreRecording())
	// Stream proxy. Accepts `?token=<jwt>` so the <video> tag works without
	// custom Authorization headers (HTML5 video can't send them).
	g.Get("/clips/:clipId/stream", h.StreamClip())
	// Retry transcript generation when the first attempt failed (or to refresh).
	g.Post("/clips/:clipId/retry", h.RetryClip())
	g.Post("/clips/:clipId/reprocess", h.RetryClip()) // alias

	// Watched + folder organisation.
	g.Post("/clips/:clipId/mark-watched", h.MarkWatched())
	g.Post("/clips/:clipId/folder", h.MoveToFolder())
	g.Get("/folders", h.ListFolders())
	g.Post("/folders", h.CreateFolder())
	g.Delete("/folders/:folderId", h.DeleteFolder())
}

func (h *ShadowingHandler) MarkWatched() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := h.app.resolveUserID(c, "")
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		var req struct {
			Completed bool `json:"completed"`
		}
		_ = c.BodyParser(&req)
		if err := h.svc.MarkClipWatched(c.Context(), userID, c.Params("clipId"), req.Completed); err != nil {
			return api.BadRequest(c, err.Error())
		}
		return api.OkWithMessage(c, "Updated", fiber.Map{"ok": true})
	}
}

func (h *ShadowingHandler) MoveToFolder() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := h.app.resolveUserID(c, "")
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		var req struct {
			FolderID string `json:"folderId"`
		}
		_ = c.BodyParser(&req)
		if err := h.svc.MoveClipToFolder(c.Context(), userID, c.Params("clipId"), req.FolderID); err != nil {
			return api.BadRequest(c, err.Error())
		}
		return api.OkWithMessage(c, "Moved", fiber.Map{"ok": true})
	}
}

func (h *ShadowingHandler) ListFolders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := h.app.resolveUserID(c, "")
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		folders, err := h.svc.ListFolders(c.Context(), userID)
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		return api.OkWithMessage(c, "Folders", fiber.Map{"folders": folders})
	}
}

func (h *ShadowingHandler) CreateFolder() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := h.app.resolveUserID(c, "")
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		var req struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		}
		if err := c.BodyParser(&req); err != nil || req.Name == "" {
			return api.BadRequest(c, "name required")
		}
		folder, err := h.svc.CreateFolder(c.Context(), userID, req.Name, req.Color)
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		return api.OkWithMessage(c, "Folder created", folder)
	}
}

func (h *ShadowingHandler) DeleteFolder() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := h.app.resolveUserID(c, "")
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		if err := h.svc.DeleteFolder(c.Context(), userID, c.Params("folderId")); err != nil {
			return api.BadRequest(c, err.Error())
		}
		return api.OkWithMessage(c, "Folder deleted", fiber.Map{"ok": true})
	}
}

// RetryClip re-runs Gemini transcript generation for an existing clip while
// keeping the same id / stream URL / progress / recordings / notes.
func (h *ShadowingHandler) RetryClip() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := h.app.resolveUserID(c, "")
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		clip, err := h.svc.ReprocessClip(c.Context(), userID, c.Params("clipId"))
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		return api.OkWithMessage(c, "Reprocess started", fiber.Map{
			"ok":   true,
			"clip": clip,
		})
	}
}

// StreamClip proxies the MinIO video bytes back to the browser. Accepts the
// JWT either in the Authorization header (when accessed via fetch/axios) or
// as a `token` query param (for direct <video src> access).
func (h *ShadowingHandler) StreamClip() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Get("Authorization") == "" {
			if tok := c.Query("token"); tok != "" {
				c.Request().Header.Set("Authorization", "Bearer "+tok)
			}
		}
		userID, err := h.app.resolveUserID(c, "")
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		clip, err := h.svc.LookupClipForOwner(c.Context(), userID, c.Params("clipId"))
		if err != nil {
			return api.NotFound(c, "clip not found")
		}
		if clip.MinIOObjectKey == "" {
			return c.Status(fiber.StatusAccepted).JSON(api.Err("PROCESSING", "video is still processing"))
		}

		rangeHeader := c.Get("Range")
		reader, ct, size, err := h.svc.OpenObjectStream(c.Context(), clip.MinIOObjectKey, rangeHeader)
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		c.Set("Content-Type", ct)
		c.Set("Accept-Ranges", "bytes")
		c.Set("Cache-Control", "private, max-age=300")
		if rangeHeader != "" {
			c.Status(fiber.StatusPartialContent)
		}
		// SendStream takes a length – Fiber will Close the reader for us.
		return c.SendStream(reader, int(size))
	}
}

// ScoreRecording grades the user's recording vs. the segment transcript.
func (h *ShadowingHandler) ScoreRecording() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := h.app.resolveUserID(c, "")
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		score, feedback, err := h.svc.ScoreRecording(c.Context(), userID, c.Params("clipId"), c.Params("recordingId"))
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		return api.OkWithMessage(c, "Recording scored", fiber.Map{"score": score, "feedback": feedback})
	}
}

// TranslateSegment returns the cached Thai translation – no Gemini round-trip.
func (h *ShadowingHandler) TranslateSegment() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := h.app.resolveUserID(c, "")
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		en, th, err := h.svc.TranslateSegment(c.Context(), userID, c.Params("clipId"), c.Params("segmentId"))
		if err != nil {
			return api.NotFound(c, "segment not found")
		}
		return api.OkWithMessage(c, "Translation", fiber.Map{
			"text":            en,
			"thaiTranslation": th,
			"cached":          true,
		})
	}
}

func (h *ShadowingHandler) CreateClip() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req struct {
			UserID     string `json:"userId"`
			YouTubeURL string `json:"youtubeUrl"`
		}
		if err := c.BodyParser(&req); err != nil {
			return api.BadRequest(c, "invalid body")
		}
		userID, err := h.app.resolveUserID(c, req.UserID)
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		if req.YouTubeURL == "" {
			return api.BadRequest(c, "youtubeUrl required")
		}
		clip, err := h.svc.CreateClip(c.Context(), userID, req.YouTubeURL)
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		return api.OkWithMessage(c, "Clip created", clip)
	}
}

func (h *ShadowingHandler) ListClips() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := h.app.resolveUserID(c, "")
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		limit, _ := strconv.Atoi(c.Query("limit", "30"))
		sort := c.Query("sort", "recent") // "recent" | "watched"
		folderID := c.Query("folderId", "")
		unwatched := c.Query("unwatched", "false") == "true"
		clips, err := h.svc.ListClips(c.Context(), userID, limit, sort, folderID, unwatched)
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		return api.OkWithMessage(c, "Shadowing clips", fiber.Map{"clips": clips})
	}
}

func (h *ShadowingHandler) GetClip() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := h.app.resolveUserID(c, "")
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		detail, err := h.svc.GetClipDetail(c.Context(), userID, c.Params("clipId"))
		if err != nil {
			return api.NotFound(c, err.Error())
		}
		return api.OkWithMessage(c, "Shadowing clip", detail)
	}
}

func (h *ShadowingHandler) SaveProgress() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req struct {
			UserID              string  `json:"userId"`
			CurrentSegmentIndex int     `json:"currentSegmentIndex"`
			LastWatchedTime     float64 `json:"lastWatchedTime"`
			CompletedSegments   []int   `json:"completedSegments"`
		}
		if err := c.BodyParser(&req); err != nil {
			return api.BadRequest(c, "invalid body")
		}
		userID, err := h.app.resolveUserID(c, req.UserID)
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		if err := h.svc.SaveProgress(c.Context(), userID, c.Params("clipId"),
			req.CurrentSegmentIndex, req.LastWatchedTime, req.CompletedSegments); err != nil {
			return api.InternalError(c, err.Error())
		}
		return api.OkWithMessage(c, "Progress saved", fiber.Map{"ok": true})
	}
}

func (h *ShadowingHandler) UploadRecording() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := h.app.resolveUserID(c, c.FormValue("userId"))
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		fh, err := c.FormFile("file")
		if err != nil {
			return api.BadRequest(c, "audio file required")
		}
		src, err := fh.Open()
		if err != nil {
			return api.InternalError(c, "open file failed")
		}
		defer src.Close()
		audio, err := io.ReadAll(src)
		if err != nil {
			return api.InternalError(c, "read file failed")
		}
		duration, _ := strconv.ParseFloat(c.FormValue("durationSeconds"), 64)
		id, url, err := h.svc.SaveRecording(c.Context(), userID, c.Params("clipId"), c.Params("segmentId"),
			audio, fh.Header.Get("Content-Type"), duration)
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		return api.OkWithMessage(c, "Recording saved", fiber.Map{"id": id, "audioUrl": url})
	}
}

func (h *ShadowingHandler) ListRecordings() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := h.app.resolveUserID(c, "")
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		recs, err := h.svc.ListRecordings(c.Context(), userID, c.Params("clipId"), c.Params("segmentId"))
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		return api.OkWithMessage(c, "Recordings", fiber.Map{"recordings": recs})
	}
}

func (h *ShadowingHandler) UpsertNote() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req struct {
			UserID    string `json:"userId"`
			SegmentID string `json:"segmentId"`
			NoteText  string `json:"noteText"`
		}
		if err := c.BodyParser(&req); err != nil {
			return api.BadRequest(c, "invalid body")
		}
		userID, err := h.app.resolveUserID(c, req.UserID)
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		if req.NoteText == "" {
			return api.BadRequest(c, "noteText required")
		}
		note, err := h.svc.UpsertNote(c.Context(), userID, c.Params("clipId"), req.SegmentID, req.NoteText)
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		return api.OkWithMessage(c, "Note saved", note)
	}
}

func (h *ShadowingHandler) ListNotes() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := h.app.resolveUserID(c, "")
		if err != nil {
			return api.BadRequest(c, err.Error())
		}
		notes, err := h.svc.ListNotes(c.Context(), userID, c.Params("clipId"))
		if err != nil {
			return api.InternalError(c, err.Error())
		}
		return api.OkWithMessage(c, "Notes", fiber.Map{"notes": notes})
	}
}
