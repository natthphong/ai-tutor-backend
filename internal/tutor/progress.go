package tutor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

func (s *Service) ensureUnitProgress(ctx context.Context, userID string, unitID int) {
	_, _ = s.db.Exec(ctx, `INSERT INTO user_unit_progress (id, user_id, unit_id, status, current_step)
		VALUES ($1, $2, $3, 'in_progress', 'intro')
		ON CONFLICT (user_id, unit_id) DO NOTHING`, uuid.New().String(), userID, unitID)
}

func (s *Service) updateUnitSkillScore(ctx context.Context, userID string, unitID int, skill string, score float64) {
	score = normalizeScore(score)
	s.ensureUnitProgress(ctx, userID, unitID)
	var query string
	switch skill {
	case "speaking":
		query = `UPDATE user_unit_progress
			SET speaking_score = GREATEST(COALESCE(speaking_score,0), $1),
			    attempt_count = attempt_count + 1,
			    mastery_score = LEAST(1.0, (COALESCE(listening_score,0) + GREATEST(COALESCE(speaking_score,0), $1) + COALESCE(reading_score,0)) / 3.0),
			    updated_at = now()
			WHERE user_id = $2 AND unit_id = $3`
	case "reading":
		query = `UPDATE user_unit_progress
			SET reading_score = GREATEST(COALESCE(reading_score,0), $1),
			    attempt_count = attempt_count + 1,
			    mastery_score = LEAST(1.0, (COALESCE(listening_score,0) + COALESCE(speaking_score,0) + GREATEST(COALESCE(reading_score,0), $1)) / 3.0),
			    updated_at = now()
			WHERE user_id = $2 AND unit_id = $3`
	default:
		query = `UPDATE user_unit_progress
			SET listening_score = GREATEST(COALESCE(listening_score,0), $1),
			    attempt_count = attempt_count + 1,
			    mastery_score = LEAST(1.0, (GREATEST(COALESCE(listening_score,0), $1) + COALESCE(speaking_score,0) + COALESCE(reading_score,0)) / 3.0),
			    updated_at = now()
			WHERE user_id = $2 AND unit_id = $3`
	}
	_, _ = s.db.Exec(ctx, query, score, userID, unitID)
}

func (s *Service) updateSessionPractice(ctx context.Context, sessionID string, mode string, nextAction string, itemID string, state map[string]interface{}) {
	if state == nil {
		state = map[string]interface{}{}
	}
	stateJSON, _ := json.Marshal(state)
	if itemID != "" {
		if _, err := s.db.Exec(ctx, `UPDATE tutor_sessions SET mode = $1, current_action = $2, current_item_id = $3, resume_state = $4::jsonb, updated_at = now() WHERE id = $5`,
			mode, nextAction, itemID, string(stateJSON), sessionID); err != nil {
			_, _ = s.db.Exec(ctx, `UPDATE tutor_sessions SET mode = $1, current_action = $2, current_item_id = $3 WHERE id = $4`,
				mode, nextAction, itemID, sessionID)
		}
		return
	}
	if _, err := s.db.Exec(ctx, `UPDATE tutor_sessions SET mode = $1, current_action = $2, resume_state = $3::jsonb, updated_at = now() WHERE id = $4`,
		mode, nextAction, string(stateJSON), sessionID); err != nil {
		_, _ = s.db.Exec(ctx, `UPDATE tutor_sessions SET mode = $1, current_action = $2 WHERE id = $3`, mode, nextAction, sessionID)
	}
}

func (s *Service) clearSessionItem(ctx context.Context, sessionID string, mode string, nextAction string, state map[string]interface{}) {
	if state == nil {
		state = map[string]interface{}{}
	}
	stateJSON, _ := json.Marshal(state)
	if _, err := s.db.Exec(ctx, `UPDATE tutor_sessions SET mode = $1, current_action = $2, current_item_id = NULL, resume_state = $3::jsonb, updated_at = now() WHERE id = $4`,
		mode, nextAction, string(stateJSON), sessionID); err != nil {
		_, _ = s.db.Exec(ctx, `UPDATE tutor_sessions SET mode = $1, current_action = $2, current_item_id = NULL WHERE id = $3`, mode, nextAction, sessionID)
	}
}

