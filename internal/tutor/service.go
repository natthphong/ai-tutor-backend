package tutor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/internal/ai"
	"go.uber.org/zap"
)

type Service struct {
	db     *pgxpool.Pool
	router *ai.Router
	logger *zap.Logger
}

func NewService(db *pgxpool.Pool, router *ai.Router) *Service {
	return &Service{db: db, router: router, logger: zap.L()}
}

// EnsureUser creates or gets a user by LINE ID
func (s *Service) EnsureUser(ctx context.Context, lineUserID string, displayName string) (string, error) {
	var userID string
	err := s.db.QueryRow(ctx,
		`SELECT id FROM tutor_users WHERE line_user_id = $1`, lineUserID).Scan(&userID)
	if err == nil {
		s.db.Exec(ctx, `UPDATE tutor_users SET last_active_at = now(), display_name = COALESCE(NULLIF($2,''), display_name) WHERE id = $1`, userID, displayName)
		return userID, nil
	}
	userID = uuid.New().String()
	_, err = s.db.Exec(ctx,
		`INSERT INTO tutor_users (id, line_user_id, display_name, last_active_at) VALUES ($1, $2, $3, now())`,
		userID, lineUserID, displayName)
	return userID, err
}

// GetDueItems returns counts of items due for review
func (s *Service) GetDueItems(ctx context.Context, userID string) (DueItems, error) {
	var d DueItems
	s.db.QueryRow(ctx, `SELECT COUNT(*) FROM tutor_flashcards WHERE user_id = $1 AND next_due_at <= now()`, userID).Scan(&d.VocabularyDueCount)
	s.db.QueryRow(ctx, `SELECT COUNT(*) FROM weaknesses WHERE user_id = $1 AND next_due_at <= now() AND resolved = false`, userID).Scan(&d.WeaknessDueCount)
	s.db.QueryRow(ctx, `SELECT COUNT(*) FROM user_unit_progress WHERE user_id = $1 AND status = 'review_due' AND next_due_at <= now()`, userID).Scan(&d.UnitReviewDueCount)
	return d, nil
}

// StartSession creates a new tutor session and decides next action
func (s *Service) StartSession(ctx context.Context, userID string, preferredMode string) (map[string]interface{}, error) {
	dueItems, _ := s.GetDueItems(ctx, userID)
	var currentUnitID int
	var currentStep, unitStatus string
	err := s.db.QueryRow(ctx, `SELECT current_unit_id FROM tutor_users WHERE id = $1`, userID).Scan(&currentUnitID)
	if err != nil {
		currentUnitID = 1
	}
	err = s.db.QueryRow(ctx, `SELECT COALESCE(current_step,'intro'), COALESCE(status,'not_started') FROM user_unit_progress WHERE user_id = $1 AND unit_id = $2`, userID, currentUnitID).Scan(&currentStep, &unitStatus)
	if err != nil {
		currentStep = "intro"
		unitStatus = "not_started"
	}

	var weaknessCount int
	s.db.QueryRow(ctx, `SELECT COUNT(*) FROM weaknesses WHERE user_id = $1 AND resolved = false`, userID).Scan(&weaknessCount)

	decision := DecideNextAction(DecisionInput{
		UserID: userID, CurrentUnitID: currentUnitID, DueItems: dueItems,
		RecentWeaknesses: weaknessCount, PreferredMode: preferredMode,
		CurrentStep: currentStep, UnitStatus: unitStatus, WeaknessThreshold: 5,
	})

	sessionID := uuid.New().String()
	_, err = s.db.Exec(ctx,
		`INSERT INTO tutor_sessions (id, user_id, unit_id, mode, status, current_action) VALUES ($1, $2, $3, $4, 'active', $5)`,
		sessionID, userID, currentUnitID, decision.Mode, decision.Action)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	if unitStatus == "not_started" {
		s.db.Exec(ctx, `INSERT INTO user_unit_progress (id, user_id, unit_id, status, current_step) VALUES ($1, $2, $3, 'in_progress', 'intro') ON CONFLICT (user_id, unit_id) DO UPDATE SET status = 'in_progress'`,
			uuid.New().String(), userID, currentUnitID)
	}

	var unitTitle string
	s.db.QueryRow(ctx, `SELECT COALESCE(title,'') FROM lesson_units WHERE id = $1`, currentUnitID).Scan(&unitTitle)

	// Store assistant opening message
	msgID := uuid.New().String()
	s.db.Exec(ctx, `INSERT INTO tutor_messages (id, session_id, user_id, role, content, content_th, message_type) VALUES ($1,$2,$3,'assistant',$4,$5,'text')`,
		msgID, sessionID, userID, "Let's study together! "+decision.Instruction, decision.Reason)

	return map[string]interface{}{
		"sessionId":  sessionID,
		"nextAction": decision.Action,
		"mode":       decision.Mode,
		"unit":       map[string]interface{}{"unitNo": currentUnitID, "title": unitTitle},
		"message": map[string]interface{}{
			"role": "assistant", "content": "Let's study together! " + decision.Instruction, "contentTh": decision.Reason,
		},
		"dueItems": dueItems,
	}, nil
}

