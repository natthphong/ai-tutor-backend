package tutor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"go.uber.org/zap"

	"gitlab.com/home-server7795544/home-server/iam/iam-backend/config"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/internal/ai"
)

// ShadowingService is responsible for the parroto.app-style YouTube practice
// flow: download → MinIO upload → Gemini transcript → DB persistence →
// segment-based playback / recording.
type ShadowingService struct {
	db          *pgxpool.Pool
	router      *ai.Router
	minioClient *minio.Client
	cfg         *config.Config
	logger      *zap.Logger
}

func NewShadowingService(db *pgxpool.Pool, router *ai.Router, mc *minio.Client, cfg *config.Config) *ShadowingService {
	return &ShadowingService{db: db, router: router, minioClient: mc, cfg: cfg, logger: zap.L()}
}

// ---- DTOs ----

type ShadowingClipDTO struct {
	ID               string     `json:"id"`
	UserID           string     `json:"userId"`
	YouTubeURL       string     `json:"youtubeUrl"`
	YouTubeID        string     `json:"youtubeId"`
	Title            string     `json:"title"`
	ThumbnailURL     string     `json:"thumbnailUrl"`
	MinIOObjectKey   string     `json:"minioObjectKey,omitempty"`
	StreamURL        string     `json:"streamUrl"`
	ProxyStreamURL   string     `json:"proxyStreamUrl,omitempty"`
	DurationSeconds  int        `json:"durationSeconds"`
	Status           string     `json:"status"`
	VideoStatus      string     `json:"videoStatus"`
	TranscriptStatus string     `json:"transcriptStatus"`
	ErrorMessage     string     `json:"errorMessage,omitempty"`
	FolderID         string     `json:"folderId,omitempty"`
	IsCompleted      bool       `json:"isCompleted"`
	WatchedAt        *time.Time `json:"watchedAt,omitempty"`
	LastSegmentIdx   int        `json:"lastSegmentIndex"`
	LastWatchedTime  float64    `json:"lastWatchedTime"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type ShadowingFolderDTO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color,omitempty"`
	ClipCount int       `json:"clipCount"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ShadowingSegmentDTO struct {
	ID              string  `json:"id"`
	Index           int     `json:"index"`
	StartTime       float64 `json:"startTime"`
	EndTime         float64 `json:"endTime"`
	Text            string  `json:"text"`
	ThaiTranslation string  `json:"thaiTranslation"`
	IPA             string  `json:"ipa,omitempty"`
}

type ShadowingProgressDTO struct {
	CurrentSegmentIndex int     `json:"currentSegmentIndex"`
	LastWatchedTime     float64 `json:"lastWatchedTime"`
	CompletedSegments   []int   `json:"completedSegments"`
}

type ShadowingClipDetail struct {
	Clip     ShadowingClipDTO      `json:"clip"`
	Segments []ShadowingSegmentDTO `json:"segments"`
	Progress ShadowingProgressDTO  `json:"progress"`
}

// ---- Helpers ----

var youtubeIDRegex = regexp.MustCompile(`(?:v=|youtu\.be/|youtube\.com/shorts/|/embed/)([A-Za-z0-9_-]{11})`)

// ParseYouTubeID extracts the 11-character video id from a URL. Returns ""
// when the URL is unrecognised.
func ParseYouTubeID(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if m := youtubeIDRegex.FindStringSubmatch(rawURL); len(m) == 2 {
		return m[1]
	}
	// Also accept bare video IDs.
	if len(rawURL) == 11 && regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`).MatchString(rawURL) {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err == nil {
		if v := u.Query().Get("v"); len(v) == 11 {
			return v
		}
	}
	return ""
}

// localFallbackEnabled returns true when the operator opted out of real
// YouTube downloading. The fallback path still creates a clip + canned segments
// so the frontend has something to render during dev / CI.
func (s *ShadowingService) localFallbackEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("SHADOWING_LOCAL_FALLBACK")))
	return v == "1" || v == "true" || v == "yes"
}

// ---- Public API ----

// CreateClip inserts the clip row with the YouTube embed URL ready to play
// and kicks off transcript generation in the background. video_status is
// ready the moment we hand back the embed URL, but clip-level `status` stays
// "processing" until transcript_status flips to ready so the UI can tell the
// user the clip isn't fully usable for shadowing yet.
func (s *ShadowingService) CreateClip(ctx context.Context, userID, youtubeURL string) (ShadowingClipDTO, error) {
	yid := ParseYouTubeID(youtubeURL)
	if yid == "" {
		return ShadowingClipDTO{}, fmt.Errorf("invalid YouTube URL")
	}
	id := uuid.New().String()
	thumb := fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", yid)
	streamURL := fmt.Sprintf("https://www.youtube.com/embed/%s", yid)
	// Placeholder title – replaced once yt-dlp returns the real one. We keep
	// it user-friendly so the UI never has to display a raw URL.
	provisionalTitle := fmt.Sprintf("Loading title… (%s)", yid)
	_, err := s.db.Exec(ctx,
		`INSERT INTO shadowing_clips (id, user_id, youtube_url, youtube_id, title, thumbnail_url, stream_url, status, video_status, transcript_status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,'processing','ready','pending')`,
		id, userID, youtubeURL, yid, provisionalTitle, thumb, streamURL)
	if err != nil {
		return ShadowingClipDTO{}, fmt.Errorf("insert clip: %w", err)
	}
	s.logger.Info("shadowing: clip created",
		zap.String("clipId", id),
		zap.String("userId", userID),
		zap.String("youtubeId", yid))

	// Background transcript generation. Detached context because the request
	// context is cancelled as soon as we respond.
	go s.processTranscriptAsync(id, yid)

	return s.getClip(ctx, id)
}

