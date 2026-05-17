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
	ID              string    `json:"id"`
	UserID          string    `json:"userId"`
	YouTubeURL      string    `json:"youtubeUrl"`
	YouTubeID       string    `json:"youtubeId"`
	Title           string    `json:"title"`
	ThumbnailURL    string    `json:"thumbnailUrl"`
	MinIOObjectKey  string    `json:"minioObjectKey,omitempty"`
	StreamURL       string    `json:"streamUrl"`
	ProxyStreamURL  string    `json:"proxyStreamUrl,omitempty"`
	DurationSeconds int       `json:"durationSeconds"`
	Status          string    `json:"status"`
	ErrorMessage    string    `json:"errorMessage,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
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

// CreateClip inserts the clip row and kicks off processing in a background
// goroutine. It returns immediately with status="pending" so the frontend can
// poll GetClip until status becomes "ready" or "failed".
func (s *ShadowingService) CreateClip(ctx context.Context, userID, youtubeURL string) (ShadowingClipDTO, error) {
	yid := ParseYouTubeID(youtubeURL)
	if yid == "" {
		return ShadowingClipDTO{}, fmt.Errorf("invalid YouTube URL")
	}
	id := uuid.New().String()
	thumb := fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", yid)
	_, err := s.db.Exec(ctx,
		`INSERT INTO shadowing_clips (id, user_id, youtube_url, youtube_id, thumbnail_url, status) VALUES ($1,$2,$3,$4,$5,'pending')`,
		id, userID, youtubeURL, yid, thumb)
	if err != nil {
		return ShadowingClipDTO{}, fmt.Errorf("insert clip: %w", err)
	}

	// Kick off background processing. We deliberately use a detached context
	// because the HTTP request's context will be cancelled when we respond.
	go s.processClipAsync(id, userID, youtubeURL, yid)

	return s.getClip(ctx, id)
}

// ListClips returns the user's shadowing clips, most recent first.
func (s *ShadowingService) ListClips(ctx context.Context, userID string, limit int) ([]ShadowingClipDTO, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := s.db.Query(ctx,
		`SELECT id::text, user_id::text, youtube_url, COALESCE(youtube_id,''), COALESCE(title,''),
		        COALESCE(thumbnail_url,''), COALESCE(minio_object_key,''), COALESCE(stream_url,''),
		        COALESCE(duration_seconds,0), COALESCE(status,'pending'), COALESCE(error_message,''),
		        created_at, updated_at
		 FROM shadowing_clips WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`,
		userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ShadowingClipDTO
	for rows.Next() {
		var c ShadowingClipDTO
		if err := rows.Scan(&c.ID, &c.UserID, &c.YouTubeURL, &c.YouTubeID, &c.Title,
			&c.ThumbnailURL, &c.MinIOObjectKey, &c.StreamURL, &c.DurationSeconds, &c.Status,
			&c.ErrorMessage, &c.CreatedAt, &c.UpdatedAt); err == nil {
			out = append(out, s.enrichClip(ctx, c))
		}
	}
	return out, rows.Err()
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
func (s *ShadowingService) enrichClip(ctx context.Context, c ShadowingClipDTO) ShadowingClipDTO {
	if c.MinIOObjectKey == "" {
		return c
	}
	if presigned := s.PresignedGet(ctx, c.MinIOObjectKey); presigned != "" {
		c.StreamURL = presigned
	}
	// Always expose the backend proxy URL so the frontend has a fallback when
	// the presigned URL can't be reached (private MinIO endpoint, CORS, etc.).
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

func (s *ShadowingService) processClipAsync(clipID, userID, youtubeURL, yid string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	s.setClipStatus(ctx, clipID, "processing", "")

	if s.localFallbackEnabled() {
		s.processClipFallback(ctx, clipID, yid)
		return
	}

	// Real path: try yt-dlp → MinIO → Gemini transcript.
	tmpDir, err := os.MkdirTemp("", "shadow-"+clipID+"-*")
	if err != nil {
		s.failClip(ctx, clipID, "tmpdir: "+err.Error())
		return
	}
	defer os.RemoveAll(tmpDir)

	mediaPath, title, duration, err := s.downloadYouTube(ctx, youtubeURL, tmpDir)
	if err != nil {
		s.logger.Warn("shadowing download failed; using fallback",
			zap.String("clipId", clipID), zap.Error(err))
		s.processClipFallback(ctx, clipID, yid)
		return
	}

	objectKey, streamURL, err := s.uploadToMinIO(ctx, clipID, mediaPath)
	if err != nil {
		s.failClip(ctx, clipID, "minio: "+err.Error())
		return
	}

	segs, err := s.transcribeWithGemini(ctx, mediaPath)
	if err != nil || len(segs) == 0 {
		s.logger.Warn("Gemini transcript failed; storing media without segments",
			zap.String("clipId", clipID), zap.Error(err))
	} else {
		s.replaceSegments(ctx, clipID, segs)
	}

	_, _ = s.db.Exec(ctx,
		`UPDATE shadowing_clips SET status = 'ready', minio_object_key = $1, stream_url = $2,
		   title = COALESCE(NULLIF(title,''), $3), duration_seconds = COALESCE(NULLIF(duration_seconds,0), $4),
		   error_message = '', updated_at = now() WHERE id = $5`,
		objectKey, streamURL, title, duration, clipID)
}

// processClipFallback creates canned segments so the UI works during dev / CI
// without yt-dlp or a Gemini key.
func (s *ShadowingService) processClipFallback(ctx context.Context, clipID, yid string) {
	streamURL := fmt.Sprintf("https://www.youtube.com/embed/%s", yid)
	title := fmt.Sprintf("YouTube clip %s", yid)
	segs := []ShadowingSegmentDTO{
		{Index: 0, StartTime: 0, EndTime: 4.2, Text: "Look at this city.", ThaiTranslation: "ดูเมืองนี้สิ"},
		{Index: 1, StartTime: 4.2, EndTime: 7.8, Text: "My name's Kiki and I'm a witch.", ThaiTranslation: "ฉันชื่อกิกิ และฉันเป็นแม่มด"},
		{Index: 2, StartTime: 7.8, EndTime: 12.0, Text: "I love flying through the sky.", ThaiTranslation: "ฉันรักการบินผ่านท้องฟ้า"},
	}
	s.replaceSegments(ctx, clipID, segs)
	_, _ = s.db.Exec(ctx,
		`UPDATE shadowing_clips SET status = 'ready', stream_url = $1, title = $2,
		   duration_seconds = 12, error_message = '(fallback) yt-dlp/Gemini unavailable',
		   updated_at = now() WHERE id = $3`,
		streamURL, title, clipID)
}

func (s *ShadowingService) setClipStatus(ctx context.Context, clipID, status, errMsg string) {
	_, _ = s.db.Exec(ctx,
		`UPDATE shadowing_clips SET status = $1, error_message = $2, updated_at = now() WHERE id = $3`,
		status, errMsg, clipID)
}

func (s *ShadowingService) failClip(ctx context.Context, clipID, msg string) {
	s.logger.Warn("shadowing clip failed", zap.String("clipId", clipID), zap.String("reason", msg))
	s.setClipStatus(ctx, clipID, "failed", msg)
}

func (s *ShadowingService) downloadYouTube(ctx context.Context, ytURL, tmpDir string) (string, string, int, error) {
	// Prefer yt-dlp if available. We pick mp4 up to 720p.
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return "", "", 0, fmt.Errorf("yt-dlp not installed")
	}
	out := filepath.Join(tmpDir, "video.mp4")
	cmd := exec.CommandContext(ctx, "yt-dlp",
		"-f", "bestvideo[height<=720][ext=mp4]+bestaudio[ext=m4a]/best[height<=720][ext=mp4]/best",
		"--merge-output-format", "mp4",
		"--no-playlist",
		"-o", out,
		ytURL,
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", "", 0, fmt.Errorf("yt-dlp run: %w", err)
	}
	// Get title + duration with --print
	meta := exec.CommandContext(ctx, "yt-dlp", "--no-playlist", "--print", "%(title)s\n%(duration)s", ytURL)
	b, err := meta.Output()
	title := ""
	duration := 0
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		if len(lines) >= 1 {
			title = lines[0]
		}
		if len(lines) >= 2 {
			duration, _ = strconv.Atoi(lines[1])
		}
	}
	return out, title, duration, nil
}

func (s *ShadowingService) uploadToMinIO(ctx context.Context, clipID, mediaPath string) (string, string, error) {
	if s.minioClient == nil {
		return "", "", fmt.Errorf("minio not configured")
	}
	bucket := s.cfg.MinIO.Bucket
	key := fmt.Sprintf("shadowing/clips/%s/video.mp4", clipID)
	f, err := os.Open(mediaPath)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return "", "", err
	}
	_, err = s.minioClient.PutObject(ctx, bucket, key, f, stat.Size(),
		minio.PutObjectOptions{ContentType: "video/mp4"})
	if err != nil {
		return "", "", err
	}
	return key, s.objectURL(bucket, key), nil
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

func (s *ShadowingService) transcribeWithGemini(ctx context.Context, mediaPath string) ([]ShadowingSegmentDTO, error) {
	// Step 1: get raw transcript via STT (Gemini or fallback).
	audio, err := os.ReadFile(mediaPath)
	if err != nil {
		return nil, fmt.Errorf("read media: %w", err)
	}
	resp, err := s.router.Transcribe(ctx, ai.STTRequest{
		AudioData: audio,
		Filename:  filepath.Base(mediaPath),
		MediaType: "video/mp4",
	})
	if err != nil || strings.TrimSpace(resp.Text) == "" {
		return nil, fmt.Errorf("stt failed: %v", err)
	}

	// Step 2: ask Gemini to (a) split into 8–15 short sentence segments aligned
	// to natural pauses, (b) estimate start/end times across the total media
	// duration, (c) include a Thai translation of each segment so the
	// translate-on-click flow can pull from the DB cache.
	prompt := fmt.Sprintf(`You are preparing a parroto.app-style shadowing transcript