func (s *Service) getCurrentOrNextListening(ctx context.Context, sessionID string, unitID int) (string, string) {
	var itemID, content string
	err := s.db.QueryRow(ctx, `
		SELECT li.id::text, li.content
		FROM tutor_sessions ts
		JOIN lesson_items li ON li.id = ts.current_item_id
		WHERE ts.id = $1 AND li.unit_id = $2 AND li.item_type IN ('listening_sentence','example_sentence')`,
		sessionID, unitID,
	).Scan(&itemID, &content)
	if err == nil && content != "" {
		return itemID, content
	}
	err = s.db.QueryRow(ctx, `
		SELECT id::text, content
		FROM lesson_items
		WHERE unit_id = $1 AND item_type = 'listening_sentence' AND length(content) BETWEEN 8 AND 180
		ORDER BY sort_order ASC
		LIMIT 1`, unitID).Scan(&itemID, &content)
	if err != nil {
		_ = s.db.QueryRow(ctx, `
			SELECT id::text, content
			FROM lesson_items
			WHERE unit_id = $1 AND item_type = 'example_sentence' AND length(content) BETWEEN 8 AND 180
			ORDER BY sort_order ASC
			LIMIT 1`, unitID).Scan(&itemID, &content)
	}
	if content != "" {
		s.updateSessionPractice(ctx, sessionID, "listening", "start_listening", itemID, map[string]interface{}{
			"lessonItemId": itemID,
			"targetText":   content,
		})
		return itemID, content
	}
	content = firstSentence(s.lessonContentExcerpt(ctx, unitID))
	if content != "" {
		s.updateSessionPractice(ctx, sessionID, "listening", "start_listening", "", map[string]interface{}{
			"targetText": content,
		})
	}
	return itemID, content
}

func (s *Service) nextHintLevel(ctx context.Context, sessionID string) int {
	var failCount int
	_ = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM listening_attempts WHERE session_id = $1 AND is_correct = false`, sessionID).Scan(&failCount)
	if failCount <= 0 {
		return 1
	}
	if failCount == 1 {
		return 2
	}
	return 3
}

func (s *Service) buildWeaknessReview(ctx context.Context, userID string, unitID int) string {
	rows, err := s.db.Query(ctx, `
		SELECT weakness_type, COALESCE(detail,''), COALESCE(example_correct,'')
		FROM weaknesses
		WHERE user_id = $1 AND ($2 = 0 OR unit_id = $2) AND resolved = false
		ORDER BY next_due_at NULLS LAST, created_at DESC
		LIMIT 3`, userID, unitID)
	if err != nil || rows == nil {
		return ""
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var weaknessType, detail, example string
		_ = rows.Scan(&weaknessType, &detail, &example)
		line := "- " + weaknessType
		if detail != "" {
			line += ": " + detail
		}
		if example != "" {
			line += " เช่น " + example
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}
	return "จุดที่ควรทวนตอนนี้:\n" + stringsJoin(lines, "\n")
}

func stringsJoin(values []string, sep string) string {
	result := ""
	for i, value := range values {
		if i > 0 {
			result += sep
		}
		result += value
	}
	return result
}

// LessonProgressDTO is the structured progress payload returned to clients
// resuming a lesson.
type LessonProgressDTO struct {
	UserID         string  `json:"userId"`
	UnitID         int     `json:"unitId"`
	Status         string  `json:"status"`
	CurrentStep    string  `json:"currentStep"`
	MasteryScore   float64 `json:"masteryScore"`
	ListeningScore float64 `json:"listeningScore"`
	SpeakingScore  float64 `json:"speakingScore"`
	ReadingScore   float64 `json:"readingScore"`
	AttemptCount   int     `json:"attemptCount"`
	ActiveSession  string  `json:"activeSessionId,omitempty"`
}

// InsertLessonMessage stores a message into tutor_messages. Public wrapper
// around the existing private helper, used by the lesson-chat endpoints.
func (s *Service) InsertLessonMessage(ctx context.Context, sessionID, userID string, unitID int, role, content, contentTh, messageType string, metadata interface{}) {
	if sessionID == "" {
		// Find or create a fallback session for this lesson so we satisfy the
		// FK without forcing the caller to know session_id.
		_ = s.db.QueryRow(ctx,
			`SELECT id::text FROM tutor_sessions WHERE user_id = $1 AND unit_id = $2 AND status = 'active' ORDER BY started_at DESC LIMIT 1`,
			userID, unitID).Scan(&sessionID)
	}
	if sessionID == "" {
		sessionID = uuid.New().String()
		_, _ = s.db.Exec(ctx,
			`INSERT INTO tutor_sessions (id, user_id, unit_id, mode, status) VALUES ($1,$2,$3,'mixed','active') ON CONFLICT DO NOTHING`,
			sessionID, userID, unitID)
	}
	s.insertTutorMessage(ctx, sessionID, userID, unitID, role, content, contentTh, messageType, "client_persist", 0, metadata)
}

// GetLessonProgress returns the persisted progress row for (user, unit),
// or a zero row if none exists yet.
func (s *Service) GetLessonProgress(ctx context.Context, userID string, unitID int) (LessonProgressDTO, error) {
	out := LessonProgressDTO{UserID: userID, UnitID: unitID, Status: "not_started", CurrentStep: "intro"}
	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(status,'not_started'), COALESCE(current_step,'intro'),
		       COALESCE(mastery_score,0), COALESCE(listening_score,0),
		       COALESCE(speaking_score,0), COALESCE(reading_score,0),
		       COALESCE(attempt_count,0)
		FROM user_unit_progress WHERE user_id = $1 AND unit_id = $2`,
		userID, unitID,
	).Scan(&out.Status, &out.CurrentStep, &out.MasteryScore, &out.ListeningScore, &out.SpeakingScore, &out.ReadingScore, &out.AttemptCount)
	_ = s.db.QueryRow(ctx, `SELECT id::text FROM tutor_sessions WHERE user_id = $1 AND unit_id = $2 AND status = 'active' ORDER BY started_at DESC LIMIT 1`,
		userID, unitID).Scan(&out.ActiveSession)
	return out, nil
}