// ReprocessClip re-runs transcript generation for an existing clip. The
// stream URL, youtube id and title are preserved. Returns the latest clip
// state. Safe to call multiple times.
func (s *ShadowingService) ReprocessClip(ctx context.Context, userID, clipID string) (ShadowingClipDTO, error) {
	clip, err := s.getClip(ctx, clipID)
	if err != nil {
		return ShadowingClipDTO{}, err
	}
	if clip.UserID != userID {
		return ShadowingClipDTO{}, fmt.Errorf("forbidden")
	}
	if clip.YouTubeID == "" {
		return ShadowingClipDTO{}, fmt.Errorf("clip has no youtube id")
	}
	// Mark transcript pending; keep video/stream untouched.
	_, _ = s.db.Exec(ctx,
		`UPDATE shadowing_clips
		   SET transcript_status = 'pending', status = 'processing',
		       error_message = '', updated_at = now()
		 WHERE id = $1`, clipID)
	s.logger.Info("shadowing: reprocess requested",
		zap.String("clipId", clipID),
		zap.String("userId", userID),
		zap.String("youtubeId", clip.YouTubeID))
	go s.processTranscriptAsync(clipID, clip.YouTubeID)
	return s.getClip(ctx, clipID)
}

// ListClips returns the user's shadowing clips. Sort options:
//
//	"recent"  – most recently created (default)
//	"watched" – most recently watched (resume / continue-watching surface)
//
// Filter options:
//
//	folderId   – limit to a specific folder (empty string returns all)
//	unwatched  – when true, only is_completed=false
func (s *ShadowingService) ListClips(ctx context.Context, userID string, limit int, sort, folderID string, unwatched bool) ([]ShadowingClipDTO, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	orderBy := "c.created_at DESC NULLS LAST"
	if sort == "watched" {
		orderBy = "COALESCE(p.updated_at, c.updated_at) DESC NULLS LAST"
	}
	folderClause := ""
	args := []interface{}{userID, limit}
	if folderID != "" {
		folderClause = " AND c.folder_id = $3 "
		args = append(args, folderID)
	}
	if unwatched {
		folderClause += " AND COALESCE(c.is_completed,false) = false "
	}
	query := `
		SELECT c.id::text, c.user_id::text, c.youtube_url, COALESCE(c.youtube_id,''), COALESCE(c.title,''),
		       COALESCE(c.thumbnail_url,''), COALESCE(c.minio_object_key,''), COALESCE(c.stream_url,''),
		       COALESCE(c.duration_seconds,0), COALESCE(c.status,'pending'),
		       COALESCE(c.video_status,'pending'), COALESCE(c.transcript_status,'pending'),
		       COALESCE(c.error_message,''),
		       COALESCE(c.folder_id::text,''), COALESCE(c.is_completed,false), c.watched_at,
		       COALESCE(p.current_segment_index,0), COALESCE(p.last_watched_time,0),
		       c.created_at, c.updated_at
		FROM shadowing_clips c
		LEFT JOIN shadowing_progress p ON p.clip_id = c.id AND p.user_id = c.user_id
		WHERE c.user_id = $1` + folderClause + `
		ORDER BY ` + orderBy + `
		LIMIT $2`
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ShadowingClipDTO
	for rows.Next() {
		var c ShadowingClipDTO
		if err := rows.Scan(&c.ID, &c.UserID, &c.YouTubeURL, &c.YouTubeID, &c.Title,
			&c.ThumbnailURL, &c.MinIOObjectKey, &c.StreamURL, &c.DurationSeconds, &c.Status,
			&c.VideoStatus, &c.TranscriptStatus, &c.ErrorMessage,
			&c.FolderID, &c.IsCompleted, &c.WatchedAt,
			&c.LastSegmentIdx, &c.LastWatchedTime,
			&c.CreatedAt, &c.UpdatedAt); err == nil {
			out = append(out, s.enrichClip(ctx, c))
		}
	}
	return out, rows.Err()
}

// MarkClipWatched flips is_completed on the clip and stamps watched_at.
func (s *ShadowingService) MarkClipWatched(ctx context.Context, userID, clipID string, completed bool) error {
	res, err := s.db.Exec(ctx,
		`UPDATE shadowing_clips
		    SET is_completed = $1,
		        watched_at = CASE WHEN $1 THEN now() ELSE watched_at END,
		        updated_at = now()
		  WHERE id = $2 AND user_id = $3`,
		completed, clipID, userID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("clip not found")
	}
	s.logger.Info("shadowing: mark watched",
		zap.String("clipId", clipID),
		zap.String("userId", userID),
		zap.Bool("completed", completed))
	return nil
}

// MoveClipToFolder sets/clears folder_id on a clip. Pass "" to detach.
func (s *ShadowingService) MoveClipToFolder(ctx context.Context, userID, clipID, folderID string) error {
	if folderID == "" {
		res, err := s.db.Exec(ctx,
			`UPDATE shadowing_clips SET folder_id = NULL, updated_at = now()
			  WHERE id = $1 AND user_id = $2`, clipID, userID)
		if err != nil {
			return err
		}
		if res.RowsAffected() == 0 {
			return fmt.Errorf("clip not found")
		}
		return nil
	}
	// Verify folder ownership.
	var owner string
	if err := s.db.QueryRow(ctx,
		`SELECT user_id::text FROM shadowing_folders WHERE id = $1`, folderID).Scan(&owner); err != nil {
		return fmt.Errorf("folder not found")
	}
	if owner != userID {
		return fmt.Errorf("forbidden")
	}
	res, err := s.db.Exec(ctx,
		`UPDATE shadowing_clips SET folder_id = $1, updated_at = now()
		  WHERE id = $2 AND user_id = $3`, folderID, clipID, userID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("clip not found")
	}
	return nil
}