// GetNextStep determines and executes the next tutor step
func (s *Service) GetNextStep(ctx context.Context, sessionID string, userID string) (map[string]interface{}, error) {
	var unitID int
	var mode, currentAction string
	err := s.db.QueryRow(ctx, `SELECT unit_id, mode, COALESCE(current_action,'') FROM tutor_sessions WHERE id = $1 AND user_id = $2`, sessionID, userID).Scan(&unitID, &mode, &currentAction)
	if err != nil {
		return nil, fmt.Errorf("session not found")
	}

	var currentStep string
	s.db.QueryRow(ctx, `SELECT COALESCE(current_step,'intro') FROM user_unit_progress WHERE user_id = $1 AND unit_id = $2`, userID, unitID).Scan(&currentStep)

	var unitTitle, grammarFocus, rawContent string
	s.db.QueryRow(ctx, `SELECT COALESCE(title,''), COALESCE(grammar_focus,''), COALESCE(raw_content,'') FROM lesson_units WHERE id = $1`, unitID).Scan(&unitTitle, &grammarFocus, &rawContent)

	result := map[string]interface{}{"sessionId": sessionID, "unitId": unitID, "mode": mode}

	switch currentStep {
	case "intro", "grammar_explanation":
		resp, err := s.explainGrammar(ctx, unitTitle, grammarFocus, rawContent, userID, sessionID)
		if err != nil {
			result["nextAction"] = "start_listening"
			result["instruction"] = "มาฝึกฟังกันครับ"
		} else {
			result["explanation"] = resp
			result["nextAction"] = "grammar_explained"
		}
		s.updateStep(ctx, userID, unitID, "listening_practice")

	case "listening_practice":
		sentence := s.getListeningSentence(ctx, unitID)
		result["nextAction"] = "start_listening"
		result["mode"] = "listening"
		result["instruction"] = "ฟังแล้วพิมพ์สิ่งที่ได้ยินครับ"
		result["targetHidden"] = true
		if sentence != "" {
			audioData, provider, err := s.router.Synthesize(ctx, ai.TTSRequest{Text: sentence, VoiceStyle: "friendly English teacher"})
			if err == nil {
				key := s.saveAudio(ctx, audioData, userID, sessionID)
				result["audioUrl"] = "/api/v1/files?key=" + key
				result["audioProvider"] = provider
			}
		}

	case "speaking_practice":
		result["nextAction"] = "start_speaking"
		result["mode"] = "speaking"
		result["instruction"] = "ลองพูดตาม pattern ที่เรียนครับ"
		result["pattern"] = grammarFocus
		s.updateStep(ctx, userID, unitID, "reading_practice")

	case "reading_practice":
		passage, _ := s.generateReadingPassage(ctx, grammarFocus, "A1", unitTitle)
		result["nextAction"] = "start_reading"
		result["mode"] = "reading"
		result["instruction"] = "อ่านแล้วลองแปลเป็นภาษาไทยครับ"
		result["passage"] = passage
		s.updateStep(ctx, userID, unitID, "mini_quiz")

	case "mini_quiz":
		result["nextAction"] = "review_summary"
		result["instruction"] = "สรุปสิ่งที่เรียนในบทนี้"
		s.updateStep(ctx, userID, unitID, "review_summary")

	case "review_summary", "schedule_review":
		s.completeUnit(ctx, userID, unitID)
		result["nextAction"] = "unit_completed"
		result["instruction"] = "เรียนจบบทนี้แล้วครับ!"
	}

	return result, nil
}

