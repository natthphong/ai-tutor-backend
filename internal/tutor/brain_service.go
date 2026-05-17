package tutor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/internal/ai"
	"go.uber.org/zap"
)

type sessionContext struct {
	SessionID     string
	UserID        string
	UnitID        int
	UnitNo        int
	UnitTitle     string
	GrammarFocus  string
	RawContent    string
	Mode          string
	CurrentAction string
	CurrentStep   string
	CurrentItemID string
	ResumeState   map[string]interface{}
}

func (s *Service) HandleTurn(ctx context.Context, sessionID string, userID string, req TurnRequest) (map[string]interface{}, error) {
	sc, err := s.loadSessionContext(ctx, sessionID, userID)
	if err != nil {
		return nil, err
	}
	inputKind := strings.TrimSpace(req.InputKind)
	if inputKind == "" {
		inputKind = "text"
	}

	intent := ClassifyTutorIntent(req.Text, req.ClientAction, sc.Mode)
	s.insertTutorMessage(ctx, sessionID, userID, sc.UnitID, "user", req.Text, "", inputKind, intent.Intent, 0, nil)

	var response map[string]interface{}
	switch intent.Intent {
	case IntentHintRequest:
		response = s.handleHintRequest(ctx, sc)
	case IntentRevealAnswer:
		response = s.handleRevealAnswer(ctx, sc)
	case IntentRepeatRequest:
		response = s.handleRepeatRequest(ctx, sc)
	case IntentReviewRequest:
		response = s.handleReviewRequest(ctx, sc)
	case IntentRestartUnit:
		response = s.handleRestartUnit(ctx, sc)
	case IntentContinue:
		step, err := s.GetNextStep(ctx, sessionID, userID)
		if err != nil {
			return nil, err
		}
		response = s.stepToTurnResponse(ctx, sc, step)
	case IntentChangeMode:
		response = s.handleChangeMode(ctx, sc, intent.Mode)
	case IntentQuestion:
		response = s.handleQuestion(ctx, sc, req.Text)
	default:
		response = s.handlePracticeAnswer(ctx, sc, req.Text)
	}

	if response == nil {
		response = s.baseTurnResponse(sc, intent.Intent, "unknown")
		response["messages"] = []TutorMessageDTO{{Role: "assistant", Content: "I am ready. Please try again.", ContentTh: "พร้อมแล้วครับ ลองอีกครั้งได้เลย", Type: "text"}}
	}
	response["intent"] = intent.Intent
	response["intentConfidence"] = intent.Confidence
	response["availableActions"] = []string{"hint", "repeat", "review", "restart", "continue"}

	actionTaken, _ := response["nextAction"].(string)
	s.insertTurnEvent(ctx, sessionID, userID, sc.UnitID, req.Text, inputKind, req.ClientAction, intent, actionTaken, response)
	return response, nil
}

func (s *Service) loadSessionContext(ctx context.Context, sessionID string, userID string) (sessionContext, error) {
	var sc sessionContext
	var resumeRaw string
	err := s.db.QueryRow(ctx, `
		SELECT ts.id, ts.user_id, ts.unit_id, COALESCE(lu.unit_no, ts.unit_id), COALESCE(lu.title,''), COALESCE(lu.grammar_focus,''), COALESCE(lu.raw_content,''),
		       COALESCE(ts.mode,'mixed'), COALESCE(ts.current_action,''), COALESCE(ts.current_item_id::text,''), COALESCE(ts.resume_state,'{}'::jsonb)::text,
		       COALESCE(up.current_step,'intro')
		FROM tutor_sessions ts
		JOIN lesson_units lu ON lu.id = ts.unit_id
		LEFT JOIN user_unit_progress up ON up.user_id = ts.user_id AND up.unit_id = ts.unit_id
		WHERE ts.id = $1 AND ts.user_id = $2`,
		sessionID, userID,
	).Scan(&sc.SessionID, &sc.UserID, &sc.UnitID, &sc.UnitNo, &sc.UnitTitle, &sc.GrammarFocus, &sc.RawContent, &sc.Mode, &sc.CurrentAction, &sc.CurrentItemID, &resumeRaw, &sc.CurrentStep)
	if err != nil {
		err = s.db.QueryRow(ctx, `
			SELECT ts.id, ts.user_id, ts.unit_id, COALESCE(lu.unit_no, ts.unit_id), COALESCE(lu.title,''), COALESCE(lu.grammar_focus,''), COALESCE(lu.raw_content,''),
			       COALESCE(ts.mode,'mixed'), COALESCE(ts.current_action,''), COALESCE(ts.current_item_id::text,''),
			       COALESCE(up.current_step,'intro')
			FROM tutor_sessions ts
			JOIN lesson_units lu ON lu.id = ts.unit_id
			LEFT JOIN user_unit_progress up ON up.user_id = ts.user_id AND up.unit_id = ts.unit_id
			WHERE ts.id = $1 AND ts.user_id = $2`,
			sessionID, userID,
		).Scan(&sc.SessionID, &sc.UserID, &sc.UnitID, &sc.UnitNo, &sc.UnitTitle, &sc.GrammarFocus, &sc.RawContent, &sc.Mode, &sc.CurrentAction, &sc.CurrentItemID, &sc.CurrentStep)
		if err != nil {
			return sc, fmt.Errorf("session not found")
		}
		resumeRaw = "{}"
	}
	if resumeRaw == "" {
		resumeRaw = "{}"
	}
	_ = json.Unmarshal([]byte(resumeRaw), &sc.ResumeState)
	if sc.ResumeState == nil {
		sc.ResumeState = map[string]interface{}{}
	}
	return sc, nil
}