// CreateFolder makes a new shadowing folder for the user.
func (s *ShadowingService) CreateFolder(ctx context.Context, userID, name, color string) (ShadowingFolderDTO, error) {
	id := uuid.New().String()
	_, err := s.db.Exec(ctx,
		`INSERT INTO shadowing_folders (id, user_id, name, color) VALUES ($1,$2,$3,$4)`,
		id, userID, name, color)
	if err != nil {
		return ShadowingFolderDTO{}, err
	}
	return ShadowingFolderDTO{ID: id, Name: name, Color: color}, nil
}

// ListFolders returns the user's folders with clip counts.
func (s *ShadowingService) ListFolders(ctx context.Context, userID string) ([]ShadowingFolderDTO, error) {
	rows, err := s.db.Query(ctx, `
		SELECT f.id::text, f.name, COALESCE(f.color,''),
		       (SELECT COUNT(*) FROM shadowing_clips c WHERE c.folder_id = f.id) as clip_count,
		       f.created_at, f.updated_at
		FROM shadowing_folders f
		WHERE f.user_id = $1
		ORDER BY f.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ShadowingFolderDTO
	for rows.Next() {
		var f ShadowingFolderDTO
		if err := rows.Scan(&f.ID, &f.Name, &f.Color, &f.ClipCount, &f.CreatedAt, &f.UpdatedAt); err == nil {
			out = append(out, f)
		}
	}
	return out, rows.Err()
}

// DeleteFolder detaches all clips from the folder and removes it.
func (s *ShadowingService) DeleteFolder(ctx context.Context, userID, folderID string) error {
	res, err := s.db.Exec(ctx,
		`DELETE FROM shadowing_folders WHERE id = $1 AND user_id = $2`,
		folderID, userID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("folder not found")
	}
	return nil
}

// GetClipDetail returns clip + segments + progress for resume.
func (s *ShadowingService) GetClipDetail(ctx context.Context, userID, clipID string) (ShadowingClipDetail, error) {
	clip, err := s.getClip(ctx, clipID)
	if err != nil {
		return ShadowingClipDetail{}, err
	}
	if clip.UserID != userID {
		return ShadowingClipDetail{}, fmt.Errorf("forbidden")
	}
	clip = s.enrichClip(ctx, clip)
	segs, err := s.listSegments(ctx, clipID)
	if err != nil {
		return ShadowingClipDetail{}, err
	}
	prog := s.getProgress(ctx, userID, clipID)
	return ShadowingClipDetail{Clip: clip, Segments: segs, Progress: prog}, nil
}

// enrichClip replaces streamUrl with a short-lived presigned URL when the
// clip has a MinIO object. We always advertise a backend proxy stream URL too
// so the frontend can fall back if presigned URLs can't reach the browser
// (e.g. MinIO bound to a private network).
// enrichClip is a no-op for new YouTube-embed clips: streamUrl already points
// at youtube.com/embed/<id>. For legacy clips that have a MinIO object key we
// (a) replace streamUrl with a fresh presigned URL and (b) advertise the
// backend stream proxy as a fallback when presigned URLs are unreachable.
func (s *ShadowingService) enrichClip(ctx context.Context, c ShadowingClipDTO) ShadowingClipDTO {
	if c.MinIOObjectKey == "" {
		return c
	}
	if presigned := s.PresignedGet(ctx, c.MinIOObjectKey); presigned != "" {
		c.StreamURL = presigned
	}
	c.ProxyStreamURL = fmt.Sprintf("/v1/shadowing/clips/%s/stream", c.ID)
	return c
}

// SaveProgress upserts current segment index + last_watched_time.
func (s *ShadowingService) SaveProgress(ctx context.Context, userID, clipID string, segIdx int, time float64, completed []int) error {
	completedJSON, _ := json.Marshal(completed)
	_, err := s.db.Exec(ctx,
		`INSERT INTO shadowing_progress (id, user_id, clip_id, current_segment_index, last_watched_time, completed_segments)
		 VALUES ($1,$2,$3,$4,$5,$6::jsonb)
		 ON CONFLICT (user_id, clip_id) DO UPDATE SET
		   current_segment_index = EXCLUDED.current_segment_index,
		   last_watched_time = EXCLUDED.last_watched_time,
		   completed_segments = EXCLUDED.completed_segments,
		   updated_at = now()`,
		uuid.New().String(), userID, clipID, segIdx, time, string(completedJSON))
	return err
}

// SaveRecording uploads the audio to MinIO and stores metadata.
func (s *ShadowingService) SaveRecording(ctx context.Context, userID, clipID, segmentID string, audio []byte, contentType string, duration float64) (string, string, error) {
	if s.minioClient == nil {
		return "", "", fmt.Errorf("minio not configured")
	}
	bucket := s.cfg.MinIO.Bucket
	prefix := s.cfg.MinIO.PrefixUserAudio
	if prefix == "" {
		prefix = "user-audio/"
	}
	key := fmt.Sprintf("%sshadowing/%s/%s/%s.webm", prefix, clipID, segmentID, uuid.New().String())
	_, err := s.minioClient.PutObject(ctx, bucket, key, byteReader(audio), int64(len(audio)),
		minio.PutObjectOptions{ContentType: contentTypeOrDefault(contentType, "audio/webm")})
	if err != nil {
		return "", "", fmt.Errorf("minio put: %w", err)
	}
	publicURL := s.objectURL(bucket, key)

	recID := uuid.New().String()
	_, err = s.db.Exec(ctx,
		`INSERT INTO shadowing_recordings (id, user_id, clip_id, segment_id, audio_object_key, audio_url, duration_seconds)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		recID, userID, clipID, nullableUUID(segmentID), key, publicURL, duration)
	if err != nil {
		return "", "", fmt.Errorf("insert recording: %w", err)
	}
	return recID, publicURL, nil
}