// UpsertLessonProgress writes progress fields from the client. Empty fields
// are ignored (no clobbering of existing values).
func (s *Service) UpsertLessonProgress(ctx context.Context, userID string, unitID int, currentStep, status string, listening, speaking, reading float64) (LessonProgressDTO, error) {
	s.ensureUnitProgress(ctx, userID, unitID)
	if currentStep != "" {
		_, _ = s.db.Exec(ctx, `UPDATE user_unit_progress SET current_step = $1, updated_at = now() WHERE user_id = $2 AND unit_id = $3`, currentStep, userID, unitID)
	}
	if status != "" {
		_, _ = s.db.Exec(ctx, `UPDATE user_unit_progress SET status = $1, updated_at = now() WHERE user_id = $2 AND unit_id = $3`, status, userID, unitID)
	}
	if listening > 0 {
		s.updateUnitSkillScore(ctx, userID, unitID, "listening", listening)
	}
	if speaking > 0 {
		s.updateUnitSkillScore(ctx, userID, unitID, "speaking", speaking)
	}
	if reading > 0 {
		s.updateUnitSkillScore(ctx, userID, unitID, "reading", reading)
	}
	return s.GetLessonProgress(ctx, userID, unitID)
}

func (s *Service) GetUnitHistory(ctx context.Context, userID string, unitID int) ([]TutorMessageDTO, error) {
	rows, err := s.db.Query(ctx, `
		SELECT role, COALESCE(content,''), COALESCE(content_th,''), COALESCE(message_type,'text')
		FROM tutor_messages
		WHERE user_id = $1 AND unit_id = $2
		ORDER BY created_at ASC
		LIMIT 200`, userID, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var messages []TutorMessageDTO
	for rows.Next() {
		var m TutorMessageDTO
		if err := rows.Scan(&m.Role, &m.Content, &m.ContentTh, &m.Type); err == nil {
			messages = append(messages, m)
		}
	}
	return messages, rows.Err()
}

func (s *Service) GetDueFlashcards(ctx context.Context, userID string, limit int) ([]FlashcardReviewItem, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.db.Query(ctx, `
		SELECT id::text, front, back, COALESCE(example,''), COALESCE(example_th,''), card_type, COALESCE(mastery_score,0)
		FROM tutor_flashcards
		WHERE user_id = $1 AND next_due_at <= now()
		ORDER BY next_due_at ASC, created_at ASC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cards []FlashcardReviewItem
	for rows.Next() {
		var c FlashcardReviewItem
		if err := rows.Scan(&c.ID, &c.Front, &c.Back, &c.Example, &c.ExampleTh, &c.CardType, &c.MasteryScore); err == nil {
			cards = append(cards, c)
		}
	}
	return cards, rows.Err()
}

func (s *Service) getStoredReadingPassage(ctx context.Context, sessionID string, unitID int, grammarFocus, level, unitTitle string) string {
	var resumeRaw string
	_ = s.db.QueryRow(ctx, `SELECT COALESCE(resume_state,'{}'::jsonb)::text FROM tutor_sessions WHERE id = $1`, sessionID).Scan(&resumeRaw)
	var state map[string]interface{}
	_ = json.Unmarshal([]byte(resumeRaw), &state)
	if state != nil {
		fresh, _ := state["freshPrompt"].(bool)
		if !fresh {
			if passage, ok := state["passage"].(string); ok && passage != "" {
				return passage
			}
		}
	}
	passage, err := s.generateReadingPassage(ctx, grammarFocus, level, unitTitle)
	if err != nil || passage == "" {
		passage = s.lessonContentExcerpt(ctx, unitID)
	}
	if passage == "" {
		passage = fmt.Sprintf("This unit practices %s. Read the examples carefully and translate the meaning.", grammarFocus)
	}
	s.updateSessionPractice(ctx, sessionID, "reading", "start_reading", "", map[string]interface{}{
		"passage": passage,
		"pattern": grammarFocus,
	})
	return passage
}

func (s *Service) lessonContentExcerpt(ctx context.Context, unitID int) string {
	var content string
	_ = s.db.QueryRow(ctx, `
		SELECT content FROM lesson_items
		WHERE unit_id = $1 AND item_type IN ('reading_passage','grammar_explanation','example_sentence')
		ORDER BY CASE item_type WHEN 'reading_passage' THEN 1 WHEN 'grammar_explanation' THEN 2 ELSE 3 END, sort_order
		LIMIT 1`, unitID).Scan(&content)
	if len(content) > 500 {
		return content[:500]
	}
	if content == "" {
		_ = s.db.QueryRow(ctx, `SELECT COALESCE(raw_content, summary, title, '') FROM lesson_units WHERE id = $1`, unitID).Scan(&content)
		if len(content) > 500 {
			return content[:500]
		}
	}
	return content
}