func (s *Service) baseTurnResponse(sc sessionContext, intent string, nextAction string) map[string]interface{} {
	return map[string]interface{}{
		"sessionId":  sc.SessionID,
		"unit":       map[string]interface{}{"unitNo": sc.UnitNo, "title": sc.UnitTitle},
		"unitId":     sc.UnitID,
		"mode":       sc.Mode,
		"intent":     intent,
		"nextAction": nextAction,
		"practice":   s.practiceFromState(sc),
	}
}

// handleRevealAnswer responds with the current target answer plus a short
// Thai meaning and grammar note. It never refuses to reveal — that's the whole
// point of this intent.
func (s *Service) handleRevealAnswer(ctx context.Context, sc sessionContext) map[string]interface{} {
	practice := s.practiceFromState(sc)
	target := strings.TrimSpace(practice.TargetText)
	if target == "" && sc.Mode == "listening" {
		_, target = s.getCurrentOrNextListening(ctx, sc.SessionID, sc.UnitID)
		practice.TargetText = target
		practice.TTSAvailable = target != ""
	}
	if target == "" {
		target = strings.TrimSpace(practice.Passage)
	}

	if target == "" {
		msg := TutorMessageDTO{
			Role: "assistant", Type: "answer_reveal",
			Content:   "There is no active answer in this step yet. Let's pick a practice first.",
			ContentTh: "ตอนนี้ยังไม่มีโจทย์ที่ต้องตอบครับ ลองกด continue เพื่อเริ่มฝึกได้เลย",
		}
		s.insertTutorMessage(ctx, sc.SessionID, sc.UserID, sc.UnitID, "assistant", msg.Content, msg.ContentTh, "answer_reveal", IntentRevealAnswer, 0, nil)
		resp := s.baseTurnResponse(sc, IntentRevealAnswer, "no_answer_available")
		resp["messages"] = []TutorMessageDTO{msg}
		return resp
	}

	thai, grammar := s.buildAnswerExplanation(ctx, sc, target)
	contentTh := "เฉลย: " + target
	if thai != "" {
		contentTh += "\nแปลว่า: " + thai
	}
	if grammar != "" {
		contentTh += "\nไวยากรณ์: " + grammar
	}
	msg := TutorMessageDTO{
		Role:      "assistant",
		Type:      "answer_reveal",
		Content:   "Correct answer: " + target,
		ContentTh: contentTh,
	}
	metadata := map[string]interface{}{
		"correctAnswer":   target,
		"thaiMeaning":     thai,
		"grammarNoteTh":   grammar,
		"revealRequested": true,
	}
	s.insertTutorMessage(ctx, sc.SessionID, sc.UserID, sc.UnitID, "assistant", msg.Content, msg.ContentTh, "answer_reveal", IntentRevealAnswer, 0, metadata)

	resp := s.baseTurnResponse(sc, IntentRevealAnswer, "answer_revealed")
	resp["messages"] = []TutorMessageDTO{msg}
	resp["practice"] = practice
	resp["result"] = map[string]interface{}{
		"correctAnswer": target,
		"thaiMeaning":   thai,
		"grammarNoteTh": grammar,
		"nextAction":    "retry_or_continue",
	}
	return resp
}