func (s *ShadowingService) ListRecordings(ctx context.Context, userID, clipID, segmentID string) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id::text, COALESCE(segment_id::text,''), audio_object_key, COALESCE(audio_url,''),
		        COALESCE(duration_seconds,0), COALESCE(score,0), COALESCE(ai_feedback,''), created_at
		 FROM shadowing_recordings
		 WHERE user_id = $1 AND clip_id = $2 AND ($3 = '' OR segment_id::text = $3)
		 ORDER BY created_at DESC LIMIT 50`,
		userID, clipID, segmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var id, segID, objectKey, audioURL, fb string
		var dur, score float64
		var createdAt time.Time
		if err := rows.Scan(&id, &segID, &objectKey, &audioURL, &dur, &score, &fb, &createdAt); err == nil {
			out = append(out, map[string]interface{}{
				"id":              id,
				"segmentId":       segID,
				"audioObjectKey":  objectKey,
				"audioUrl":        audioURL,
				"durationSeconds": dur,
				"score":           score,
				"aiFeedback":      fb,
				"createdAt":       createdAt,
			})
		}
	}
	return out, rows.Err()
}

func (s *ShadowingService) UpsertNote(ctx context.Context, userID, clipID, segmentID, text string) (map[string]interface{}, error) {
	noteID := uuid.New().String()
	// Per-segment notes are stored uniquely per (user, clip, segment). For
	// clip-level notes (segmentID empty) we always insert a new row.
	if segmentID != "" {
		_, err := s.db.Exec(ctx,
			`DELETE FROM shadowing_notes WHERE user_id = $1 AND clip_id = $2 AND segment_id = $3::uuid`,
			userID, clipID, segmentID)
		if err != nil {
			return nil, err
		}
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO shadowing_notes (id, user_id, clip_id, segment_id, note_text) VALUES ($1,$2,$3,$4,$5)`,
		noteID, userID, clipID, nullableUUID(segmentID), text)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"id": noteID, "noteText": text, "segmentId": segmentID}, nil
}

