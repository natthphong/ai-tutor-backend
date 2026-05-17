package tutor

import (
	"strings"
	"unicode"
)

const (
	IntentAnswer        = "answer"
	IntentQuestion      = "question"
	IntentHintRequest   = "hint_request"
	IntentRepeatRequest = "repeat_request"
	IntentReviewRequest = "review_request"
	IntentRestartUnit   = "restart_unit"
	IntentContinue      = "continue"
	IntentChangeMode    = "change_mode"
	IntentRevealAnswer  = "reveal_answer"
	IntentUnknown       = "unknown"
)

type IntentClassification struct {
	Intent     string
	Confidence float64
	Mode       string
	Reason     string
}

type TurnRequest struct {
	UserID       string `json:"userId"`
	Text         string `json:"text"`
	InputKind    string `json:"inputKind"`
	ClientAction string `json:"clientAction"`
}

type TutorMessageDTO struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	ContentTh string `json:"contentTh,omitempty"`
	Type      string `json:"type"`
}

type PracticeState struct {
	LessonItemID string `json:"lessonItemId,omitempty"`
	TargetText   string `json:"targetText,omitempty"`
	Passage      string `json:"passage,omitempty"`
	Pattern      string `json:"pattern,omitempty"`
	TTSAvailable bool   `json:"ttsAvailable"`
}

type FlashcardReviewItem struct {
	ID           string  `json:"id"`
	Front        string  `json:"front"`
	Back         string  `json:"back"`
	Example      string  `json:"example,omitempty"`
	ExampleTh    string  `json:"exampleTh,omitempty"`
	CardType     string  `json:"cardType"`
	MasteryScore float64 `json:"masteryScore"`
}

func ClassifyTutorIntent(text, clientAction, currentMode string) IntentClassification {
	action := normalizeIntentText(clientAction)
	switch action {
	case "hint":
		return IntentClassification{Intent: IntentHintRequest, Confidence: 1, Reason: "client_action"}
	case "repeat":
		return IntentClassification{Intent: IntentRepeatRequest, Confidence: 1, Reason: "client_action"}
	case "review":
		return IntentClassification{Intent: IntentReviewRequest, Confidence: 1, Reason: "client_action"}
	case "restart":
		return IntentClassification{Intent: IntentRestartUnit, Confidence: 1, Reason: "client_action"}
	case "continue", "next":
		return IntentClassification{Intent: IntentContinue, Confidence: 1, Reason: "client_action"}
	case "answer":
		return IntentClassification{Intent: IntentAnswer, Confidence: 1, Reason: "client_action"}
	case "reveal", "reveal_answer", "show_answer":
		return IntentClassification{Intent: IntentRevealAnswer, Confidence: 1, Reason: "client_action"}
	}

	normalized := normalizeIntentText(text)
	if normalized == "" {
		return IntentClassification{Intent: IntentUnknown, Confidence: 0.2, Reason: "empty"}
	}
	if IsAnswerRevealRequest(normalized) {
		return IntentClassification{Intent: IntentRevealAnswer, Confidence: 0.99, Reason: "reveal_keyword"}
	}

	if containsAny(normalized, []string{"เริ่มใหม่", "เริ่มต้นใหม่", "restart", "reset", "start over"}) {
		return IntentClassification{Intent: IntentRestartUnit, Confidence: 0.98, Reason: "restart_keyword"}
	}
	if containsAny(normalized, []string{"ขอทวน", "ทวน", "ทบทวน", "review", "revise", "recap"}) {
		return IntentClassification{Intent: IntentReviewRequest, Confidence: 0.95, Reason: "review_keyword"}
	}
	if containsAny(normalized, []string{"ฟังอีก", "พูดอีก", "อ่านอีก", "อีกครั้ง", "อีกรอบ", "repeat", "again", "one more time"}) {
		return IntentClassification{Intent: IntentRepeatRequest, Confidence: 0.95, Reason: "repeat_keyword"}
	}
	if containsAny(normalized, []string{"ใบ้", "คำใบ้", "hint", "suggest", "ช่วยหน่อย", "ช่วยหน่อยครับ", "ช่วยหน่อยค่ะ"}) {
		return IntentClassification{Intent: IntentHintRequest, Confidence: 0.95, Reason: "hint_keyword"}
	}
	if containsAny(normalized, []string{"เรียนต่อ", "ไปต่อ", "ต่อเลย", "continue", "next", "go on"}) {
		return IntentClassification{Intent: IntentContinue, Confidence: 0.9, Reason: "continue_keyword"}
	}
	if mode := requestedMode(normalized); mode != "" {
		return IntentClassification{Intent: IntentChangeMode, Confidence: 0.9, Mode: mode, Reason: "mode_keyword"}
	}
	if looksLikeQuestion(normalized) {
		return IntentClassification{Intent: IntentQuestion, Confidence: 0.88, Reason: "question_shape"}
	}

	confidence := 0.7
	if currentMode == "listening" || currentMode == "speaking" || currentMode == "reading" {
		confidence = 0.82
	}
	return IntentClassification{Intent: IntentAnswer, Confidence: confidence, Reason: "default_practice_answer"}
}

func normalizeIntentText(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	var lastSpace bool
	for _, r := range value {
		if unicode.IsSpace(r) {
			if !lastSpace {
				b.WriteRune(' ')
				lastSpace = true
			}
			continue
		}
		lastSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func containsAny(value string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func requestedMode(value string) string {
	switch {
	case containsAny(value, []string{"ฝึกฟัง", "โหมดฟัง", "listening"}):
		return "listening"
	case containsAny(value, []string{"ฝึกพูด", "โหมดพูด", "speaking"}):
		return "speaking"
	case containsAny(value, []string{"ฝึกอ่าน", "โหมดอ่าน", "reading"}):
		return "reading"
	default:
		return ""
	}
}

func looksLikeQuestion(value string) bool {
	if strings.Contains(value, "?") {
		return true
	}
	thQuestion := []string{"คืออะไร", "แปลว่า", "ต่างกัน", "ใช้ยังไง", "ใช้อย่างไร", "ทำไม", "เมื่อไหร่", "ตรงไหน", "อธิบาย", "ช่วยอธิบาย"}
	if containsAny(value, thQuestion) {
		return true
	}
	enQuestion := []string{"what ", "why ", "how ", "when ", "where ", "which ", "can you explain", "does it mean", "difference between"}
	return containsAny(" "+value+" ", enQuestion)
}

func normalizeScore(score float64) float64 {
	switch {
	case score > 5:
		return score / 100
	case score > 1:
		return score / 5
	case score < 0:
		return 0
	default:
		return score
	}
}