// buildAnswerExplanation asks the LLM for a Thai translation + grammar note
// of the target sentence. If the LLM fails we still return ("", "") and the
// caller will reveal at least the answer itself.
func (s *Service) buildAnswerExplanation(ctx context.Context, sc sessionContext, target string) (thai string, grammar string) {
	prompt := BuildAnswerRevealPrompt(sc.UnitTitle, sc.GrammarFocus, target)
	resp, err := s.router.Chat(ctx, ai.ChatRequest{
		SystemPrompt: BuildTutorSystemPrompt(),
		Messages:     []ai.ChatMessage{{Role: "user", Content: prompt}},
		UseCase:      "answer_reveal",
		UserID:       sc.UserID,
		SessionID:    sc.SessionID,
	})
	if err != nil {
		return "", ""
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Content), &parsed); err != nil {
		return "", ""
	}
	thai, _ = parsed["thaiMeaning"].(string)
	grammar, _ = parsed["grammarNoteTh"].(string)
	return thai, grammar
}

func (s *Service) handleHintRequest(ctx context.Context, sc sessionContext) map[string]interface{} {
	practice := s.practiceFromState(sc)
	hintTh := "ลองดูคำใบ้นี้ แล้วตอบใหม่อีกครั้งครับ"
	hint := "Try again with this hint."
	metadata := map[string]interface{}{}

	switch sc.Mode {
	case "listening":
		if practice.TargetText == "" {
			itemID, target := s.getCurrentOrNextListening(ctx, sc.SessionID, sc.UnitID)
			practice.LessonItemID = itemID
			practice.TargetText = target
			practice.TTSAvailable = target != ""
		}
		level := s.nextHintLevel(ctx, sc.SessionID)
		hint = BuildMaskedHint(practice.TargetText, level)
		hintTh = "ลองสะกดทีละคำตามคำใบ้ด้านบน แล้วพิมพ์ตอบใหม่ได้เลยครับ"

	case "speaking":
		pattern := practice.Pattern
		if pattern == "" {
			pattern = sc.GrammarFocus
		}
		situation := ""
		if v, ok := sc.ResumeState["situation"].(string); ok {
			situation = v
		}
		example, coachTh, noteTh := s.aiSpeakingHint(ctx, sc, pattern, situation)
		hint = "Pattern: " + pattern
		if example != "" {
			hint += "\nExample: " + example
			metadata["example"] = example
		}
		hintTh = coachTh
		if hintTh == "" {
			hintTh = "ลองพูดเล่าเรื่องของคุณเองโดยใช้ pattern: " + pattern
		}
		if noteTh != "" {
			hintTh += "\nไวยากรณ์: " + noteTh
			metadata["grammarNoteTh"] = noteTh
		}

	case "reading":
		pattern := practice.Pattern
		if pattern == "" {
			pattern = sc.GrammarFocus
		}
		passage := practice.Passage
		keyWords, coachTh, noteTh := s.aiReadingHint(ctx, sc, pattern, passage)
		// Frontend hint: masked first sentence + key words.
		first := firstSentence(passage)
		var b strings.Builder
		if first != "" {
			b.WriteString("Focus phrase: ")
			b.WriteString(BuildMaskedHint(first, 1))
		}
		if len(keyWords) > 0 {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString("Key words: ")
			for i, kw := range keyWords {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(kw)
			}
		}
		if pattern != "" {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString("Pattern: " + pattern)
		}
		hint = b.String()
		if hint == "" {
			hint = "Read for the main idea first, then translate phrase by phrase."
		}
		hintTh = coachTh
		if hintTh == "" {
			hintTh = "อ่านจับใจความก่อน แล้วค่อยแปลทีละวลีครับ"
		}
		if noteTh != "" {
			hintTh += "\nไวยากรณ์: " + noteTh
			metadata["grammarNoteTh"] = noteTh
		}

	default:
		hintTh = "บอกได้เลยครับว่าอยากให้ใบ้ส่วนไหน"
	}

	msg := TutorMessageDTO{Role: "assistant", Content: hint, ContentTh: hintTh, Type: "hint"}
	s.insertTutorMessage(ctx, sc.SessionID, sc.UserID, sc.UnitID, "assistant", msg.Content, msg.ContentTh, "hint", IntentHintRequest, 0, metadata)
	resp := s.baseTurnResponse(sc, IntentHintRequest, "show_hint")
	resp["messages"] = []TutorMessageDTO{msg}
	resp["practice"] = practice
	if len(metadata) > 0 {
		resp["hintMeta"] = metadata
	}
	return resp
}