// EvaluateListening evaluates a listening answer
func (s *Service) EvaluateListening(ctx context.Context, sessionID, userID, lessonItemID, answer string) (map[string]interface{}, error) {
	var unitID int
	s.db.QueryRow(ctx, `SELECT unit_id FROM tutor_sessions WHERE id = $1`, sessionID).Scan(&unitID)

	targetText := s.getListeningSentence(ctx, unitID)
	if targetText == "" {
		targetText = "She is on her way to work."
	}

	var unitTitle string
	s.db.QueryRow(ctx, `SELECT COALESCE(title,'') FROM lesson_units WHERE id = $1`, unitID).Scan(&unitTitle)

	// Use AI to evaluate
	prompt := BuildListeningPrompt(targetText, answer, 0, unitTitle)
	resp, err := s.router.Chat(ctx, ai.ChatRequest{
		SystemPrompt: BuildTutorSystemPrompt(),
		Messages:     []ai.ChatMessage{{Role: "user", Content: prompt}},
		UseCase:      "listening_evaluation",
	})

	result := map[string]interface{}{}
	if err != nil {
		// Fallback: simple string comparison
		score := simpleCompare(targetText, answer)
		isCorrect := score >= 0.85
		result["isCorrect"] = isCorrect
		result["score"] = score
		result["feedbackTh"] = "ลองอีกครั้งนะครับ"
		if isCorrect {
			result["feedbackTh"] = "เก่งมากครับ! ถูกต้อง"
			result["nextAction"] = "next_step"
		} else {
			result["nextAction"] = "retry_listening"
			result["correction"] = targetText
		}
	} else {
		json.Unmarshal([]byte(resp.Content), &result)
	}

	// Store attempt
	attemptID := uuid.New().String()
	score := 0.0
	if s, ok := result["score"].(float64); ok {
		score = s
	}
	isCorrect := score >= 0.85
	mistakesJSON, _ := json.Marshal(result["mistakes"])
	s.db.Exec(ctx, `INSERT INTO listening_attempts (id, user_id, session_id, unit_id, target_text, user_text, score, is_correct, mistakes) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		attemptID, userID, sessionID, unitID, targetText, answer, score, isCorrect, string(mistakesJSON))

	// Store weaknesses if any
	if mistakes, ok := result["mistakes"].([]interface{}); ok {
		for _, m := range mistakes {
			if mm, ok := m.(map[string]interface{}); ok {
				wID := uuid.New().String()
				wType, _ := mm["type"].(string)
				wDetail, _ := mm["detail"].(string)
				nextDue := CalculateNextDue(score, 0, 0)
				s.db.Exec(ctx, `INSERT INTO weaknesses (id, user_id, unit_id, source_type, source_id, weakness_type, detail, example_wrong, example_correct, next_due_at) VALUES ($1,$2,$3,'listening',$4,$5,$6,$7,$8,$9)`,
					wID, userID, unitID, attemptID, wType, wDetail, answer, targetText, nextDue)
			}
		}
	}

	if isCorrect {
		s.updateStep(ctx, userID, unitID, "speaking_practice")
	}
	return result, nil
}

// EvaluateSpeaking evaluates a speaking transcript
func (s *Service) EvaluateSpeaking(ctx context.Context, sessionID, userID, transcript string) (map[string]interface{}, error) {
	var unitID int
	s.db.QueryRow(ctx, `SELECT unit_id FROM tutor_sessions WHERE id = $1`, sessionID).Scan(&unitID)

	var unitTitle, grammarFocus string
	s.db.QueryRow(ctx, `SELECT COALESCE(title,''), COALESCE(grammar_focus,'') FROM lesson_units WHERE id = $1`, unitID).Scan(&unitTitle, &grammarFocus)

	prompt := BuildSpeakingCorrectionPrompt(grammarFocus, "Practice using this pattern in a sentence.", transcript, unitTitle)
	resp, err := s.router.Chat(ctx, ai.ChatRequest{
		SystemPrompt: BuildTutorSystemPrompt(),
		Messages:     []ai.ChatMessage{{Role: "user", Content: prompt}},
		UseCase:      "speaking_evaluation",
	})

	result := map[string]interface{}{"transcript": transcript}
	if err != nil {
		result["score"] = 0.7
		result["feedbackTh"] = "ลองพูดอีกครั้งนะครับ"
		result["nextAction"] = "retry_speaking"
	} else {
		json.Unmarshal([]byte(resp.Content), &result)
		result["transcript"] = transcript
	}

	score := 0.0
	if sc, ok := result["score"].(float64); ok {
		score = sc
	}

	attemptID := uuid.New().String()
	mistakesJSON, _ := json.Marshal(result["mistakes"])
	s.db.Exec(ctx, `INSERT INTO speaking_attempts (id, user_id, session_id, unit_id, transcript, target_pattern, score, feedback_th, mistakes) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		attemptID, userID, sessionID, unitID, transcript, grammarFocus, score, result["feedbackTh"], string(mistakesJSON))

	if score >= 0.85 {
		s.updateStep(ctx, userID, unitID, "reading_practice")
	}
	return result, nil
}