func (s *ShadowingService) ListNotes(ctx context.Context, userID, clipID string) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id::text, COALESCE(segment_id::text,''), note_text, created_at, updated_at
		 FROM shadowing_notes WHERE user_id = $1 AND clip_id = $2 ORDER BY created_at DESC`,
		userID, clipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var id, segID, text string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &segID, &text, &createdAt, &updatedAt); err == nil {
			out = append(out, map[string]interface{}{
				"id":        id,
				"segmentId": segID,
				"noteText":  text,
				"createdAt": createdAt,
				"updatedAt": updatedAt,
			})
		}
	}
	return out, rows.Err()
}

// ---- Internal ----

// processTranscriptAsync regenerates the per-clip transcript+translation.
//
// Flow:
//  1. Fetch real title + duration from yt-dlp metadata (so we don't keep
//     showing a raw URL on the UI).
//  2. Try YouTube auto-captions first — these come with REAL timestamps so
//     prev/next/auto-stop stay in sync with playback. We then ask Gemini to
//     produce Thai translations only (much cheaper, much more reliable).
//  3. If captions are unavailable, fall back to STT-on-audio + Gemini
//     segmentation. Timings here are estimates.
//  4. status is set to 'ready' ONLY when segments are actually persisted.
//     Failures leave segments=[] and transcript_status='failed' with a
//     retryable error message.
func (s *ShadowingService) processTranscriptAsync(clipID, yid string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	logger := s.logger.With(zap.String("clipId", clipID), zap.String("youtubeId", yid))
	logger.Info("shadowing: transcript start")
	s.setTranscriptStatus(ctx, clipID, "processing", "")

	// Explicit opt-in for canned segments (CI / agent tests). Default off.
	if s.localFallbackEnabled() {
		logger.Info("shadowing: local fallback (env flag) — using canned transcript")
		s.applyFallbackTranscript(ctx, clipID)
		s.setClipStatus(ctx, clipID, "ready")
		return
	}

	youtubeURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", yid)
	tmpDir, err := os.MkdirTemp("", "shadow-"+clipID+"-*")
	if err != nil {
		logger.Error("shadowing: tmpdir failed", zap.Error(err))
		s.setTranscriptStatus(ctx, clipID, "failed", "tmpdir: "+err.Error())
		s.setClipStatus(ctx, clipID, "failed")
		return
	}
	defer os.RemoveAll(tmpDir)

	// (1) Metadata: title + duration.
	if title, duration := s.fetchYouTubeMetadata(ctx, youtubeURL); title != "" {
		logger.Info("shadowing: title resolved", zap.String("title", title), zap.Int("durationSec", duration))
		s.updateClipMeta(ctx, clipID, title, duration)
	}

	// (2) Captions-first path. yt-dlp emits a VTT with real timings that
	// already match the YouTube player, so prev / next / auto-stop will be
	// aligned with playback.
	rawCaps, capErr := s.downloadAutoCaptions(ctx, youtubeURL, tmpDir)
	if capErr == nil && len(rawCaps) > 0 {
		logger.Info("shadowing: using auto-captions",
			zap.Int("rawCues", len(rawCaps)))
		segs := CombineToSentences(rawCaps)
		if len(segs) > 0 {
			if tErr := s.translateSegments(ctx, segs); tErr != nil {
				logger.Warn("shadowing: translation failed (segments saved without Thai)", zap.Error(tErr))
			}
			s.replaceSegments(ctx, clipID, segs)
			duration := 0
			if last := segs[len(segs)-1]; last.EndTime > 0 {
				duration = int(last.EndTime)
			}
			s.finalizeTranscript(ctx, clipID, duration)
			logger.Info("shadowing: transcript ready (captions)",
				zap.Int("segments", len(segs)),
				zap.Int("durationSec", duration))
			return
		}
		logger.Warn("shadowing: captions combined into 0 segments — trying STT fallback")
	} else if capErr != nil {
		logger.Warn("shadowing: auto-captions unavailable — trying STT fallback",
			zap.Error(capErr))
	}

	// (3) STT fallback. Timings here are estimated by Gemini.
	mediaPath, audioDur, err := s.downloadYouTubeAudio(ctx, youtubeURL, tmpDir)
	if err != nil {
		logger.Error("shadowing: yt-dlp audio failed", zap.Error(err))
		s.clearSegments(ctx, clipID)
		s.setTranscriptStatus(ctx, clipID, "failed",
			"Could not download YouTube media. You can retry.")
		s.setClipStatus(ctx, clipID, "failed")
		return
	}

	segs, err := s.transcribeAudioFile(ctx, mediaPath, audioDur)
	if err != nil || len(segs) == 0 {
		logger.Error("shadowing: STT/segment failed", zap.Error(err))
		s.clearSegments(ctx, clipID)
		s.setTranscriptStatus(ctx, clipID, "failed",
			"Transcript generation failed. Please retry.")
		s.setClipStatus(ctx, clipID, "failed")
		return
	}
	s.replaceSegments(ctx, clipID, segs)
	if audioDur <= 0 && len(segs) > 0 {
		audioDur = int(segs[len(segs)-1].EndTime)
	}
	s.finalizeTranscript(ctx, clipID, audioDur)
	logger.Info("shadowing: transcript ready (STT)",
		zap.Int("segments", len(segs)),
		zap.Int("durationSec", audioDur))
}

// finalizeTranscript flips the clip + transcript to ready.
func (s *ShadowingService) finalizeTranscript(ctx context.Context, clipID string, duration int) {
	_, _ = s.db.Exec(ctx, `
		UPDATE shadowing_clips
		SET transcript_status = 'ready',
		    status = 'ready',
		    duration_seconds = COALESCE(NULLIF(duration_seconds,0), $1),
		    error_message = '', updated_at = now()
		WHERE id = $2`, duration, clipID)
}

// setClipStatus updates the clip-level status only.
func (s *ShadowingService) setClipStatus(ctx context.Context, clipID, status string) {
	_, _ = s.db.Exec(ctx,
		`UPDATE shadowing_clips SET status = $1, updated_at = now() WHERE id = $2`,
		status, clipID)
}

func (s *ShadowingService) clearSegments(ctx context.Context, clipID string) {
	_, _ = s.db.Exec(ctx, `DELETE FROM shadowing_segments WHERE clip_id = $1`, clipID)
}

func (s *ShadowingService) updateClipMeta(ctx context.Context, clipID, title string, duration int) {
	_, _ = s.db.Exec(ctx,
		`UPDATE shadowing_clips
		    SET title = COALESCE(NULLIF($1,''), title),
		        duration_seconds = COALESCE(NULLIF($2,0), duration_seconds),
		        updated_at = now()
		  WHERE id = $3`, title, duration, clipID)
}

// fetchYouTubeMetadata returns (title, durationSeconds). Best-effort — empty
// title means we couldn't read it.
func (s *ShadowingService) fetchYouTubeMetadata(ctx context.Context, ytURL string) (string, int) {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return "", 0
	}
	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--no-playlist", "--no-warnings",
		"--print", "%(title)s",
		"--print", "%(duration)s",
		ytURL)
	out, err := cmd.Output()
	if err != nil {
		return "", 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	title := ""
	duration := 0
	if len(lines) >= 1 {
		title = strings.TrimSpace(lines[0])
	}
	if len(lines) >= 2 {
		duration, _ = strconv.Atoi(strings.TrimSpace(lines[1]))
	}
	return title, duration
}

// downloadAutoCaptions fetches the English VTT subtitles (manual or auto)
// using yt-dlp and returns the parsed cues. Returns an error when captions
// are unavailable — the caller should fall back to STT in that case.
func (s *ShadowingService) downloadAutoCaptions(ctx context.Context, ytURL, tmpDir string) ([]RawCaption, error) {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return nil, fmt.Errorf("yt-dlp not installed")
	}
	outTemplate := filepath.Join(tmpDir, "captions.%(ext)s")
	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--no-playlist", "--no-warnings",
		"--skip-download",
		"--write-auto-subs", "--write-subs",
		"--sub-langs", "en.*,en",
		"--sub-format", "vtt/best",
		"--convert-subs", "vtt",
		"-o", outTemplate,
		ytURL,
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp captions: %w", err)
	}
	entries, _ := os.ReadDir(tmpDir)
	var vttPath string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".vtt") {
			vttPath = filepath.Join(tmpDir, e.Name())
			break
		}
	}
	if vttPath == "" {
		return nil, fmt.Errorf("no .vtt produced (video may have captions disabled)")
	}
	data, err := os.ReadFile(vttPath)
	if err != nil {
		return nil, err
	}
	return ParseVTT(string(data))
}

// translateSegments asks Gemini for Thai translations of pre-timed segments.
// Times are NOT changed — only thai_translation is populated. The segments
// slice is mutated in place.
func (s *ShadowingService) translateSegments(ctx context.Context, segs []ShadowingSegmentDTO) error {
	if len(segs) == 0 {
		return nil
	}
	var b strings.Builder
	for i, seg := range segs {
		b.WriteString(fmt.Sprintf("%d. %s\n", i, seg.Text))
	}
	prompt := fmt.Sprintf(`Translate each numbered English segment into natural
Thai for a learner. Keep the count and order EXACTLY the same. Respond in
STRICT JSON only, no markdown fence:
{"translations": ["thai for segment 0", "thai for segment 1", ...]}