// aiSpeakingHint requests a coach-style hint via the LLM. Falls back to a
// deterministic template when the AI is unavailable.
func (s *Service) aiSpeakingHint(ctx context.Context, sc sessionContext, pattern, situation string) (example, messageTh, grammarTh string) {
	prompt := BuildSpeakingHintPrompt(sc.UnitTitle, pattern, situation)
	resp, err := s.router.Chat(ctx, ai.ChatRequest{
		SystemPrompt: BuildTutorSystemPrompt(),
		Messages:     []ai.ChatMessage{{Role: "user", Content: prompt}},
		UseCase:      "speaking_hint",
		UserID:       sc.UserID,
		SessionID:    sc.SessionID,
	})
	if err != nil {
		return "", "", ""
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(stripCodeFences(resp.Content)), &parsed); err != nil {
		return "", "", ""
	}
	example, _ = parsed["example"].(string)
	messageTh, _ = parsed["messageTh"].(string)
	grammarTh, _ = parsed["grammarNoteTh"].(string)
	return example, messageTh, grammarTh
}

func (s *Service) aiReadingHint(ctx context.Context, sc sessionContext, pattern, passage string) (keyWords []string, messageTh, grammarTh string) {
	prompt := BuildReadingHintPrompt(sc.UnitTitle, pattern, passage)
	resp, err := s.router.Chat(ctx, ai.ChatRequest{
		SystemPrompt: BuildTutorSystemPrompt(),
		Messages:     []ai.ChatMessage{{Role: "user", Content: prompt}},
		UseCase:      "reading_hint",
		UserID:       sc.UserID,
		SessionID:    sc.SessionID,
	})
	if err != nil {
		return nil, "", ""
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(stripCodeFences(resp.Content)), &parsed); err != nil {
		return nil, "", ""
	}
	messageTh, _ = parsed["messageTh"].(string)
	grammarTh, _ = parsed["grammarNoteTh"].(string)
	if arr, ok := parsed["keyWords"].([]interface{}); ok {
		for _, kw := range arr {
			if m, ok := kw.(map[string]interface{}); ok {
				en, _ := m["word"].(string)
				th, _ := m["meaningTh"].(string)
				if en != "" {
					if th != "" {
						keyWords = append(keyWords, en+" ("+th+")")
					} else {
						keyWords = append(keyWords, en)
					}
				}
			}
		}
	}
	return keyWords, messageTh, grammarTh
}

func (s *Service) handleRepeatRequest(ctx context.Context, sc sessionContext) map[string]interface{} {
	practice := s.practiceFromState(sc)
	if sc.Mode == "listening" && practice.TargetText == "" {
		itemID, target := s.getCurrentOrNextListening(ctx, sc.SessionID, sc.UnitID)
		practice.LessonItemID = itemID
		practice.TargetText = target
		practice.TTSAvailable = target != ""
	}
	msg := TutorMessageDTO{Role: "assistant", Content: "Let's repeat the current practice.", ContentTh: "ได้ครับ มาทำข้อเดิมอีกครั้ง", Type: "text"}
	s.insertTutorMessage(ctx, sc.SessionID, sc.UserID, sc.UnitID, "assistant", msg.Content, msg.ContentTh, "text", IntentRepeatRequest, 0, nil)
	resp := s.baseTurnResponse(sc, IntentRepeatRequest, "repeat_current")
	resp["messages"] = []TutorMessageDTO{msg}
	resp["practice"] = practice
	return resp
}

func (s *Service) handleReviewRequest(ctx context.Context, sc sessionContext) map[string]interface{} {
	summary := s.buildWeaknessReview(ctx, sc.UserID, sc.UnitID)
	if summary == "" {
		summary = "วันนี้เราจะทวน pattern หลักของบทนี้: " + sc.GrammarFocus
	}
	msg := TutorMessageDTO{Role: "assistant", Content: "Review mode is ready.", ContentTh: summary, Type: "summary"}
	s.insertTutorMessage(ctx, sc.SessionID, sc.UserID, sc.UnitID, "assistant", msg.Content, msg.ContentTh, "summary", IntentReviewRequest, 0, nil)
	resp := s.baseTurnResponse(sc, IntentReviewRequest, "review_unit")
	resp["mode"] = "review"
	resp["messages"] = []TutorMessageDTO{msg}
	return resp
}