// EvaluateReading evaluates a reading translation
func (s *Service) EvaluateReading(ctx context.Context, sessionID, userID, lessonItemID, translation string) (map[string]interface{}, error) {
	var unitID int
	s.db.QueryRow(ctx, `SELECT unit_id FROM tutor_sessions WHERE id = $1`, sessionID).Scan(&unitID)

	var unitTitle string
	s.db.QueryRow(ctx, `SELECT COALESCE(title,'') FROM lesson_units WHERE id = $1`, unitID).Scan(&unitTitle)

	passage := "Sarah is in her car. She is on her way to work."
	prompt := BuildReadingEvaluationPrompt(passage, translation, unitTitle)
	resp, err := s.router.Chat(ctx, ai.ChatRequest{
		SystemPrompt: BuildTutorSystemPrompt(),
		Messages:     []ai.ChatMessage{{Role: "user", Content: prompt}},
		UseCase:      "reading_evaluation",
	})

	result := map[string]interface{}{}
	if err != nil {
		result["score"] = 0.8
		result["feedbackTh"] = "แปลได้ดีครับ"
		result["nextAction"] = "continue_reading"
	} else {
		json.Unmarshal([]byte(resp.Content), &result)
	}

	// Create flashcards from vocabulary
	createdCards := 0
	if vocab, ok := result["vocabulary"].([]interface{}); ok {
		for _, v := range vocab {
			if vm, ok := v.(map[string]interface{}); ok {
				front, _ := vm["word"].(string)
				back, _ := vm["meaningTh"].(string)
				example, _ := vm["example"].(string)
				exampleTh, _ := vm["exampleTh"].(string)
				if front != "" && back != "" {
					cardID := uuid.New().String()
					s.db.Exec(ctx, `INSERT INTO tutor_flashcards (id, user_id, unit_id, card_type, front, back, example, example_th, source_type, next_due_at) VALUES ($1,$2,$3,'vocabulary',$4,$5,$6,$7,'reading',now() + interval '1 day')`,
						cardID, userID, unitID, front, back, example, exampleTh)
					createdCards++
				}
			}
		}
	}
	result["createdFlashcards"] = createdCards

	score := 0.0
	if sc, ok := result["score"].(float64); ok {
		score = sc
	}
	attemptID := uuid.New().String()
	vocabJSON, _ := json.Marshal(result["vocabulary"])
	s.db.Exec(ctx, `INSERT INTO reading_attempts (id, user_id, session_id, unit_id, passage, user_translation, score, feedback_th, vocabulary) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		attemptID, userID, sessionID, unitID, passage, translation, score, result["feedbackTh"], string(vocabJSON))

	s.updateStep(ctx, userID, unitID, "mini_quiz")
	return result, nil
}

// ReviewFlashcard processes a flashcard review
func (s *Service) ReviewFlashcard(ctx context.Context, userID, flashcardID string, score float64) (map[string]interface{}, error) {
	var currentMastery float64
	var reviewCount, consecutiveCorrect int
	err := s.db.QueryRow(ctx, `SELECT mastery_score, review_count, consecutive_correct FROM tutor_flashcards WHERE id = $1 AND user_id = $2`, flashcardID, userID).Scan(&currentMastery, &reviewCount, &consecutiveCorrect)
	if err != nil {
		return nil, fmt.Errorf("flashcard not found")
	}

	newMastery := UpdateMasteryScore(currentMastery, score)
	newConsecutive := consecutiveCorrect
	if score >= 0.85 {
		newConsecutive++
	} else {
		newConsecutive = 0
	}
	nextDue := CalculateNextDue(score, reviewCount+1, newConsecutive)

	s.db.Exec(ctx, `UPDATE tutor_flashcards SET mastery_score = $1, review_count = review_count + 1, consecutive_correct = $2, next_due_at = $3, last_reviewed_at = now(), updated_at = now() WHERE id = $4`,
		newMastery, newConsecutive, nextDue, flashcardID)

	result := "pass"
	if score < 0.60 {
		result = "fail"
	}
	return map[string]interface{}{
		"result": result, "nextDueAt": nextDue.Format(time.RFC3339), "masteryScore": newMastery, "level": MasteryLevel(newMastery),
	}, nil
}

// GetProgress returns user progress data
func (s *Service) GetProgress(ctx context.Context, userID string) (map[string]interface{}, error) {
	var currentUnit int
	s.db.QueryRow(ctx, `SELECT current_unit_id FROM tutor_users WHERE id = $1`, userID).Scan(&currentUnit)
	var completedUnits int
	s.db.QueryRow(ctx, `SELECT COUNT(*) FROM user_unit_progress WHERE user_id = $1 AND status IN ('completed','mastered')`, userID).Scan(&completedUnits)
	dueItems, _ := s.GetDueItems(ctx, userID)

	var lScore, sScore, rScore float64
	s.db.QueryRow(ctx, `SELECT COALESCE(AVG(listening_score),0), COALESCE(AVG(speaking_score),0), COALESCE(AVG(reading_score),0) FROM user_unit_progress WHERE user_id = $1 AND status != 'not_started'`, userID).Scan(&lScore, &sScore, &rScore)

	rows, _ := s.db.Query(ctx, `SELECT weakness_type, COUNT(*) as cnt FROM weaknesses WHERE user_id = $1 AND resolved = false GROUP BY weakness_type ORDER BY cnt DESC LIMIT 5`, userID)
	var topWeaknesses []string
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var wt string
			var cnt int
			rows.Scan(&wt, &cnt)
			topWeaknesses = append(topWeaknesses, wt)
		}
	}

	var streak int
	s.db.QueryRow(ctx, `SELECT streak_count FROM tutor_users WHERE id = $1`, userID).Scan(&streak)

	return map[string]interface{}{
		"currentUnit": currentUnit, "completedUnits": completedUnits, "streak": streak,
		"dueToday": map[string]interface{}{"vocabulary": dueItems.VocabularyDueCount, "weakness": dueItems.WeaknessDueCount, "unit": dueItems.UnitReviewDueCount},
		"scores":   map[string]interface{}{"listening": lScore, "speaking": sScore, "reading": rScore},
		"topWeaknesses": topWeaknesses,
	}, nil
}

// --- Helper methods ---

func (s *Service) updateStep(ctx context.Context, userID string, unitID int, step string) {
	s.db.Exec(ctx, `UPDATE user_unit_progress SET current_step = $1, updated_at = now() WHERE user_id = $2 AND unit_id = $3`, step, userID, unitID)
}

func (s *Service) completeUnit(ctx context.Context, userID string, unitID int) {
	nextDue := time.Now().Add(7 * 24 * time.Hour)
	s.db.Exec(ctx, `UPDATE user_unit_progress SET status = 'completed', completed_at = now(), next_due_at = $1, updated_at = now() WHERE user_id = $2 AND unit_id = $3`, nextDue, userID, unitID)
	s.db.Exec(ctx, `UPDATE tutor_users SET current_unit_id = current_unit_id + 1, updated_at = now() WHERE id = $1`, userID)
}

func (s *Service) getListeningSentence(ctx context.Context, unitID int) string {
	var content string
	err := s.db.QueryRow(ctx, `SELECT content FROM lesson_items WHERE unit_id = $1 AND item_type = 'listening_sentence' ORDER BY RANDOM() LIMIT 1`, unitID).Scan(&content)
	if err != nil {
		s.db.QueryRow(ctx, `SELECT content FROM lesson_items WHERE unit_id = $1 AND item_type = 'example_sentence' ORDER BY RANDOM() LIMIT 1`, unitID).Scan(&content)
	}
	return content
}

func (s *Service) explainGrammar(ctx context.Context, unitTitle, grammarFocus, rawContent, userID, sessionID string) (map[string]interface{}, error) {
	if rawContent == "" {
		rawContent = unitTitle
	}
	truncated := rawContent
	if len(truncated) > 2000 {
		truncated = truncated[:2000]
	}
	prompt := BuildGrammarExplanationPrompt(unitTitle, grammarFocus, truncated)
	resp, err := s.router.Chat(ctx, ai.ChatRequest{
		SystemPrompt: BuildTutorSystemPrompt(),
		Messages:     []ai.ChatMessage{{Role: "user", Content: prompt}},
		UseCase:      "grammar_explanation",
	})
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	json.Unmarshal([]byte(resp.Content), &result)
	return result, nil
}

func (s *Service) generateReadingPassage(ctx context.Context, grammarFocus, level, unitTitle string) (string, error) {
	prompt := BuildReadingPassagePrompt(grammarFocus, level, unitTitle)
	resp, err := s.router.Chat(ctx, ai.ChatRequest{
		SystemPrompt: BuildTutorSystemPrompt(),
		Messages:     []ai.ChatMessage{{Role: "user", Content: prompt}},
		UseCase:      "reading_passage",
	})
	if err != nil {
		return "", err
	}
	var result map[string]interface{}
	json.Unmarshal([]byte(resp.Content), &result)
	if p, ok := result["passage"].(string); ok {
		return p, nil
	}
	return resp.Content, nil
}

func (s *Service) saveAudio(ctx context.Context, data []byte, userID, sessionID string) string {
	// For MVP, return a placeholder key. MinIO integration added in handler.
	return fmt.Sprintf("tts/%s/%s/%s.mp3", userID, sessionID, uuid.New().String())
}

func simpleCompare(target, answer string) float64 {
	t := strings.ToLower(strings.TrimSpace(target))
	a := strings.ToLower(strings.TrimSpace(answer))
	if t == a {
		return 1.0
	}
	tWords := strings.Fields(t)
	aWords := strings.Fields(a)
	if len(tWords) == 0 {
		return 0
	}
	matches := 0
	for _, tw := range tWords {
		for _, aw := range aWords {
			if tw == aw {
				matches++
				break
			}
		}
	}
	return float64(matches) / float64(len(tWords))
}