for a Thai learner. The English transcript is given verbatim below. Split it
into 8-15 SHORT segments (one sentence each), distribute start/end times
evenly across the clip, and translate each segment into natural Thai.

TRANSCRIPT:
"""
%s
"""

Respond in strict JSON only, NO markdown fence:
{
  "segments": [
    {"index": 0, "start_time": 0.0, "end_time": 4.2,
     "text": "Look at this city, my name's Kiki and I'm a witch.",
     "thai_translation": "ดูเมืองนี้สิ ฉันชื่อกิกิ และฉันเป็นแม่มด"}
  ]
}`, resp.Text)

	chat, err := s.router.Chat(ctx, ai.ChatRequest{
		SystemPrompt: "You only output strict JSON.",
		Messages:     []ai.ChatMessage{{Role: "user", Content: prompt}},
		UseCase:      "shadowing_transcript",
	})
	if err != nil {
		return nil, err
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
		return nil, err
	}
	out := make([]ShadowingSegmentDTO, 0, len(parsed.Segments))
	for i, p := range parsed.Segments {
		idx := p.Index
		if idx == 0 && i > 0 {
			idx = i
		}
		out = append(out, ShadowingSegmentDTO{
			Index:           idx,
			StartTime:       p.StartTime,
			EndTime:         p.EndTime,
			Text:            p.Text,
			ThaiTranslation: p.ThaiTranslation,
		})
	}
	return out, nil
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
		`SELECT id::text, user_id::text, youtube_url, COALESCE(youtube_id,''), COALESCE(title,''),
		        COALESCE(thumbnail_url,''), COALESCE(minio_object_key,''), COALESCE(stream_url,''),
		        COALESCE(duration_seconds,0), COALESCE(status,'pending'), COALESCE(error_message,''),
		        created_at, updated_at
		 FROM shadowing_clips WHERE id = $1`,
		clipID).Scan(&c.ID, &c.UserID, &c.YouTubeURL, &c.YouTubeID, &c.Title, &c.ThumbnailURL,
		&c.MinIOObjectKey, &c.StreamURL, &c.DurationSeconds, &c.Status, &c.ErrorMessage, &c.CreatedAt, &c.UpdatedAt)
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
