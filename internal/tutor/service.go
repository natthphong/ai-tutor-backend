package tutor

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/config"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/internal/ai"
	"go.uber.org/zap"
)

type Service struct {
	db          *pgxpool.Pool
	router      *ai.Router
	minioClient *minio.Client
	cfg         *config.Config
	logger      *zap.Logger
}

func NewService(db *pgxpool.Pool, router *ai.Router, minioClient *minio.Client, cfg *config.Config) *Service {
	return &Service{db: db, router: router, minioClient: minioClient, cfg: cfg, logger: zap.L()}
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
	s.db.QueryRow(ctx, `SELECT COUNT(*) FROM user_unit_progress WHERE user_id = $1 AND status IN ('review_due','completed','mastered') AND next_due_at <= now()`, userID).Scan(&d.UnitReviewDueCount)
	return d, nil
}

// StartSession creates or resumes a tutor session and decides next action
func (s *Service) StartSession(ctx context.Context, userID string, preferredMode string) (map[string]interface{}, error) {
	dueItems, _ := s.GetDueItems(ctx, userID)
	var currentUnitID int
	var currentStep, unitStatus string
	err := s.db.QueryRow(ctx, `SELECT current_unit_id FROM tutor_users WHERE id = $1`, userID).Scan(&currentUnitID)
	if err != nil {
		currentUnitID = 1
	}
	if dueItems.UnitReviewDueCount > 0 {
		_ = s.db.QueryRow(ctx, `SELECT unit_id FROM user_unit_progress WHERE user_id = $1 AND status IN ('review_due','completed','mastered') AND next_due_at <= now() ORDER BY next_due_at ASC LIMIT 1`, userID).Scan(&currentUnitID)
	}
	err = s.db.QueryRow(ctx, `SELECT COALESCE(current_step,'intro'), COALESCE(status,'not_started') FROM user_unit_progress WHERE user_id = $1 AND unit_id = $2`, userID, currentUnitID).Scan(&currentStep, &unitStatus)
	if err != nil {
		currentStep = "intro"
		unitStatus = "not_started"
	}

	var unitTitle, grammarFocus string
	s.db.QueryRow(ctx, `SELECT COALESCE(title,''), COALESCE(grammar_focus,'') FROM lesson_units WHERE id = $1`, currentUnitID).Scan(&unitTitle, &grammarFocus)

	var weaknessCount int
	s.db.QueryRow(ctx, `SELECT COUNT(*) FROM weaknesses WHERE user_id = $1 AND resolved = false`, userID).Scan(&weaknessCount)

	weaknessThreshold := s.cfg.Tutor.WeaknessThreshold
	if weaknessThreshold <= 0 {
		weaknessThreshold = 5
	}
	decision := DecideNextAction(DecisionInput{
		UserID: userID, CurrentUnitID: currentUnitID, DueItems: dueItems,
		RecentWeaknesses: weaknessCount, PreferredMode: preferredMode,
		CurrentStep: currentStep, UnitStatus: unitStatus, WeaknessThreshold: weaknessThreshold,
	})

	// Try to find existing active session
	var sessionID string
	var mode, currentAction string
	err = s.db.QueryRow(ctx, `SELECT id, mode, current_action FROM tutor_sessions WHERE user_id = $1 AND unit_id = $2 AND status = 'active' ORDER BY created_at DESC LIMIT 1`, userID, currentUnitID).Scan(&sessionID, &mode, &currentAction)

	var messages []map[string]interface{}

	if err == nil {
		// Resume existing session
		decision.Mode = mode
		decision.Action = currentAction
		// Fetch the full per-unit history so the chat is restored after a refresh.
		rows, _ := s.db.Query(ctx, `SELECT role, content, content_th, message_type FROM tutor_messages WHERE user_id = $1 AND unit_id = $2 ORDER BY created_at DESC LIMIT 200`, userID, currentUnitID)
		defer rows.Close()
		var tempMsgs []map[string]interface{}
		for rows.Next() {
			var r, c, cTh, t string
			rows.Scan(&r, &c, &cTh, &t)
			tempMsgs = append([]map[string]interface{}{{"role": r, "content": c, "contentTh": cTh, "type": t}}, tempMsgs...)
		}
		messages = tempMsgs
		if len(messages) == 0 {
			messages = append(messages, map[string]interface{}{
				"role": "assistant", "content": fmt.Sprintf("Welcome back to Unit %d: %s. We are continuing our lesson on %s. Let's resume!", currentUnitID, unitTitle, grammarFocus), "contentTh": "ยินดีต้อนรับกลับเข้าสู่บทเรียนครับ มาเรียนกันต่อเลย!", "type": "text",
			})
		}
	} else {
		// Create new session
		sessionID = uuid.New().String()
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

		introEn := fmt.Sprintf("Welcome to Unit %d: %s. Today we will learn about %s. Let's study together!", currentUnitID, unitTitle, grammarFocus)
		introTh := fmt.Sprintf("ยินดีต้อนรับสู่บทเรียนที่ %d เรื่อง %s วันนี้เราจะมาเรียนเรื่อง %s กันนะครับ", currentUnitID, unitTitle, grammarFocus)

		// Store assistant opening message
		msgID := uuid.New().String()
		s.insertTutorMessage(ctx, sessionID, userID, currentUnitID, "assistant", introEn, introTh, "text", "session_start", 0, nil)
		_ = msgID

		messages = []map[string]interface{}{
			{"role": "assistant", "content": introEn, "contentTh": introTh, "type": "text"},
		}
	}

	return map[string]interface{}{
		"sessionId":        sessionID,
		"nextAction":       decision.Action,
		"mode":             decision.Mode,
		"unit":             map[string]interface{}{"unitNo": currentUnitID, "title": unitTitle},
		"messages":         messages, // return array of messages to frontend
		"dueItems":         dueItems,
		"availableActions": []string{"hint", "repeat", "review", "restart", "continue"},
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
			if pattern, ok := resp["pattern"].(string); ok && pattern != "" {
				grammarFocus = pattern
			}
		}
		s.updateStep(ctx, userID, unitID, "listening_practice")
		s.updateSessionPractice(ctx, sessionID, "mixed", "grammar_explained", "", map[string]interface{}{
			"pattern": grammarFocus,
		})

	case "listening_practice":
		itemID, sentence := s.getCurrentOrNextListening(ctx, sessionID, unitID)
		result["nextAction"] = "start_listening"
		result["mode"] = "listening"
		result["instruction"] = "ฟังแล้วพิมพ์สิ่งที่ได้ยินครับ"
		result["lessonItemId"] = itemID
		result["targetHidden"] = true
		result["targetText"] = sentence
		result["ttsAvailable"] = sentence != ""
		s.updateSessionPractice(ctx, sessionID, "listening", "start_listening", itemID, map[string]interface{}{
			"lessonItemId": itemID,
			"targetText":   sentence,
			"pattern":      grammarFocus,
		})

	case "speaking_practice":
		result["nextAction"] = "start_speaking"
		result["mode"] = "speaking"
		result["instruction"] = "ลองพูดหรือพิมพ์ประโยคภาษาอังกฤษโดยใช้ pattern ที่เรียนครับ"
		result["pattern"] = grammarFocus
		result["situation"] = fmt.Sprintf("Make one natural sentence about your life using: %s", grammarFocus)
		s.updateSessionPractice(ctx, sessionID, "speaking", "start_speaking", "", map[string]interface{}{
			"pattern":   grammarFocus,
			"situation": result["situation"],
		})

	case "reading_practice":
		passage := s.getStoredReadingPassage(ctx, sessionID, unitID, grammarFocus, "A1", unitTitle)
		result["nextAction"] = "start_reading"
		result["mode"] = "reading"
		result["instruction"] = "อ่านแล้วลองแปลเป็นภาษาไทยครับ"
		result["passage"] = passage
		result["pattern"] = grammarFocus
		s.updateSessionPractice(ctx, sessionID, "reading", "start_reading", "", map[string]interface{}{
			"passage": passage,
			"pattern": grammarFocus,
		})

	case "mini_quiz":
		result["nextAction"] = "review_summary"
		result["instruction"] = "สรุปสิ่งที่เรียนในบทนี้"
		s.updateStep(ctx, userID, unitID, "review_summary")
		s.updateSessionPractice(ctx, sessionID, "mixed", "review_summary", "", map[string]interface{}{
			"pattern": grammarFocus,
		})

	case "review_summary", "schedule_review":
		s.completeUnit(ctx, userID, unitID)
		result["nextAction"] = "unit_completed"
		result["instruction"] = "เรียนจบบทนี้แล้วครับ!"
	}

	return result, nil
}