func (s *Service) handleRestartUnit(ctx context.Context, sc sessionContext) map[string]interface{} {
	s.db.Exec(ctx, `UPDATE user_unit_progress SET status = 'in_progress', current_step = 'intro', updated_at = now() WHERE user_id = $1 AND unit_id = $2`, sc.UserID, sc.UnitID)
	s.db.Exec(ctx, `UPDATE tutor_sessions SET mode = 'mixed', current_action = 'restart_unit', current_item_id = NULL, resume_state = '{}'::jsonb, updated_at = now() WHERE id = $1`, sc.SessionID)
	msg := TutorMessageDTO{Role: "assistant", Content: "We restarted this unit.", ContentTh: "เริ่ม Unit นี้ใหม่แล้วครับ เดี๋ยวอธิบายตั้งแต่ต้นอีกครั้ง", Type: "text"}
	s.insertTutorMessage(ctx, sc.SessionID, sc.UserID, sc.UnitID, "assistant", msg.Content, msg.ContentTh, "text", IntentRestartUnit, 0, nil)
	resp := s.baseTurnResponse(sc, IntentRestartUnit, "continue_unit")
	resp["mode"] = "mixed"
	resp["messages"] = []TutorMessageDTO{msg}
	resp["practice"] = PracticeState{}
	return resp
}

func (s *Service) handleChangeMode(ctx context.Context, sc sessionContext, mode string) map[string]interface{} {
	if mode == "" {
		mode = "mixed"
	}
	step := "listening_practice"
	if mode == "speaking" {
		step = "speaking_practice"
	} else if mode == "reading" {
		step = "reading_practice"
	}
	s.db.Exec(ctx, `UPDATE user_unit_progress SET current_step = $1, updated_at = now() WHERE user_id = $2 AND unit_id = $3`, step, sc.UserID, sc.UnitID)
	s.db.Exec(ctx, `UPDATE tutor_sessions SET mode = $1, current_action = 'change_mode', updated_at = now() WHERE id = $2`, mode, sc.SessionID)
	stepResp, err := s.GetNextStep(ctx, sc.SessionID, sc.UserID)
	if err != nil {
		resp := s.baseTurnResponse(sc, IntentChangeMode, "change_mode")
		resp["mode"] = mode
		return resp
	}
	sc.Mode = mode
	return s.stepToTurnResponse(ctx, sc, stepResp)
}

func (s *Service) handleQuestion(ctx context.Context, sc sessionContext, question string) map[string]interface{} {
	truncated := sc.RawContent
	if len(truncated) > 3000 {
		truncated = truncated[:3000]
	}
	prompt := BuildQuestionAnswerPrompt(sc.UnitTitle, sc.GrammarFocus, truncated, question)
	resp, err := s.router.Chat(ctx, ai.ChatRequest{
		SystemPrompt: BuildTutorSystemPrompt(),
		Messages:     []ai.ChatMessage{{Role: "user", Content: prompt}},
		UseCase:      "unit_question_answer",
		UserID:       sc.UserID,
		SessionID:    sc.SessionID,
	})
	message := TutorMessageDTO{Role: "assistant", Content: "Good question. Let's connect it to this unit.", ContentTh: "คำถามดีครับ จุดนี้เกี่ยวกับบทนี้โดยตรง", Type: "text"}
	if err == nil {
		var parsed map[string]interface{}
		if json.Unmarshal([]byte(resp.Content), &parsed) == nil {
			if v, ok := parsed["message"].(string); ok && v != "" {
				message.Content = v
			}
			if v, ok := parsed["messageTh"].(string); ok && v != "" {
				message.ContentTh = v
			}
		}
	}
	s.insertTutorMessage(ctx, sc.SessionID, sc.UserID, sc.UnitID, "assistant", message.Content, message.ContentTh, "text", IntentQuestion, 0, nil)
	result := map[string]interface{}{"nextAction": "answer_question", "feedbackTh": message.ContentTh}
	respMap := s.baseTurnResponse(sc, IntentQuestion, "answer_question")
	respMap["messages"] = []TutorMessageDTO{message}
	respMap["result"] = result
	return respMap
}