SEGMENTS:
%s`, b.String())

	chat, err := s.router.Chat(ctx, ai.ChatRequest{
		SystemPrompt: "You only output strict JSON. Never include markdown fences.",
		Messages:     []ai.ChatMessage{{Role: "user", Content: prompt}},
		UseCase:      "shadowing_translate",
	})
	if err != nil {
		return err
	}
	var parsed struct {
		Translations []string `json:"translations"`
	}
	if err := json.Unmarshal([]byte(stripCodeFences(chat.Content)), &parsed); err != nil {
		return err
	}
	for i := range segs {
		if i < len(parsed.Translations) {
			segs[i].ThaiTranslation = strings.TrimSpace(parsed.Translations[i])
		}
	}
	return nil
}

// applyFallbackTranscript inserts a 3-line canned transcript. Only runs when
// the operator opts in with SHADOWING_LOCAL_FALLBACK=true (CI / agent tests
// without yt-dlp or Gemini). Never auto-runs on production failures.
func (s *ShadowingService) applyFallbackTranscript(ctx context.Context, clipID string) {
	segs := []ShadowingSegmentDTO{
		{Index: 0, StartTime: 0, EndTime: 4.2, Text: "Look at this city.", ThaiTranslation: "ดูเมืองนี้สิ"},
		{Index: 1, StartTime: 4.2, EndTime: 7.8, Text: "My name's Kiki and I'm a witch.", ThaiTranslation: "ฉันชื่อกิกิ และฉันเป็นแม่มด"},
		{Index: 2, StartTime: 7.8, EndTime: 12.0, Text: "I love flying through the sky.", ThaiTranslation: "ฉันรักการบินผ่านท้องฟ้า"},
	}
	s.replaceSegments(ctx, clipID, segs)
	_, _ = s.db.Exec(ctx,
		`UPDATE shadowing_clips
		   SET transcript_status = 'ready', duration_seconds = 12,
		       error_message = '(fallback transcript, dev only)', updated_at = now()
		 WHERE id = $1`, clipID)
}

// downloadYouTubeAudio extracts the best-available audio track using yt-dlp.
// Returns the local file path and the video duration in seconds (0 if unknown).
func (s *ShadowingService) downloadYouTubeAudio(ctx context.Context, ytURL, tmpDir string) (string, int, error) {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return "", 0, fmt.Errorf("yt-dlp not installed")
	}

	// Audio-only download → tmpDir/audio.m4a (or .webm, .opus, … depending on
	// what YouTube serves). The output template lets yt-dlp pick the right
	// extension; we pick the resulting file by listing the directory.
	outTemplate := filepath.Join(tmpDir, "audio.%(ext)s")
	cmd := exec.CommandContext(ctx, "yt-dlp",
		"-f", "bestaudio[ext=m4a]/bestaudio[ext=webm]/bestaudio",
		"--no-playlist",
		"-o", outTemplate,
		ytURL,
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", 0, fmt.Errorf("yt-dlp run: %w", err)
	}

	// Locate whichever extension yt-dlp settled on.
	var mediaPath string
	entries, _ := os.ReadDir(tmpDir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "audio.") {
			mediaPath = filepath.Join(tmpDir, e.Name())
			break
		}
	}
	if mediaPath == "" {
		return "", 0, fmt.Errorf("yt-dlp output not found in tmp dir")
	}

	// Best-effort duration from yt-dlp metadata.
	duration := 0
	meta := exec.CommandContext(ctx, "yt-dlp", "--no-playlist", "--print", "%(duration)s", ytURL)
	if b, err := meta.Output(); err == nil {
		duration, _ = strconv.Atoi(strings.TrimSpace(string(b)))
	}
	return mediaPath, duration, nil
}

// transcribeAudioFile sends the local audio bytes to the AI router for STT,
// then asks Gemini to split the transcript into sentence-aligned segments
// with Thai translation. The audio file is read once and discarded; nothing
// is written to MinIO.
func (s *ShadowingService) transcribeAudioFile(ctx context.Context, mediaPath string, durationSec int) ([]ShadowingSegmentDTO, error) {
	audio, err := os.ReadFile(mediaPath)
	if err != nil {
		return nil, fmt.Errorf("read media: %w", err)
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("empty audio file")
	}

	mediaType := mediaMIME(mediaPath)
	stt, err := s.router.Transcribe(ctx, ai.STTRequest{
		AudioData: audio,
		Filename:  filepath.Base(mediaPath),
		MediaType: mediaType,
	})
	if err != nil {
		return nil, fmt.Errorf("stt: %w", err)
	}
	transcript := strings.TrimSpace(stt.Text)
	if transcript == "" {
		return nil, fmt.Errorf("empty stt transcript")
	}

	durationHint := durationSec
	if durationHint <= 0 {
		durationHint = 60
	}
	prompt := fmt.Sprintf(`You are preparing a parroto.app-style shadowing transcript for a Thai
learner. Below is the literal English transcript of the audio. Split it into
8-25 SHORT sentence-aligned segments. Distribute start/end times in seconds
across the total media duration (~%d s). Translate each segment into natural
Thai for the cache.

TRANSCRIPT:
"""
%s
"""