// EvaluateListening evaluates a listening answer using a deterministic
// evaluator. AI is only used afterwards to enrich feedback text – the score
// and correctness boolean are always trustworthy.
func (s *Service) EvaluateListening(ctx context.Context, sessionID, userID, lessonItemID, answer string) (map[string]interface{}, error) {
	var unitID int
	s.db.QueryRow(ctx, `SELECT unit_id FROM tutor_sessions WHERE id = $1`, sessionID).Scan(&unitID)

	currentItemID, targetText := s.getCurrentOrNextListening(ctx, sessionID, unitID)
	if lessonItemID == "" {
		lessonItemID = currentItemID
	}
	if targetText == "" {
		targetText = s.lessonContentExcerpt(ctx, unitID)
	}

	eval := EvaluateAnswer(targetText, answer)
	finalScore := eval.Score
	isCorrect := eval.IsCorrect

	// Determine progressive hint level based on previous failures.
	var failCount int
	s.db.QueryRow(ctx, `SELECT COUNT(*) FROM listening_attempts WHERE session_id = $1 AND is_correct = false`, sessionID).Scan(&failCount)
	hintLevel := 1
	if failCount >= 1 {
		hintLevel = 2
	}
	if failCount >= 2 {
		hintLevel = 3
	}

	result := map[string]interface{}{
		"score":         finalScore,
		"isCorrect":     isCorrect,
		"targetText":    targetText,
		"normalizedExp": eval.NormalizedExp,
		"normalizedAns": eval.NormalizedAns,
		"matchRatio":    eval.MatchRatio,
	}
	if !isCorrect {
		result["hint"] = BuildMaskedHint(targetText, hintLevel)
		result["correction"] = targetText
		if eval.NormalizedAns == "" {
			result["feedbackTh"] = "ลองพิมพ์คำตอบใหม่นะครับ"
		} else if finalScore >= 0.6 {
			result["feedbackTh"] = "ใกล้แล้วครับ! ดูคำใบ้ด้านล่างแล้วลองอีกครั้ง"
		} else {
			result["feedbackTh"] = "ลองอีกครั้งนะครับ ดู hint เป็นตัวช่วยได้"
		}
		mistakes := make([]map[string]interface{}, 0)
		for _, w := range eval.MissingWords {
			mistakes = append(mistakes, map[string]interface{}{"type": "missing_word", "value": w})
		}
		for _, w := range eval.ExtraWords {
			mistakes = append(mistakes, map[string]interface{}{"type": "extra_word", "value": w})
		}
		result["mistakes"] = mistakes
	} else {
		result["feedbackTh"] = "เก่งมากครับ! ตอบถูกต้องเป๊ะ"
	}

	// Store attempt
	attemptID := uuid.New().String()
	mistakesJSON, _ := json.Marshal(result["mistakes"])
	s.db.Exec(ctx, `INSERT INTO listening_attempts (id, user_id, session_id, unit_id, target_text, user_text, score, is_correct, mistakes) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		attemptID, userID, sessionID, unitID, targetText, answer, finalScore, isCorrect, string(mistakesJSON))

	// Store weaknesses if any
	for _, m := range listenMistakeIter(result["mistakes"]) {
		wID := uuid.New().String()
		wType, _ := m["type"].(string)
		wDetail, _ := m["value"].(string)
		nextDue := CalculateNextDue(finalScore, 0, 0)
		s.db.Exec(ctx, `INSERT INTO weaknesses (id, user_id, unit_id, source_type, source_id, weakness_type, detail, example_wrong, example_correct, next_due_at) VALUES ($1,$2,$3,'listening',$4,$5,$6,$7,$8,$9)`,
			wID, userID, unitID, attemptID, wType, wDetail, answer, targetText, nextDue)
	}

	if isCorrect {
		s.markItemUsed(ctx, sessionID, "usedListeningIds", lessonItemID)
		pass := s.incrementPassCount(ctx, sessionID, "listeningPassCount")
		result["passCount"] = pass
		result["passRequired"] = RequiredPassPerSkill

		if pass >= RequiredPassPerSkill {
			s.updateStep(ctx, userID, unitID, "speaking_practice")
			s.clearSessionItem(ctx, sessionID, "speaking", "start_speaking", map[string]interface{}{})
			result["nextAction"] = "start_speaking"
			result["feedbackTh"] = "เก่งมาก! ผ่าน listening ครบ 3 รอบแล้ว ไปฝึกพูดกันต่อเลย"
		} else {
			// Stay in listening, queue up a fresh sentence.
			nextID, nextText := s.pickNextListeningItem(ctx, sessionID, unitID)
			if nextText == "" {
				// no more items – allow advancement so we don't trap the user
				s.updateStep(ctx, userID, unitID, "speaking_practice")
				s.clearSessionItem(ctx, sessionID, "speaking", "start_speaking", map[string]interface{}{})
				result["nextAction"] = "start_speaking"
			} else {
				s.updateSessionPractice(ctx, sessionID, "listening", "start_listening", nextID, map[string]interface{}{
					"lessonItemId": nextID,
					"targetText":   nextText,
				})
				result["nextAction"] = "start_listening"
				result["nextItemId"] = nextID
				result["nextTargetText"] = nextText
				result["feedbackTh"] = "ถูกต้องครับ! รอบที่ " + itoa(pass) + "/3 — มาฟังประโยคถัดไปกัน"
			}
		}
	} else {
		result["nextAction"] = "retry_listening"
		s.updateSessionPractice(ctx, sessionID, "listening", "retry_listening", lessonItemID, map[string]interface{}{
			"lessonItemId": lessonItemID,
			"targetText":   targetText,
		})
	}
	s.updateUnitSkillScore(ctx, userID, unitID, "listening", finalScore)
	return result, nil
}

// listenMistakeIter normalises whatever shape was stored into result["mistakes"].
func listenMistakeIter(v interface{}) []map[string]interface{} {
	switch m := v.(type) {
	case []map[string]interface{}:
		return m
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(m))
		for _, item := range m {
			if mm, ok := item.(map[string]interface{}); ok {
				out = append(out, mm)
			}
		}
		return out
	}
	return nil
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

	score := parseScore(result["score"])
	result["score"] = score
	grammarScore := parseScore(result["grammarScore"])
	if grammarScore == 0 {
		grammarScore = score
	}
	pronunciationScore := parseScore(result["pronunciationScore"])
	if pronunciationScore == 0 {
		pronunciationScore = score
	}
	fluencyScore := parseScore(result["fluencyScore"])
	if fluencyScore == 0 {
		fluencyScore = score
	}
	nativeSuggestion, _ := result["nativeSuggestion"].(string)

	attemptID := uuid.New().String()
	mistakesJSON, _ := json.Marshal(result["mistakes"])
	_, insertErr := s.db.Exec(ctx, `INSERT INTO speaking_attempts (id, user_id, session_id, unit_id, transcript, target_pattern, score, feedback_th, correction_text, mistakes, pronunciation_score, fluency_score, grammar_score, native_suggestion) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		attemptID, userID, sessionID, unitID, transcript, grammarFocus, score, result["feedbackTh"], result["correction"], string(mistakesJSON), pronunciationScore, fluencyScore, grammarScore, nativeSuggestion)
	if insertErr != nil {
		s.db.Exec(ctx, `INSERT INTO speaking_attempts (id, user_id, session_id, unit_id, transcript, target_pattern, score, feedback_th, mistakes) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			attemptID, userID, sessionID, unitID, transcript, grammarFocus, score, result["feedbackTh"], string(mistakesJSON))
	}

	if score >= 0.85 {
		pass := s.incrementPassCount(ctx, sessionID, "speakingPassCount")
		result["passCount"] = pass
		result["passRequired"] = RequiredPassPerSkill
		if pass >= RequiredPassPerSkill {
			s.updateStep(ctx, userID, unitID, "reading_practice")
			s.clearSessionItem(ctx, sessionID, "reading", "start_reading", map[string]interface{}{"pattern": grammarFocus})
			result["nextAction"] = "start_reading"
			result["feedbackTh"] = "ฝึกพูดครบ 3 รอบแล้ว มาฝึกอ่านกันต่อครับ"
		} else {
			// Stay in speaking, request a fresh creative situation next round.
			s.updateSessionPractice(ctx, sessionID, "speaking", "start_speaking", "", map[string]interface{}{
				"pattern":     grammarFocus,
				"situation":   "Tell me a NEW everyday Thai-life moment that uses: " + grammarFocus,
				"freshPrompt": true,
			})
			result["nextAction"] = "start_speaking"
			result["feedbackTh"] = "เก่งมาก! รอบที่ " + itoa(pass) + "/3 ของพูด — ลองพูดอีกประโยคใหม่ที่ต่างจากเดิม"
		}
	} else {
		result["nextAction"] = "retry_speaking"
		s.updateSessionPractice(ctx, sessionID, "speaking", "retry_speaking", "", map[string]interface{}{"pattern": grammarFocus})
	}
	result["grammarScore"] = grammarScore
	result["pronunciationScore"] = pronunciationScore
	result["fluencyScore"] = fluencyScore
	s.updateUnitSkillScore(ctx, userID, unitID, "speaking", score)
	return result, nil
}

// EvaluateSpeakingText evaluates a text-based speaking answer (user typed instead of recording)
func (s *Service) EvaluateSpeakingText(ctx context.Context, sessionID, userID, text string) (map[string]interface{}, error) {
	return s.EvaluateSpeaking(ctx, sessionID, userID, text)
}

// EvaluateReading evaluates a reading translation
func (s *Service) EvaluateReading(ctx context.Context, sessionID, userID, lessonItemID, translation string) (map[string]interface{}, error) {
	var unitID int
	s.db.QueryRow(ctx, `SELECT unit_id FROM tutor_sessions WHERE id = $1`, sessionID).Scan(&unitID)

	var unitTitle, grammarFocus, level string
	s.db.QueryRow(ctx, `SELECT COALESCE(title,''), COALESCE(grammar_focus,''), COALESCE(level,'A1') FROM lesson_units WHERE id = $1`, unitID).Scan(&unitTitle, &grammarFocus, &level)

	passage := s.getStoredReadingPassage(ctx, sessionID, unitID, grammarFocus, level, unitTitle)
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

	score := parseScore(result["score"])
	result["score"] = score
	attemptID := uuid.New().String()
	vocabJSON, _ := json.Marshal(result["vocabulary"])
	mistakesJSON, _ := json.Marshal(result["mistakes"])
	s.db.Exec(ctx, `INSERT INTO reading_attempts (id, user_id, session_id, unit_id, passage, user_translation, ai_translation, score, feedback_th, vocabulary, mistakes) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		attemptID, userID, sessionID, unitID, passage, translation, result["aiTranslation"], score, result["feedbackTh"], string(vocabJSON), string(mistakesJSON))

	if score >= 0.75 {
		pass := s.incrementPassCount(ctx, sessionID, "readingPassCount")
		result["passCount"] = pass
		result["passRequired"] = RequiredPassPerSkill
		if pass >= RequiredPassPerSkill {
			s.updateStep(ctx, userID, unitID, "mini_quiz")
			s.clearSessionItem(ctx, sessionID, "mixed", "mini_quiz", map[string]interface{}{"pattern": grammarFocus})
			result["nextAction"] = "mini_quiz"
			result["feedbackTh"] = "ครบ 3 รอบของอ่านแล้ว ไปสรุปบทเรียนกันต่อ"
		} else {
			// Force a new passage next round.
			s.clearSessionItem(ctx, sessionID, "reading", "start_reading", map[string]interface{}{
				"pattern":     grammarFocus,
				"freshPrompt": true,
			})
			result["nextAction"] = "start_reading"
			result["feedbackTh"] = "ดีมาก! รอบที่ " + itoa(pass) + "/3 ของอ่าน — มาลองอีก passage นึง"
		}
	} else {
		result["nextAction"] = "retry_reading"
		s.updateSessionPractice(ctx, sessionID, "reading", "retry_reading", "", map[string]interface{}{"passage": passage, "pattern": grammarFocus})
	}
	result["passage"] = passage
	s.updateUnitSkillScore(ctx, userID, unitID, "reading", score)
	return result, nil
}

// ReviewFlashcard processes a flashcard review
func (s *Service) ReviewFlashcard(ctx context.Context, userID, flashcardID string, score float64) (map[string]interface{}, error) {
	score = normalizeScore(score)
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
		"dueToday":      map[string]interface{}{"vocabulary": dueItems.VocabularyDueCount, "weakness": dueItems.WeaknessDueCount, "unit": dueItems.UnitReviewDueCount},
		"scores":        map[string]interface{}{"listening": lScore, "speaking": sScore, "reading": rScore},
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
	s.NotifyLineAsync(fmt.Sprintf("AI Tutor Loop: user completed Unit %d. Review scheduled at %s.", unitID, nextDue.Format(time.RFC3339)))
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

func (s *Service) GetCachedTTS(ctx context.Context, text string) ([]byte, error) {
	if s.minioClient == nil {
		return nil, fmt.Errorf("minio not configured")
	}
	hash := md5.Sum([]byte(strings.ToLower(strings.TrimSpace(text))))
	key := hex.EncodeToString(hash[:])

	var minioPath string
	err := s.db.QueryRow(ctx, `SELECT minio_path FROM tts_cache WHERE cache_key = $1`, key).Scan(&minioPath)
	if err != nil {
		return nil, err
	}

	obj, err := s.minioClient.GetObject(ctx, s.cfg.MinIO.Bucket, minioPath, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(obj)
}

func (s *Service) CacheTTS(ctx context.Context, text string, audioData []byte) error {
	if s.minioClient == nil {
		return fmt.Errorf("minio not configured")
	}
	hash := md5.Sum([]byte(strings.ToLower(strings.TrimSpace(text))))
	key := hex.EncodeToString(hash[:])

	minioPath := fmt.Sprintf("%s%s.wav", s.cfg.MinIO.PrefixTTS, key)
	_, err := s.minioClient.PutObject(ctx, s.cfg.MinIO.Bucket, minioPath, bytes.NewReader(audioData), int64(len(audioData)), minio.PutObjectOptions{ContentType: "audio/wav"})
	if err != nil {
		return err
	}

	_, err = s.db.Exec(ctx, `INSERT INTO tts_cache (cache_key, original_text, minio_path) VALUES ($1, $2, $3) ON CONFLICT (cache_key) DO NOTHING`, key, text, minioPath)
	return err
}