func (s *Service) handlePracticeAnswer(ctx context.Context, sc sessionContext, text string) map[string]interface{} {
	mode := sc.Mode
	if mode == "mixed" || mode == "" {
		mode = modeFromStep(sc.CurrentStep)
	}
	var result map[string]interface{}
	var err error
	switch mode {
	case "speaking":
		result, err = s.EvaluateSpeakingText(ctx, sc.SessionID, sc.UserID, text)
	case "reading":
		result, err = s.EvaluateReading(ctx, sc.SessionID, sc.UserID, sc.CurrentItemID, text)
	default:
		mode = "listening"
		result, err = s.EvaluateListening(ctx, sc.SessionID, sc.UserID, sc.CurrentItemID, text)
	}
	if err != nil {
		result = map[string]interface{}{
			"score":      0.0,
			"feedbackTh": "ระบบตรวจคำตอบมีปัญหา ลองอีกครั้งครับ",
			"nextAction": "retry",
		}
	}
	score := parseScore(result["score"])
	content := resultMessageContent(mode, result)
	contentTh, _ := result["feedbackTh"].(string)
	msg := TutorMessageDTO{Role: "assistant", Content: content, ContentTh: contentTh, Type: "result"}
	s.insertTutorMessage(ctx, sc.SessionID, sc.UserID, sc.UnitID, "assistant", msg.Content, msg.ContentTh, "result", IntentAnswer, score, result)
	nextAction, _ := result["nextAction"].(string)
	if nextAction == "" {
		nextAction = "retry"
	}
	resp := s.baseTurnResponse(sc, IntentAnswer, nextAction)
	resp["mode"] = mode
	resp["messages"] = []TutorMessageDTO{msg}
	resp["result"] = result
	resp["practice"] = s.practiceFromStateAfterResult(sc, result)
	return resp
}

func (s *Service) stepToTurnResponse(ctx context.Context, sc sessionContext, step map[string]interface{}) map[string]interface{} {
	mode, _ := step["mode"].(string)
	if mode == "" {
		mode = sc.Mode
	}
	nextAction, _ := step["nextAction"].(string)
	if nextAction == "" {
		nextAction = "continue"
	}
	content, contentTh := stepMessage(step)
	msg := TutorMessageDTO{Role: "assistant", Content: content, ContentTh: contentTh, Type: "text"}
	if content != "" || contentTh != "" {
		s.insertTutorMessage(ctx, sc.SessionID, sc.UserID, sc.UnitID, "assistant", msg.Content, msg.ContentTh, "text", IntentContinue, 0, step)
	}
	resp := s.baseTurnResponse(sc, IntentContinue, nextAction)
	resp["mode"] = mode
	resp["messages"] = []TutorMessageDTO{msg}
	resp["practice"] = practiceFromStep(step)
	return resp
}

func (s *Service) practiceFromState(sc sessionContext) PracticeState {
	p := PracticeState{}
	if v, ok := sc.ResumeState["lessonItemId"].(string); ok {
		p.LessonItemID = v
	}
	if v, ok := sc.ResumeState["targetText"].(string); ok {
		p.TargetText = v
		p.TTSAvailable = v != ""
	}
	if v, ok := sc.ResumeState["passage"].(string); ok {
		p.Passage = v
	}
	if v, ok := sc.ResumeState["pattern"].(string); ok {
		p.Pattern = v
	}
	return p
}

func (s *Service) practiceFromStateAfterResult(sc sessionContext, result map[string]interface{}) PracticeState {
	p := s.practiceFromState(sc)
	if v, ok := result["targetText"].(string); ok && v != "" {
		p.TargetText = v
		p.TTSAvailable = true
	}
	return p
}

func practiceFromStep(step map[string]interface{}) PracticeState {
	p := PracticeState{}
	if v, ok := step["lessonItemId"].(string); ok {
		p.LessonItemID = v
	}
	if v, ok := step["targetText"].(string); ok {
		p.TargetText = v
		p.TTSAvailable = v != ""
	}
	if v, ok := step["passage"].(string); ok {
		p.Passage = v
	}
	if v, ok := step["pattern"].(string); ok {
		p.Pattern = v
	}
	if v, ok := step["ttsAvailable"].(bool); ok {
		p.TTSAvailable = v
	}
	return p
}