Respond in STRICT JSON ONLY, no markdown fence, no commentary:
{
  "segments": [
    {"index": 0, "start_time": 0.0, "end_time": 4.2,
     "text": "First sentence verbatim from the transcript.",
     "thai_translation": "คำแปลเป็นภาษาไทย"}
  ]
}`, durationHint, transcript)

	chat, err := s.router.Chat(ctx, ai.ChatRequest{
		SystemPrompt: "You only output strict JSON. Never include markdown fences.",
		Messages:     []ai.ChatMessage{{Role: "user", Content: prompt}},
		UseCase:      "shadowing_transcript",
	})
	if err != nil {
		return nil, fmt.Errorf("segment chat: %w", err)
	}

	var parsed struct {
		Segments []struct {
			Index           int     `json:"index"`
			StartTime       float64 `json:"start_time"`
			EndTime         float64 `json:"end_time"`
			Text            string  `json:"text"`
			ThaiTranslation string  `json:"thai_translation"`
		} `json:"segments"`
	}
	if err := json.Unmarshal([]byte(stripCodeFences(chat.Content)), &parsed); err != nil {
		return nil, fmt.Errorf("parse segments: %w", err)
	}
	out := make([]ShadowingSegmentDTO, 0, len(parsed.Segments))
	for i, p := range parsed.Segments {
		text := strings.TrimSpace(p.Text)
		if text == "" {
			continue
		}
		idx := p.Index
		if idx == 0 && i > 0 {
			idx = i
		}
		out = append(out, ShadowingSegmentDTO{
			Index:           idx,
			StartTime:       p.StartTime,
			EndTime:         p.EndTime,
			Text:            text,
			ThaiTranslation: p.ThaiTranslation,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no segments returned")
	}
	return out, nil
}

func mediaMIME(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".m4a", ".mp4":
		return "audio/mp4"
	case ".webm":
		return "audio/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".opus":
		return "audio/ogg"
	case ".wav":
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}

// setTranscriptStatus updates transcript_status (and error_message) without
// touching the clip-level status / stream URL.
func (s *ShadowingService) setTranscriptStatus(ctx context.Context, clipID, status, errMsg string) {
	_, _ = s.db.Exec(ctx,
		`UPDATE shadowing_clips
		   SET transcript_status = $1, error_message = $2, updated_at = now()
		 WHERE id = $3`,
		status, errMsg, clipID)
}

func (s *ShadowingService) objectURL(bucket, key string) string {
	base := s.cfg.MinIO.PublicBase
	if base == "" {
		scheme := "http"
		if s.cfg.MinIO.UseSSL {
			scheme = "https"
		}
		base = fmt.Sprintf("%s://%s/%s", scheme, s.cfg.MinIO.Endpoint, bucket)
	}
	base = strings.TrimRight(base, "/")
	return base + "/" + strings.TrimLeft(key, "/")
}

// PresignedGet returns a short-lived browser-playable URL for the object.
// Returns "" when MinIO is unavailable or the object key is empty.
func (s *ShadowingService) PresignedGet(ctx context.Context, key string) string {
	if s.minioClient == nil || strings.TrimSpace(key) == "" {
		return ""
	}
	u, err := s.minioClient.PresignedGetObject(ctx, s.cfg.MinIO.Bucket, key, 2*time.Hour, nil)
	if err != nil {
		return ""
	}
	return u.String()
}

// OpenObjectStream returns a reader for the MinIO object plus its content-type
// and total size so the handler can set HTTP headers correctly. Supports a
// Range header so the browser <video> element can seek the file.
// The caller must Close the returned reader.
func (s *ShadowingService) OpenObjectStream(ctx context.Context, key, rangeHeader string) (io.ReadCloser, string, int64, error) {
	if s.minioClient == nil {
		return nil, "", 0, fmt.Errorf("minio not configured")
	}
	opts := minio.GetObjectOptions{}
	if rangeHeader != "" {
		var start, end int64
		clean := strings.TrimPrefix(rangeHeader, "bytes=")
		if i := strings.Index(clean, "-"); i >= 0 {
			fmt.Sscanf(clean[:i], "%d", &start)
			if rest := clean[i+1:]; rest != "" {
				fmt.Sscanf(rest, "%d", &end)
			}
		}
		if end > 0 {
			_ = opts.SetRange(start, end)
		} else if start > 0 {
			_ = opts.SetRange(start, 0)
		}
	}
	obj, err := s.minioClient.GetObject(ctx, s.cfg.MinIO.Bucket, key, opts)
	if err != nil {
		return nil, "", 0, err
	}
	stat, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		return nil, "", 0, err
	}
	ct := contentTypeOrDefault(stat.ContentType, "video/mp4")
	return obj, ct, stat.Size, nil
}

// LookupClipForOwner returns the clip row when (and only when) the user owns
// the clip. Used by stream/notes/etc. handlers to authorise access.
func (s *ShadowingService) LookupClipForOwner(ctx context.Context, userID, clipID string) (ShadowingClipDTO, error) {
	clip, err := s.getClip(ctx, clipID)
	if err != nil {
		return ShadowingClipDTO{}, err
	}
	if clip.UserID != userID {
		return ShadowingClipDTO{}, fmt.Errorf("forbidden")
	}
	return clip, nil
}

// ScoreRecording asks STT to transcribe the user's audio, compares it to the
// segment text with the deterministic evaluator, and persists score + feedback.
// Returns (score 0..1, feedback Thai text).
func (s *ShadowingService) ScoreRecording(ctx context.Context, userID, clipID, recordingID string) (float64, string, error) {
	var (
		audioKey  string
		segmentID string
		segText   string
	)
	err := s.db.QueryRow(ctx, `
		SELECT r.audio_object_key, COALESCE(r.segment_id::text,''), COALESCE(seg.text,'')
		FROM shadowing_recordings r
		LEFT JOIN shadowing_segments seg ON seg.id = r.segment_id
		WHERE r.id = $1 AND r.user_id = $2 AND r.clip_id = $3`,
		recordingID, userID, clipID).Scan(&audioKey, &segmentID, &segText)
	if err != nil {
		return 0, "", fmt.Errorf("recording not found: %w", err)
	}
	if segText == "" {
		return 0, "", fmt.Errorf("segment text unavailable")
	}
	if s.minioClient == nil {
		return 0, "", fmt.Errorf("minio not configured")
	}
	obj, err := s.minioClient.GetObject(ctx, s.cfg.MinIO.Bucket, audioKey, minio.GetObjectOptions{})
	if err != nil {
		return 0, "", err
	}
	defer obj.Close()
	audioBytes, err := io.ReadAll(obj)
	if err != nil {
		return 0, "", err
	}
	stt, err := s.router.Transcribe(ctx, ai.STTRequest{AudioData: audioBytes, Filename: "recording.webm", MediaType: "audio/webm"})
	if err != nil {
		return 0, "", err
	}
	eval := EvaluateAnswer(segText, stt.Text)
	feedback := "ฝึกพูดได้ดีมากครับ!"
	if !eval.IsCorrect {
		if eval.Score >= 0.6 {
			feedback = "เกือบเป๊ะแล้ว! ลองเน้นคำที่ขาดให้ชัดอีกหน่อย"
		} else {
			feedback = "ลองพูดช้า ๆ ทีละคำตามประโยคนะครับ"
		}
	}
	_, _ = s.db.Exec(ctx,
		`UPDATE shadowing_recordings SET score = $1, ai_feedback = $2 WHERE id = $3`,
		eval.Score, feedback, recordingID)
	return eval.Score, feedback, nil
}

// TranslateSegment returns the cached Thai translation from the DB. The
// frontend "translate" button uses this so we never re-call Gemini just to
// re-render the cached translation.
func (s *ShadowingService) TranslateSegment(ctx context.Context, userID, clipID, segmentID string) (string, string, error) {
	var en, th string
	err := s.db.QueryRow(ctx, `
		SELECT seg.text, COALESCE(seg.thai_translation,'')
		FROM shadowing_segments seg
		JOIN shadowing_clips c ON c.id = seg.clip_id
		WHERE seg.id = $1 AND seg.clip_id = $2 AND c.user_id = $3`,
		segmentID, clipID, userID).Scan(&en, &th)
	return en, th, err
}

func (s *ShadowingService) replaceSegments(ctx context.Context, clipID string, segs []ShadowingSegmentDTO) {
	_, _ = s.db.Exec(ctx, `DELETE FROM shadowing_segments WHERE clip_id = $1`, clipID)
	for _, seg := range segs {
		_, _ = s.db.Exec(ctx,
			`INSERT INTO shadowing_segments (id, clip_id, idx, start_time, end_time, text, thai_translation)
			 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			uuid.New().String(), clipID, seg.Index, seg.StartTime, seg.EndTime, seg.Text, seg.ThaiTranslation)
	}
}