func stepMessage(step map[string]interface{}) (string, string) {
	var b strings.Builder
	if explanation, ok := step["explanation"].(map[string]interface{}); ok {
		if v, ok := explanation["message"].(string); ok {
			b.WriteString(v)
		}
		if v, ok := explanation["pattern"].(string); ok && v != "" {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString("Pattern: " + v)
		}
		if examples, ok := explanation["examples"].([]interface{}); ok && len(examples) > 0 {
			b.WriteString("\n\nExamples:")
			for _, ex := range examples {
				if em, ok := ex.(map[string]interface{}); ok {
					if en, ok := em["en"].(string); ok {
						b.WriteString("\n- " + en)
					}
				}
			}
		}
		contentTh, _ := explanation["messageTh"].(string)
		return b.String(), contentTh
	}
	if v, ok := step["instruction"].(string); ok {
		b.WriteString(v)
	}
	if v, ok := step["passage"].(string); ok && v != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(`"` + v + `"`)
	}
	if v, ok := step["pattern"].(string); ok && v != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("Pattern: " + v)
	}
	return b.String(), ""
}

func resultMessageContent(mode string, result map[string]interface{}) string {
	score := parseScore(result["score"])
	prefix := "Good try"
	if score >= 0.85 {
		prefix = "Correct"
	} else if score < 0.5 {
		prefix = "Keep practicing"
	}
	parts := []string{prefix}
	if correction, ok := result["correction"].(string); ok && correction != "" {
		parts = append(parts, "Correction: "+correction)
	}
	if suggestion, ok := result["nativeSuggestion"].(string); ok && suggestion != "" {
		parts = append(parts, "Natural: "+suggestion)
	}
	if hint, ok := result["hint"].(string); ok && hint != "" {
		parts = append(parts, "Hint: "+hint)
	}
	if mode == "speaking" {
		if transcript, ok := result["transcript"].(string); ok && transcript != "" {
			parts = append([]string{"I heard: " + transcript}, parts...)
		}
	}
	return strings.Join(parts, "\n\n")
}

func modeFromStep(step string) string {
	switch step {
	case "speaking_practice":
		return "speaking"
	case "reading_practice":
		return "reading"
	default:
		return "listening"
	}
}

func (s *Service) insertTutorMessage(ctx context.Context, sessionID, userID string, unitID int, role, content, contentTh, messageType, intent string, score float64, metadata interface{}) {
	metadataJSON, _ := json.Marshal(metadata)
	_, err := s.db.Exec(ctx, `INSERT INTO tutor_messages (id, session_id, user_id, unit_id, role, content, content_th, message_type, intent, score, metadata) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		uuid.New().String(), sessionID, userID, unitID, role, content, contentTh, messageType, intent, score, string(metadataJSON))
	if err == nil {
		return
	}
	s.logger.Debug("insert tutor message with intelligence columns failed, falling back", zap.Error(err))
	_, _ = s.db.Exec(ctx, `INSERT INTO tutor_messages (id, session_id, user_id, role, content, content_th, message_type, metadata) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		uuid.New().String(), sessionID, userID, role, content, contentTh, messageType, string(metadataJSON))
}

func (s *Service) insertTurnEvent(ctx context.Context, sessionID, userID string, unitID int, rawInput, inputKind, clientAction string, intent IntentClassification, actionTaken string, metadata interface{}) {
	metadataJSON, _ := json.Marshal(metadata)
	_, err := s.db.Exec(ctx, `INSERT INTO tutor_turn_events (id, session_id, user_id, unit_id, raw_input, input_kind, client_action, classified_intent, confidence, action_taken, metadata) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		uuid.New().String(), sessionID, userID, unitID, rawInput, inputKind, clientAction, intent.Intent, intent.Confidence, actionTaken, string(metadataJSON))
	if err != nil {
		s.logger.Debug("insert tutor turn event failed", zap.Error(err))
	}
}

func parseScore(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return normalizeScore(v)
	case float32:
		return normalizeScore(float64(v))
	case int:
		return normalizeScore(float64(v))
	case int64:
		return normalizeScore(float64(v))
	case json.Number:
		f, _ := v.Float64()
		return normalizeScore(f)
	case string:
		var f float64
		fmt.Sscanf(v, "%f", &f)
		return normalizeScore(f)
	default:
		return 0
	}
}

func firstSentence(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, sep := range []string{".", "!", "?"} {
		if idx := strings.Index(value, sep); idx > 0 {
			return strings.TrimSpace(value[:idx+1])
		}
	}
	if len(value) > 120 {
		return value[:120] + "..."
	}
	return value
}