func (s *ShadowingService) getClip(ctx context.Context, clipID string) (ShadowingClipDTO, error) {
	var c ShadowingClipDTO
	err := s.db.QueryRow(ctx,
		`SELECT c.id::text, c.user_id::text, c.youtube_url, COALESCE(c.youtube_id,''), COALESCE(c.title,''),
		        COALESCE(c.thumbnail_url,''), COALESCE(c.minio_object_key,''), COALESCE(c.stream_url,''),
		        COALESCE(c.duration_seconds,0), COALESCE(c.status,'pending'),
		        COALESCE(c.video_status,'pending'), COALESCE(c.transcript_status,'pending'),
		        COALESCE(c.error_message,''),
		        COALESCE(c.folder_id::text,''), COALESCE(c.is_completed,false), c.watched_at,
		        COALESCE(p.current_segment_index,0), COALESCE(p.last_watched_time,0),
		        c.created_at, c.updated_at
		 FROM shadowing_clips c
		 LEFT JOIN shadowing_progress p ON p.clip_id = c.id AND p.user_id = c.user_id
		 WHERE c.id = $1`,
		clipID).Scan(&c.ID, &c.UserID, &c.YouTubeURL, &c.YouTubeID, &c.Title, &c.ThumbnailURL,
		&c.MinIOObjectKey, &c.StreamURL, &c.DurationSeconds, &c.Status,
		&c.VideoStatus, &c.TranscriptStatus, &c.ErrorMessage,
		&c.FolderID, &c.IsCompleted, &c.WatchedAt,
		&c.LastSegmentIdx, &c.LastWatchedTime,
		&c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (s *ShadowingService) listSegments(ctx context.Context, clipID string) ([]ShadowingSegmentDTO, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id::text, idx, start_time, end_time, text, COALESCE(thai_translation,''), COALESCE(ipa,'')
		 FROM shadowing_segments WHERE clip_id = $1 ORDER BY idx ASC`, clipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ShadowingSegmentDTO
	for rows.Next() {
		var s ShadowingSegmentDTO
		if err := rows.Scan(&s.ID, &s.Index, &s.StartTime, &s.EndTime, &s.Text, &s.ThaiTranslation, &s.IPA); err == nil {
			out = append(out, s)
		}
	}
	return out, rows.Err()
}

func (s *ShadowingService) getProgress(ctx context.Context, userID, clipID string) ShadowingProgressDTO {
	out := ShadowingProgressDTO{CurrentSegmentIndex: 0}
	var completedRaw string
	_ = s.db.QueryRow(ctx,
		`SELECT COALESCE(current_segment_index,0), COALESCE(last_watched_time,0),
		        COALESCE(completed_segments,'[]'::jsonb)::text
		 FROM shadowing_progress WHERE user_id = $1 AND clip_id = $2`,
		userID, clipID).Scan(&out.CurrentSegmentIndex, &out.LastWatchedTime, &completedRaw)
	if completedRaw != "" {
		_ = json.Unmarshal([]byte(completedRaw), &out.CompletedSegments)
	}
	return out
}

// ---- tiny helpers ----

func contentTypeOrDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func nullableUUID(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}

func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// drop first line
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(s)
}

// byteReader wraps a byte slice as an io.Reader without taking an extra alloc
// when callers already hold the data in memory.
type bytesReader struct {
	b   []byte
	off int
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.off >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.off:])
	r.off += n
	return n, nil
}

func byteReader(b []byte) io.Reader { return &bytesReader{b: b} }
